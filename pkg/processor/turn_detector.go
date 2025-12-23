package processor

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/shiva/blayzen/internal/frame"
)

// TurnEvent represents a turn detection event
type TurnEvent struct {
	Type       string // "turn_start", "turn_end", "utterance_complete"
	Timestamp  time.Time
	FrameID    string
	Text       string  // Complete utterance
	Confidence float64 // 0.0 to 1.0
	Metadata   map[string]interface{}
}

// TurnDetectionConfig holds configuration for turn detection
type TurnDetectionConfig struct {
	// Silence-based detection
	SilenceTimeout   time.Duration // Duration of silence before turn end
	MinUtteranceTime time.Duration // Minimum time for valid utterance
	MaxUtteranceTime time.Duration // Maximum time before forcing turn end

	// Text-based cues (simple pattern matching, not semantic)
	EndOfTurnPhrases []string // Simple phrases that indicate turn end
	QuestionPhrases  []string // Simple phrases that indicate questions

	// Timing parameters
	VADTimeoutCheck time.Duration // How often to check VAD timeout
	ProcessingDelay time.Duration // Delay to allow for final ASR results
}

// DefaultTurnDetectionConfig returns a default configuration
func DefaultTurnDetectionConfig() *TurnDetectionConfig {
	return &TurnDetectionConfig{
		SilenceTimeout:   600 * time.Millisecond, // 600ms silence = turn end
		MinUtteranceTime: 200 * time.Millisecond, // Minimum 200ms speech
		MaxUtteranceTime: 10 * time.Second,       // Force turn end after 10s
		EndOfTurnPhrases: []string{
			"thank you", "thanks", "okay", "alright", "got it",
			"that's all", "that's it", "nothing else", "done",
			"goodbye", "bye", "see you", "take care",
		},
		QuestionPhrases: []string{
			"what", "when", "where", "who", "why", "how",
			"can you", "could you", "would you", "will you",
			"do you", "are you", "is there", "is it",
		},
		VADTimeoutCheck: 50 * time.Millisecond,
		ProcessingDelay: 100 * time.Millisecond,
	}
}

// TurnDetectorProcessor detects conversation turns and utterance boundaries
type TurnDetectorProcessor struct {
	*BaseProcessor
	config  *TurnDetectionConfig
	eventCh chan TurnEvent

	// State tracking
	mu              sync.RWMutex
	utteranceBuffer strings.Builder
	utteranceStart  time.Time
	lastVoiceTime   time.Time
	isSpeaking      bool
	utteranceFrames []*frame.Frame
	silenceTimer    *time.Timer
	pendingFrames   map[string]*frame.Frame // frameID -> frame
}

// NewTurnDetectorProcessor creates a new turn detector processor
func NewTurnDetectorProcessor(config *TurnDetectionConfig) (*TurnDetectorProcessor, error) {
	if config == nil {
		config = DefaultTurnDetectionConfig()
	}

	return &TurnDetectorProcessor{
		BaseProcessor: NewBaseProcessor("turn-detector"),
		config:        config,
		eventCh:       make(chan TurnEvent, 100),
		pendingFrames: make(map[string]*frame.Frame),
	}, nil
}

// Events returns a channel of turn events
func (p *TurnDetectorProcessor) Events() <-chan TurnEvent {
	return p.eventCh
}

// Process implements the Processor interface
func (p *TurnDetectorProcessor) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
	out := make(chan *frame.Frame, 1024)

	// Initialize silence timer
	p.silenceTimer = time.NewTimer(p.config.SilenceTimeout)
	if !p.silenceTimer.Stop() {
		<-p.silenceTimer.C
	}

	go func() {
		defer close(out)
		defer close(p.eventCh)

		for {
			select {
			case <-ctx.Done():
				return

			case f, ok := <-in:
				if !ok {
					// Flush any pending utterance before exiting
					p.flushUtterance(out)
					return
				}

				// Handle different frame types
				switch f.Type {
				case "text_in":
					p.processTextFrame(ctx, f, out)
				case "vad_event":
					p.processVADEvent(ctx, f, out)
				default:
					// Pass through other frames
					select {
					case out <- f:
					case <-ctx.Done():
						f.Release()
						return
					}
				}

			case <-p.silenceTimer.C:
				// Silence timeout - check if we should end turn
				p.handleSilenceTimeout(ctx, out)
			}
		}
	}()

	return out, nil
}

