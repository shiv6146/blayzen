package interrupt

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shiva/blayzen/internal/frame"
	"github.com/shiva/blayzen/pkg/processor"
	"github.com/shiva/blayzen/pkg/vad"
)

// PlaybackState represents the current playback state
type PlaybackState string

const (
	StateIdle        PlaybackState = "idle"
	StatePlaying     PlaybackState = "playing"
	StateInterrupted PlaybackState = "interrupted"
	StatePaused      PlaybackState = "paused"
)

// InterruptEvent represents an interrupt event
type InterruptEvent struct {
	Type      string // "interrupt_start", "interrupt_end"
	Timestamp time.Time
	FrameID   string
	Reason    string // "speech_detected", "manual", "timeout"
	Metadata  map[string]interface{}
}

// InterruptConfig holds configuration for interrupt handling
type InterruptConfig struct {
	// Interrupt detection
	InterruptThreshold float64       // VAD confidence threshold for interrupt
	MinInterruptTime   time.Duration // Minimum speech duration before interrupt
	InterruptDebounce  time.Duration // Debounce time to prevent false interrupts

	// Playback control
	ClearMessageTimeout time.Duration // Timeout for clear message acknowledgment
	MaxPlaybackDelay    time.Duration // Maximum delay before considering playback stuck

	// Recovery
	AutoResumeDelay     time.Duration // Delay before auto-resuming after false interrupt
	MaxInterruptsPerSec int           // Rate limiting for interrupts
}

// DefaultInterruptConfig returns a default configuration
func DefaultInterruptConfig() *InterruptConfig {
	return &InterruptConfig{
		InterruptThreshold:  0.6, // 60% VAD confidence
		MinInterruptTime:    100 * time.Millisecond,
		InterruptDebounce:   50 * time.Millisecond,
		ClearMessageTimeout: 200 * time.Millisecond,
		MaxPlaybackDelay:    500 * time.Millisecond,
		AutoResumeDelay:     1 * time.Second,
		MaxInterruptsPerSec: 3,
	}
}

// InterruptController manages playback interruption
type InterruptController struct {
	*processor.BaseProcessor
	config  *InterruptConfig
	eventCh chan InterruptEvent

	// State management
	state       atomic.Value // PlaybackState
	interruptCh <-chan vad.VADEvent
	ttsCh       <-chan *frame.Frame
	exotelOut   chan<- *frame.Frame

	// Timing and debouncing
	mu                  sync.RWMutex
	lastInterruptTime   time.Time
	interruptCount      int
	interruptCountReset time.Time
	playbackStartTime   time.Time
	lastTTSTimestamp    time.Time

	// Frame tracking
	ttsQueue []*frame.Frame
	queueMu  sync.Mutex
}

// NewInterruptController creates a new interrupt controller
func NewInterruptController(config *InterruptConfig, vadEventCh <-chan vad.VADEvent, ttsCh <-chan *frame.Frame, exotelOut chan<- *frame.Frame) *InterruptController {
	if config == nil {
		config = DefaultInterruptConfig()
	}

	ic := &InterruptController{
		BaseProcessor: processor.NewBaseProcessor("interrupt-controller"),
		config:        config,
		eventCh:       make(chan InterruptEvent, 100),
		interruptCh:   vadEventCh,
		ttsCh:         ttsCh,
		exotelOut:     exotelOut,
		ttsQueue:      make([]*frame.Frame, 0),
	}

	ic.setState(StateIdle)
	return ic
}

// Events returns a channel of interrupt events
func (ic *InterruptController) Events() <-chan InterruptEvent {
	return ic.eventCh
}

// setState updates the playback state atomically
func (ic *InterruptController) setState(state PlaybackState) {
	ic.state.Store(state)
}

// getState returns the current playback state
func (ic *InterruptController) getState() PlaybackState {
	state, _ := ic.state.Load().(PlaybackState)
	return state
}

