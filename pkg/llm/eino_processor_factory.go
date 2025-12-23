package llm

import (
	"fmt"

	"github.com/shiva/blayzen/pkg/agent"
)

// EinoProcessorFactory creates LLM processors from agent configurations
type EinoProcessorFactory struct{}

// NewEinoProcessorFactory creates a new processor factory
func NewEinoProcessorFactory() *EinoProcessorFactory {
	return &EinoProcessorFactory{}
}

// CreateProcessor creates an LLM processor from agent configuration
func (f *EinoProcessorFactory) CreateProcessor(config *agent.AgentConfig) (agent.Processor, error) {
	switch config.Type {
	case agent.AgentTypeSingle:
		return f.createSimpleAgentProcessor(config)
	default:
		return nil, fmt.Errorf("unsupported agent type: %s", config.Type)
	}
}

// createSimpleAgentProcessor creates a simple agent processor
func (f *EinoProcessorFactory) createSimpleAgentProcessor(config *agent.AgentConfig) (agent.Processor, error) {
	return NewSimpleProcessor(config), nil
}
