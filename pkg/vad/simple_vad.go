package vad

import (
	"context"
	"time"

	"github.com/shiva/blayzen/internal/frame"
)

// VADEvent represents a voice activity detection event
type VADEvent struct {
	Type       string        `json:"type"` // "speech_start", "speech_end", "silence_detected"
	Timestamp  time.Time     `json:"timestamp"`
	Confidence float64       `json:"confidence"`
	Duration   time.Duration `json:"duration,omitempty"`
}

// SimpleVADProcessor implements a simple VAD processor
type SimpleVADProcessor struct {
	events chan VADEvent
}

// VADConfig holds VAD configuration
type VADConfig struct {
	FrameSize     int           `json:"frame_size"`
	SampleRate    int           `json:"sample_rate"`
	SilenceThresh float64       `json:"silence_threshold"`
	VADTimeout    time.Duration `json:"vad_timeout"`
}

// DefaultVADConfig returns a default VAD configuration
func DefaultVADConfig() *VADConfig {
	return &VADConfig{
		FrameSize:     640,
		SampleRate:    16000,
		SilenceThresh: 0.5,
		VADTimeout:    300 * time.Millisecond,
	}
}

// NewSimpleVADProcessor creates a new simple VAD processor
func NewSimpleVADProcessor(config *VADConfig) (*SimpleVADProcessor, error) {
	return &SimpleVADProcessor{
		events: make(chan VADEvent, 1024),
	}, nil
}

// Process processes audio frames and detects voice activity
func (v *SimpleVADProcessor) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
	out := make(chan *frame.Frame, 1024)

	go func() {
		defer close(out)
		defer close(v.events)

		for f := range in {
			if f.Type == "audio_in" {
				// Pass through audio frame
				select {
				case out <- f:
				case <-ctx.Done():
					return
				}

				// Send a simple VAD event
				vadEvent := VADEvent{
					Type:       "speech_detected",
					Timestamp:  time.Now(),
					Confidence: 0.8,
					Duration:   100 * time.Millisecond,
				}
				select {
				case v.events <- vadEvent:
				case <-ctx.Done():
					return
				}
			} else {
				// Pass through other frames
				select {
				case out <- f:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

// Events returns the VAD events channel
func (v *SimpleVADProcessor) Events() <-chan VADEvent {
	return v.events
}

// Name returns the processor name
func (v *SimpleVADProcessor) Name() string {
	return "simple_vad"
}

// Close closes the VAD processor
func (v *SimpleVADProcessor) Close() error {
	return nil
}
