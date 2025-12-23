package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/shiva/blayzen/internal/frame"
	"github.com/shiva/blayzen/pkg/agent"
)

// SimpleProcessorFactory creates simple processors for testing
type SimpleProcessorFactory struct{}

// NewSimpleProcessorFactory creates a new simple processor factory
func NewSimpleProcessorFactory() *SimpleProcessorFactory {
	return &SimpleProcessorFactory{}
}

// CreateProcessor creates a simple processor from agent configuration
func (f *SimpleProcessorFactory) CreateProcessor(config *agent.AgentConfig) (agent.Processor, error) {
	return NewSimpleProcessor(config), nil
}

// SimpleProcessor implements a basic agent processor
type SimpleProcessor struct {
	config *agent.AgentConfig
}

// NewSimpleProcessor creates a new simple processor
func NewSimpleProcessor(config *agent.AgentConfig) *SimpleProcessor {
	return &SimpleProcessor{
		config: config,
	}
}

// Process processes incoming frames and generates responses
func (p *SimpleProcessor) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
	out := make(chan *frame.Frame, 1024)

	go func() {
		defer close(out)

		for f := range in {
			switch f.Type {
			case "text_in":
				// Simple echo response
				responseText := fmt.Sprintf("Agent '%s' received: %s", p.config.Name, string(f.Payload))
				response := frame.New("text_out", []byte(responseText))
				select {
				case out <- response:
				case <-ctx.Done():
					return
				}
			case "tool_call":
				// Simple tool response
				response := frame.New("tool_response", []byte(`{"status": "completed"}`))
				response.WithMetadata("tool_call_id", f.Metadata["tool_call_id"])
				select {
				case out <- response:
				case <-ctx.Done():
					return
				}
			}
			f.Release()
		}
	}()

	return out, nil
}

// GetStats returns processor statistics
func (p *SimpleProcessor) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":    "simple",
		"agent":   p.config.Name,
		"started": time.Now(),
	}
}

// Name returns the processor name
func (p *SimpleProcessor) Name() string {
	return "simple_processor:" + p.config.Name
}
