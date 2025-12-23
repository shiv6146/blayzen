package processor

import (
	"context"

	"github.com/shiv6146/blayzen/internal/frame"
)

// Processor defines the interface for all pipeline stages.
// Each processor receives frames from an input channel, transforms them,
// and outputs them to a new channel.
type Processor interface {
	// Process starts the processor with the given input channel.
	// It returns an output channel that will receive the processed frames.
	// The processor should handle its own goroutine lifecycle.
	Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error)

	// Name returns a human-readable name for the processor
	Name() string
}

// BaseProcessor provides common functionality for processor implementations
type BaseProcessor struct {
	name string
}

// NewBaseProcessor creates a new base processor with the given name
func NewBaseProcessor(name string) *BaseProcessor {
	return &BaseProcessor{name: name}
}

// Name returns the processor name
func (p *BaseProcessor) Name() string {
	return p.name
}

// PassthroughProcessor is a processor that simply passes frames through
// without modification. Useful for testing or as a placeholder.
type PassthroughProcessor struct {
	*BaseProcessor
}

// NewPassthroughProcessor creates a new passthrough processor
func NewPassthroughProcessor() *PassthroughProcessor {
	return &PassthroughProcessor{
		BaseProcessor: NewBaseProcessor("passthrough"),
	}
}

// Process implements the Processor interface
func (p *PassthroughProcessor) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
	out := make(chan *frame.Frame, 1024)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-in:
				if !ok {
					return
				}

				select {
				case <-ctx.Done():
					return
				case out <- frame:
				}
			}
		}
	}()

	return out, nil
}

// FilterProcessor filters frames based on a predicate function
type FilterProcessor struct {
	*BaseProcessor
	predicate func(*frame.Frame) bool
}

// NewFilterProcessor creates a new filter processor
func NewFilterProcessor(name string, predicate func(*frame.Frame) bool) *FilterProcessor {
	return &FilterProcessor{
		BaseProcessor: NewBaseProcessor(name),
		predicate:     predicate,
	}
}

// Process implements the Processor interface
func (p *FilterProcessor) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
	out := make(chan *frame.Frame, 1024)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-in:
				if !ok {
					return
				}

				if p.predicate(frame) {
					select {
					case <-ctx.Done():
						return
					case out <- frame:
					}
				} else {
					// Release filtered frames back to pool
					frame.Release()
				}
			}
		}
	}()

	return out, nil
}

// TransformProcessor applies a transformation function to each frame
type TransformProcessor struct {
	*BaseProcessor
	transform func(*frame.Frame) *frame.Frame
}

// NewTransformProcessor creates a new transform processor
func NewTransformProcessor(name string, transform func(*frame.Frame) *frame.Frame) *TransformProcessor {
	return &TransformProcessor{
		BaseProcessor: NewBaseProcessor(name),
		transform:     transform,
	}
}

// Process implements the Processor interface
func (p *TransformProcessor) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
	out := make(chan *frame.Frame, 1024)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-in:
				if !ok {
					return
				}

				transformed := p.transform(frame)
				if transformed != nil {
					select {
					case <-ctx.Done():
						return
					case out <- transformed:
					}
				}
			}
		}
	}()

	return out, nil
}