// processTextFrame processes incoming text frames (from ASR)
func (p *TurnDetectorProcessor) processTextFrame(ctx context.Context, f *frame.Frame, out chan<- *frame.Frame) {
	p.mu.Lock()
	defer p.mu.Unlock()

	text := string(f.Payload)
	if text == "" {
		// Empty text, pass through
		select {
		case out <- f:
		case <-ctx.Done():
			f.Release()
			return
		}
		return
	}

	// Check if this is final or intermediate ASR result
	isFinal := false
	if final, ok := f.Metadata["asr_final"].(bool); ok {
		isFinal = final
	}

	// Add to utterance buffer
	if p.utteranceBuffer.Len() == 0 {
		// Start of new utterance
		p.utteranceStart = time.Now()
		p.emitEvent(TurnEvent{
			Type:       "turn_start",
			Timestamp:  time.Now(),
			FrameID:    f.ID,
			Text:       "",
			Confidence: 1.0,
		})
	}

	// Add space if needed
	if p.utteranceBuffer.Len() > 0 {
		p.utteranceBuffer.WriteString(" ")
	}
	p.utteranceBuffer.WriteString(text)

	// Store frame for potential turn end
	p.utteranceFrames = append(p.utteranceFrames, f.Clone())
	p.lastVoiceTime = time.Now()

	// Reset silence timer
	if !p.silenceTimer.Stop() {
		<-p.silenceTimer.C
	}
	p.silenceTimer.Reset(p.config.SilenceTimeout)

	// Check for simple pattern-based turn end indicators
	if isFinal {
		if p.checkPatternBasedTurnEnd(text) {
			// Pattern cue detected, end turn immediately
			go func() {
				time.Sleep(p.config.ProcessingDelay)
				p.endUtterance(ctx, out)
			}()
		}
	}

	// Check max utterance time
	if time.Since(p.utteranceStart) > p.config.MaxUtteranceTime {
		go func() {
			time.Sleep(p.config.ProcessingDelay)
			p.endUtterance(ctx, out)
		}()
	}

	// Pass frame downstream with turn metadata
	f.WithMetadata("turn_active", true).
		WithMetadata("utterance_partial", p.utteranceBuffer.String()).
		WithMetadata("utterance_duration_ms", time.Since(p.utteranceStart).Milliseconds())

	select {
	case out <- f:
	case <-ctx.Done():
		f.Release()
		return
	}
}

// processVADEvent processes VAD events
func (p *TurnDetectorProcessor) processVADEvent(ctx context.Context, f *frame.Frame, out chan<- *frame.Frame) {
	p.mu.Lock()
	defer p.mu.Unlock()

	vadEvent, ok := f.Metadata["vad_event"].(string)
	if !ok {
		select {
		case out <- f:
		case <-ctx.Done():
			f.Release()
			return
		}
		return
	}

	switch vadEvent {
	case "speech_start":
		if !p.isSpeaking {
			p.isSpeaking = true
			p.lastVoiceTime = time.Now()
		}

	case "speech_end":
		if p.isSpeaking {
			p.isSpeaking = false
			// Start silence timer
			if !p.silenceTimer.Stop() {
				<-p.silenceTimer.C
			}
			p.silenceTimer.Reset(p.config.SilenceTimeout)
		}
	}

	// Pass VAD frame downstream
	select {
	case out <- f:
	case <-ctx.Done():
		f.Release()
		return
	}
}

// handleSilenceTimeout handles silence timeout events
func (p *TurnDetectorProcessor) handleSilenceTimeout(ctx context.Context, out chan<- *frame.Frame) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if we have an active utterance
	if p.utteranceBuffer.Len() > 0 {
		utteranceDuration := time.Since(p.utteranceStart)

		// Check minimum utterance duration
		if utteranceDuration >= p.config.MinUtteranceTime {
			// End the utterance
			p.endUtterance(ctx, out)
		} else {
			// Too short, ignore and reset
			p.resetUtterance()
		}
	}
}

