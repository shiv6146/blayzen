package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/shiva/blayzen/internal/frame"
	"github.com/shiva/blayzen/internal/transport"
	"github.com/shiva/blayzen/pkg/processor"
)

// Pipeline orchestrates the flow of frames through a series of processors
type Pipeline struct {
	// Transport for incoming frames
	Incoming transport.IncomingTransport

	// List of processors to apply in order
	Processors []processor.Processor

	// Transport for outgoing frames
	Outgoing transport.OutgoingTransport

	// Optional error handling
	ErrorHandler func(*frame.Frame)

	// Optional frame hooks
	OnFrame func(*frame.Frame)

	// Internal state
	mu         sync.RWMutex
	running    bool
	cancelFunc context.CancelFunc
	done       chan struct{}
}

// New creates a new pipeline with the specified transports and processors
func New(incoming transport.IncomingTransport, processors []processor.Processor, outgoing transport.OutgoingTransport) *Pipeline {
	return &Pipeline{
		Incoming:   incoming,
		Processors: processors,
		Outgoing:   outgoing,
		done:       make(chan struct{}),
	}
}

// WithErrorHandler sets an error handler for the pipeline
func (p *Pipeline) WithErrorHandler(handler func(*frame.Frame)) *Pipeline {
	p.ErrorHandler = handler
	return p
}

// WithFrameHook sets a frame hook for monitoring/debugging
func (p *Pipeline) WithFrameHook(hook func(*frame.Frame)) *Pipeline {
	p.OnFrame = hook
	return p
}

// Run starts the pipeline and blocks until the context is cancelled or an error occurs
func (p *Pipeline) Run(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return errors.New("pipeline is already running")
	}
	p.running = true
	p.mu.Unlock()

	// Create a cancellable context for the pipeline
	pipelineCtx, cancel := context.WithCancel(ctx)
	p.cancelFunc = cancel
	defer cancel()

	// Close transports when done
	defer p.Incoming.Close()
	defer p.Outgoing.Close()

	// Get the input channel from the incoming transport
	in := p.Incoming.In()

	// Chain the processors together
	currentCh := in
	var err error

	for i, proc := range p.Processors {
		currentCh, err = proc.Process(pipelineCtx, currentCh)
		if err != nil {
			return fmt.Errorf("processor %d (%s) failed: %w", i, proc.Name(), err)
		}
	}

	// Start a goroutine to handle the final output
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(p.done)

		for {
			select {
			case <-pipelineCtx.Done():
				return
			case frame, ok := <-currentCh:
				if !ok {
					return
				}

				// Apply frame hook if set
				if p.OnFrame != nil {
					p.OnFrame(frame)
				}

				// Handle error frames
				if frame.Err != nil {
					if p.ErrorHandler != nil {
						p.ErrorHandler(frame)
					}
					frame.Release()
					continue
				}

				// Send to outgoing transport
				select {
				case <-pipelineCtx.Done():
					frame.Release()
					return
				case p.Outgoing.Out() <- frame:
				}
			}
		}
	}()

	// Wait for completion
	<-pipelineCtx.Done()
	wg.Wait()

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	return pipelineCtx.Err()
}

// Stop gracefully stops the pipeline
func (p *Pipeline) Stop() {
	p.mu.Lock()
	if !p.running || p.cancelFunc == nil {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	p.cancelFunc()
	<-p.done
}

// IsRunning returns true if the pipeline is currently running
func (p *Pipeline) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Builder provides a fluent interface for building pipelines
type Builder struct {
	incoming   transport.IncomingTransport
	processors []processor.Processor
	outgoing   transport.OutgoingTransport
}

// NewBuilder creates a new pipeline builder
func NewBuilder() *Builder {
	return &Builder{
		processors: make([]processor.Processor, 0),
	}
}

// WithIncoming sets the incoming transport
func (b *Builder) WithIncoming(transport transport.IncomingTransport) *Builder {
	b.incoming = transport
	return b
}

// WithOutgoing sets the outgoing transport
func (b *Builder) WithOutgoing(transport transport.OutgoingTransport) *Builder {
	b.outgoing = transport
	return b
}

// AddProcessor adds a processor to the pipeline
func (b *Builder) AddProcessor(processor processor.Processor) *Builder {
	b.processors = append(b.processors, processor)
	return b
}

// AddProcessors adds multiple processors to the pipeline
func (b *Builder) AddProcessors(processors ...processor.Processor) *Builder {
	b.processors = append(b.processors, processors...)
	return b
}

// Build creates the pipeline with the configured components
func (b *Builder) Build() (*Pipeline, error) {
	if b.incoming == nil {
		return nil, errors.New("incoming transport is required")
	}
	if b.outgoing == nil {
		return nil, errors.New("outgoing transport is required")
	}
	if len(b.processors) == 0 {
		return nil, errors.New("at least one processor is required")
	}

	return New(b.incoming, b.processors, b.outgoing), nil
}