// Run starts the interrupt controller
func (ic *InterruptController) Run(ctx context.Context) error {
	// Start goroutines for different responsibilities
	go ic.monitorVAD(ctx)
	go ic.managePlayback(ctx)

	return nil
}

// monitorVAD monitors VAD events for interrupt detection
func (ic *InterruptController) monitorVAD(ctx context.Context) {
	interruptTimer := time.NewTimer(ic.config.MinInterruptTime)
	if !interruptTimer.Stop() {
		<-interruptTimer.C
	}

	for {
		select {
		case <-ctx.Done():
			return

		case event := <-ic.interruptCh:
			ic.handleVADEvent(ctx, event, interruptTimer)
		}
	}
}

// handleVADEvent handles VAD events for interrupt detection
func (ic *InterruptController) handleVADEvent(ctx context.Context, event vad.VADEvent, timer *time.Timer) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Only consider speech start events for interrupts
	if event.Type != "speech_start" {
		return
	}

	// Check if we're currently playing TTS
	if ic.getState() != StatePlaying {
		return
	}

	// Check VAD confidence threshold
	if event.Confidence < ic.config.InterruptThreshold {
		return
	}

	// Rate limiting
	now := time.Now()
	if now.Sub(ic.interruptCountReset) > time.Second {
		ic.interruptCount = 0
		ic.interruptCountReset = now
	}

	if ic.interruptCount >= ic.config.MaxInterruptsPerSec {
		return // Rate limited
	}

	// Reset and start interrupt timer
	if !timer.Stop() {
		<-timer.C
	}
	timer.Reset(ic.config.MinInterruptTime)

	// Mark potential interrupt
	ic.lastInterruptTime = now
}

// managePlayback manages TTS playback and interrupt handling
func (ic *InterruptController) managePlayback(ctx context.Context) {
	interruptTimer := time.NewTimer(ic.config.MinInterruptTime)
	if !interruptTimer.Stop() {
		<-interruptTimer.C
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-interruptTimer.C:
			ic.handlePotentialInterrupt(ctx)

		case frame := <-ic.ttsCh:
			ic.handleTTSFrame(ctx, frame)
		}
	}
}

// handleTTSFrame handles incoming TTS frames
func (ic *InterruptController) handleTTSFrame(ctx context.Context, f *frame.Frame) {
	switch f.Type {
	case "text_out":
		// Text output indicates TTS is starting
		ic.setState(StatePlaying)
		ic.playbackStartTime = time.Now()
		ic.lastTTSTimestamp = time.Now()

		// Add to queue
		ic.queueMu.Lock()
		ic.ttsQueue = append(ic.ttsQueue, f)
		ic.queueMu.Unlock()

		// Forward to Exotel
		select {
		case ic.exotelOut <- f:
		case <-ctx.Done():
			f.Release()
			return
		}

	case "audio_out":
		// Audio output during playback
		ic.lastTTSTimestamp = time.Now()

		// Check if we should interrupt
		if ic.getState() == StateInterrupted {
			// Drop the frame
			f.Release()
			return
		}

		// Forward to Exotel
		select {
		case ic.exotelOut <- f:
		case <-ctx.Done():
			f.Release()
			return
		}

	case "text_eos":
		// End of TTS stream
		ic.setState(StateIdle)
		ic.clearTTSQueue()

		// Forward to Exotel
		select {
		case ic.exotelOut <- f:
		case <-ctx.Done():
			f.Release()
			return
		}

	default:
		// Pass through other frames
		select {
		case ic.exotelOut <- f:
		case <-ctx.Done():
			f.Release()
			return
		}
	}
}