// endUtterance ends the current utterance and sends turn end event
func (p *TurnDetectorProcessor) endUtterance(ctx context.Context, out chan<- *frame.Frame) {
	if p.utteranceBuffer.Len() == 0 {
		return
	}

	utteranceText := p.cleanUtterance(p.utteranceBuffer.String())
	utteranceDuration := time.Since(p.utteranceStart)

	// Check if utterance is meaningful
	if len(strings.Fields(utteranceText)) == 0 {
		p.resetUtterance()
		return
	}

	// Create turn end frame
	turnEndFrame := frame.New("turn_end", []byte(utteranceText)).
		WithMetadata("timestamp", time.Now()).
		WithMetadata("utterance_duration_ms", utteranceDuration.Milliseconds()).
		WithMetadata("utterance_start_time", p.utteranceStart).
		WithMetadata("frame_count", len(p.utteranceFrames))

	// Emit turn end event
	event := TurnEvent{
		Type:       "turn_end",
		Timestamp:  time.Now(),
		FrameID:    turnEndFrame.ID,
		Text:       utteranceText,
		Confidence: p.calculateTurnConfidence(utteranceText, utteranceDuration),
		Metadata: map[string]interface{}{
			"duration_ms": utteranceDuration.Milliseconds(),
			"frame_count": len(p.utteranceFrames),
		},
	}
	p.emitEvent(event)

	// Send turn end frame
	select {
	case out <- turnEndFrame:
	case <-ctx.Done():
		turnEndFrame.Release()
		return
	}

	// Reset for next utterance
	p.resetUtterance()
}

// checkPatternBasedTurnEnd checks if text contains simple pattern-based turn end cues
func (p *TurnDetectorProcessor) checkPatternBasedTurnEnd(text string) bool {
	lowerText := strings.ToLower(strings.TrimSpace(text))

	// Check for explicit end-of-turn phrases
	for _, phrase := range p.config.EndOfTurnPhrases {
		if strings.Contains(lowerText, phrase) {
			return true
		}
	}

	// Check for questions (expect response)
	for _, phrase := range p.config.QuestionPhrases {
		if strings.HasPrefix(lowerText, phrase) {
			return true
		}
	}

	// Check for sentence-ending punctuation
	if strings.HasSuffix(lowerText, ".") ||
		strings.HasSuffix(lowerText, "!") ||
		strings.HasSuffix(lowerText, "?") {
		return true
	}

	return false
}

// cleanUtterance cleans up the utterance text
func (p *TurnDetectorProcessor) cleanUtterance(text string) string {
	// Remove extra whitespace
	cleaned := strings.Join(strings.Fields(text), " ")
	return cleaned
}

// calculateTurnConfidence calculates confidence score for turn detection
func (p *TurnDetectorProcessor) calculateTurnConfidence(text string, duration time.Duration) float64 {
	confidence := 0.5 // Base confidence

	// Adjust based on utterance length
	wordCount := len(strings.Fields(text))
	if wordCount >= 3 {
		confidence += 0.2
	} else if wordCount >= 1 {
		confidence += 0.1
	}

	// Adjust based on duration
	if duration >= p.config.MinUtteranceTime && duration <= 5*time.Second {
		confidence += 0.2
	}

	// Adjust based on semantic cues
	lowerText := strings.ToLower(text)
	for _, phrase := range p.config.EndOfTurnPhrases {
		if strings.Contains(lowerText, phrase) {
			confidence += 0.3
			break
		}
	}

	return min(1.0, confidence)
}

// resetUtterance resets the utterance buffer and state
func (p *TurnDetectorProcessor) resetUtterance() {
	p.utteranceBuffer.Reset()
	p.utteranceStart = time.Time{}
	p.utteranceFrames = p.utteranceFrames[:0]
}

// flushUtterance flushes any pending utterance
func (p *TurnDetectorProcessor) flushUtterance(out chan<- *frame.Frame) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.utteranceBuffer.Len() > 0 {
		utteranceText := p.cleanUtterance(p.utteranceBuffer.String())
		if utteranceText != "" {
			turnEndFrame := frame.New("turn_end", []byte(utteranceText)).
				WithMetadata("timestamp", time.Now()).
				WithMetadata("flushed", true)

			select {
			case out <- turnEndFrame:
			default:
				turnEndFrame.Release()
			}
		}
	}

	p.resetUtterance()
}

// emitEvent emits a turn detection event
func (p *TurnDetectorProcessor) emitEvent(event TurnEvent) {
	select {
	case p.eventCh <- event:
	default:
		// Drop event if channel is full
	}
}

// GetStats returns current turn detection statistics
func (p *TurnDetectorProcessor) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := map[string]interface{}{
		"is_speaking":          p.isSpeaking,
		"utterance_active":     p.utteranceBuffer.Len() > 0,
		"utterance_duration":   time.Since(p.utteranceStart).Milliseconds(),
		"last_voice_time":      p.lastVoiceTime,
		"utterance_word_count": len(strings.Fields(p.utteranceBuffer.String())),
		"pending_frames":       len(p.pendingFrames),
	}

	return stats
}

// min returns the minimum of two floats
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
