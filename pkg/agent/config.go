package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/shiva/blayzen/internal/frame"
)

// AgentConfig is the root configuration for agents
type AgentConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Type        AgentType              `json:"type"`
	Settings    CommonSettings         `json:"settings"`
}

// AgentType defines the type of agent
type AgentType string

const (
	AgentTypeSingle AgentType = "single"
)

// CommonSettings holds common agent settings
type CommonSettings struct {
	DefaultModel ModelConfig     `json:"default_model"`
	Tools        []ToolDef       `json:"tools,omitempty"`
	Context      ContextSettings `json:"context"`
	Timeout      time.Duration   `json:"timeout,omitempty"`
	MaxRetries   int             `json:"max_retries,omitempty"`
	Streaming    bool            `json:"streaming"`
}

// UnmarshalJSON implements custom JSON unmarshalling for CommonSettings
func (cs *CommonSettings) UnmarshalJSON(data []byte) error {
	// Define alias type to avoid recursion
	type Alias CommonSettings
	aux := &struct {
		Timeout string `json:"timeout,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(cs),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Timeout != "" {
		duration, err := time.ParseDuration(aux.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout format: %w", err)
		}
		cs.Timeout = duration
	}

	return nil
}

// ModelConfig defines a chat model configuration
type ModelConfig struct {
	Provider   string                 `json:"provider"`
	Model      string                 `json:"model"`
	APIKey     string                 `json:"api_key,omitempty"`
	BaseURL    string                 `json:"base_url,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// ToolDef defines a tool that agents can use
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Function    string                 `json:"function"`
	Parameters  map[string]interface{} `json:"parameters"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ContextSettings defines context management behavior
type ContextSettings struct {
	MaxTokens      int           `json:"max_tokens"`
	ReservePercent float64       `json:"reserve_percent"`
	Summarization  bool          `json:"summarization"`
	CleanupAge     time.Duration `json:"cleanup_age"`
}

// UnmarshalJSON implements custom JSON unmarshalling for ContextSettings
func (cs *ContextSettings) UnmarshalJSON(data []byte) error {
	// Define alias type to avoid recursion
	type Alias ContextSettings
	aux := &struct {
		CleanupAge string `json:"cleanup_age"`
		*Alias
	}{
		Alias: (*Alias)(cs),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.CleanupAge != "" {
		duration, err := time.ParseDuration(aux.CleanupAge)
		if err != nil {
			return fmt.Errorf("invalid cleanup_age format: %w", err)
		}
		cs.CleanupAge = duration
	}

	return nil
}

// ConfigLoader loads and validates agent configurations
type ConfigLoader struct {
	registry map[string]interface{}
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		registry: make(map[string]interface{}),
	}
}

// RegisterFunction registers a Go function that can be referenced in config
func (cl *ConfigLoader) RegisterFunction(name string, fn interface{}) {
	cl.registry[name] = fn
}

// LoadFromFile loads agent configuration from a JSON file
func (cl *ConfigLoader) LoadFromFile(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return cl.LoadFromBytes(data)
}

// LoadFromBytes loads agent configuration from JSON bytes
func (cl *ConfigLoader) LoadFromBytes(data []byte) (*AgentConfig, error) {
	var config AgentConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := cl.validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// validateConfig validates agent configuration
func (cl *ConfigLoader) validateConfig(config *AgentConfig) error {
	if config.Name == "" {
		return fmt.Errorf("agent name is required")
	}

	if config.Type == "" {
		return fmt.Errorf("agent type is required")
	}

	return nil
}

// ModelFactory creates chat models from configuration
type ModelFactory interface {
	CreateModel(config ModelConfig) (interface{}, error)
}

// Processor interface for agent implementations
type Processor interface {
	Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error)
	GetStats() map[string]interface{}
	Name() string
}