// handlePotentialInterrupt handles a potential interrupt after debounce
func (ic *InterruptController) handlePotentialInterrupt(ctx context.Context) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Check if we should still interrupt
	now := time.Now()
	if ic.getState() != StatePlaying {
		return
	}

	// Check if it's been long enough since last interrupt
	if now.Sub(ic.lastInterruptTime) < ic.config.InterruptDebounce {
		return
	}

	// Check if we're still within the interrupt window
	if now.Sub(ic.lastInterruptTime) > ic.config.MinInterruptTime+ic.config.InterruptDebounce {
		return // Window expired
	}

	// Execute interrupt
	ic.executeInterrupt(ctx)
}

// executeInterrupt executes the interrupt sequence
func (ic *InterruptController) executeInterrupt(ctx context.Context) {
	// Update state
	ic.setState(StateInterrupted)
	ic.interruptCount++

	// Send clear message to Exotel
	clearFrame := frame.New("clear", nil).
		WithMetadata("timestamp", time.Now()).
		WithMetadata("reason", "interrupt").
		WithMetadata("interrupt_confidence", ic.config.InterruptThreshold)

	select {
	case ic.exotelOut <- clearFrame:
	case <-ctx.Done():
		clearFrame.Release()
		return
	}

	// Emit interrupt event
	event := InterruptEvent{
		Type:      "interrupt_start",
		Timestamp: time.Now(),
		Reason:    "speech_detected",
		Metadata: map[string]interface{}{
			"playback_duration": time.Since(ic.playbackStartTime).Milliseconds(),
			"vad_confidence":    ic.config.InterruptThreshold,
		},
	}
	ic.emitEvent(event)

	// Clear TTS queue
	ic.clearTTSQueue()

	// Schedule auto-resume if false interrupt
	go func() {
		time.Sleep(ic.config.AutoResumeDelay)
		if ic.getState() == StateInterrupted {
			// Check if we should resume (no continued speech)
			ic.mu.Lock()
			if time.Since(ic.lastInterruptTime) > ic.config.AutoResumeDelay {
				ic.resumePlayback(ctx)
			}
			ic.mu.Unlock()
		}
	}()
}

// resumePlayback resumes playback after a false interrupt
func (ic *InterruptController) resumePlayback(ctx context.Context) {
	ic.setState(StatePlaying)

	// Emit resume event
	event := InterruptEvent{
		Type:      "interrupt_end",
		Timestamp: time.Now(),
		Reason:    "auto_resume",
		Metadata:  map[string]interface{}{},
	}
	ic.emitEvent(event)
}

// clearTTSQueue clears the TTS playback queue
func (ic *InterruptController) clearTTSQueue() {
	ic.queueMu.Lock()
	defer ic.queueMu.Unlock()

	// Release all queued frames
	for _, frame := range ic.ttsQueue {
		frame.Release()
	}
	ic.ttsQueue = ic.ttsQueue[:0]
}

// emitEvent emits an interrupt event
func (ic *InterruptController) emitEvent(event InterruptEvent) {
	select {
	case ic.eventCh <- event:
	default:
		// Drop event if channel is full
	}
}

// GetStats returns current interrupt controller statistics
func (ic *InterruptController) GetStats() map[string]interface{} {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	ic.queueMu.Lock()
	queueSize := len(ic.ttsQueue)
	ic.queueMu.Unlock()

	stats := map[string]interface{}{
		"state":                ic.getState(),
		"last_interrupt_time":  ic.lastInterruptTime,
		"interrupt_count":      ic.interruptCount,
		"playback_start_time":  ic.playbackStartTime,
		"last_tts_timestamp":   ic.lastTTSTimestamp,
		"tts_queue_size":       queueSize,
		"playback_duration_ms": time.Since(ic.playbackStartTime).Milliseconds(),
	}

	return stats
}

// Process implements the Processor interface (passthrough for non-TTS frames)
func (ic *InterruptController) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
	out := make(chan *frame.Frame, 1024)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return

			case f, ok := <-in:
				if !ok {
					return
				}

				// Pass through all frames (interrupt handling is done via channels)
				select {
				case out <- f:
				case <-ctx.Done():
					f.Release()
					return
				}
			}
		}
	}()

	return out, nil
}
