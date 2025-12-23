package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shiva/blayzen/internal/frame"
	"github.com/shiva/blayzen/internal/pipeline"
	"github.com/shiva/blayzen/internal/transport"
	"github.com/shiva/blayzen/pkg/processor"
	"github.com/shiva/blayzen/pkg/vad"
)

// EchoBot is a simple echo bot that demonstrates the framework
type EchoBot struct {
	pipeline  *pipeline.Pipeline
	transport *transport.ExotelWebSocketTransport
}

// NewEchoBot creates a new echo bot
func NewEchoBot(wsURL string) *EchoBot {
	// Configure WebSocket transport
	config := &transport.TransportConfig{
		ReadBufferSize:  1024 * 10,
		WriteBufferSize: 1024 * 10,
		ReadTimeout:     60000,
		WriteTimeout:    10000,
		PingInterval:    30000,
		Options: map[string]interface{}{
			"url": wsURL,
		},
	}

	// Create Exotel transport
	wsTransport := transport.NewExotelWebSocketTransport(config)

	// Create processors
	vadProcessor, _ := vad.NewSimpleVADProcessor(vad.DefaultVADConfig())
	echoProcessor := NewEchoProcessor(100 * time.Millisecond)
	loggingProcessor := NewLoggingProcessor(log.New(os.Stdout, "[ECHOBOT] ", log.LstdFlags))

	// Build pipeline
	pipeline, err := pipeline.NewBuilder().
		WithIncoming(wsTransport).
		AddProcessors(vadProcessor, echoProcessor, loggingProcessor).
		WithOutgoing(wsTransport).
		Build()
	if err != nil {
		log.Fatalf("Failed to build pipeline: %v", err)
	}

	return &EchoBot{
		pipeline:  pipeline,
		transport: wsTransport,
	}
}

// EchoProcessor echoes audio frames
type EchoProcessor struct {
	*processor.BaseProcessor
	delay time.Duration
}

// NewEchoProcessor creates a new echo processor
func NewEchoProcessor(delay time.Duration) *EchoProcessor {
	return &EchoProcessor{
		BaseProcessor: processor.NewBaseProcessor("echo"),
		delay:         delay,
	}
}

// Process implements the Processor interface
func (p *EchoProcessor) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
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

				// Echo only audio frames with VAD speech
				if f.Type == "audio_in" {
					// Check if this is speech (not silence)
					if vadEvent, ok := f.Metadata["vad_event"].(string); ok {
						if vadEvent == "speech_start" || vadEvent == "speech_continuing" {
							// Simulate processing delay
							time.Sleep(p.delay)

							// Create echo frame
							echoFrame := frame.New("audio_out", f.Payload).
								WithMetadata("echo", true).
								WithMetadata("original_frame_id", f.ID).
								WithMetadata("echo_delay_ms", p.delay.Milliseconds())

							select {
							case <-ctx.Done():
								f.Release()
								echoFrame.Release()
								return
							case out <- echoFrame:
							}
						}
					}
					f.Release()
				} else {
					// Pass through non-audio frames
					select {
					case <-ctx.Done():
						f.Release()
						return
					case out <- f:
					}
				}
			}
		}
	}()

	return out, nil
}

// LoggingProcessor logs frames for debugging
type LoggingProcessor struct {
	*processor.BaseProcessor
	logger *log.Logger
}

// NewLoggingProcessor creates a new logging processor
func NewLoggingProcessor(logger *log.Logger) *LoggingProcessor {
	if logger == nil {
		logger = log.Default()
	}
	return &LoggingProcessor{
		BaseProcessor: processor.NewBaseProcessor("logger"),
		logger:        logger,
	}
}

// Process implements the Processor interface
func (p *LoggingProcessor) Process(ctx context.Context, in <-chan *frame.Frame) (<-chan *frame.Frame, error) {
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

				// Log frame information
				if f.Err != nil {
					p.logger.Printf("[%s] Error: %v", f.Type, f.Err)
				} else {
					// Log VAD events
					if vadEvent, ok := f.Metadata["vad_event"].(string); ok {
						if vadEvent == "speech_start" {
							p.logger.Printf("🎤 User started speaking")
						} else if vadEvent == "speech_end" {
							p.logger.Printf("🤫 User stopped speaking")
						}
					}

					// Log echo events
					if echo, ok := f.Metadata["echo"].(bool); ok && echo {
						p.logger.Printf("🔊 Echoing audio (delay: %v ms)", f.Metadata["echo_delay_ms"])
					}

					// Log other frame types
					switch f.Type {
					case "connected":
						p.logger.Printf("✅ Connected to Exotel")
					case "start":
						p.logger.Printf("🚀 Stream started")
					case "stop":
						p.logger.Printf("🛑 Stream stopped")
					}
				}

				// Pass through the frame
				select {
				case <-ctx.Done():
					f.Release()
					return
				case out <- f:
				}
			}
		}
	}()

	return out, nil
}

// Run starts the echo bot
func (bot *EchoBot) Run(ctx context.Context) error {
	// Set up event handlers
	bot.transport.SetConnectedHandler(func() {
		log.Println("Connected to Exotel WebSocket")
	})

	bot.transport.SetStartHandler(func(startMsg map[string]interface{}) {
		log.Printf("Received start message: %+v", startMsg)
	})

	bot.transport.SetStopHandler(func() {
		log.Println("Received stop message - shutting down")
	})

	bot.transport.SetErrorHandler(func(err error) {
		log.Printf("Transport error: %v", err)
	})

	// Set up error handler for pipeline
	bot.pipeline.WithErrorHandler(func(frame *frame.Frame) {
		log.Printf("Pipeline error: %v", frame.Err)
	})

	// Set up frame hook for monitoring
	bot.pipeline.WithFrameHook(func(frame *frame.Frame) {
		// Additional monitoring can be added here
	})

	// Run pipeline
	log.Println("Starting Echo Bot...")
	return bot.pipeline.Run(ctx)
}

// Stop gracefully stops the echo bot
func (bot *EchoBot) Stop() {
	log.Println("Stopping Echo Bot...")
	bot.pipeline.Stop()
	bot.transport.Close()
}

func main() {
	logger := log.New(os.Stdout, "[ECHOBOT] ", log.LstdFlags|log.Lmicroseconds)

	// Get WebSocket URL from environment or use default
	wsURL := os.Getenv("EXOTEL_WS_URL")
	if wsURL == "" {
		wsURL = "ws://localhost:8080/ws" // Default for testing
	}

	logger.Printf("Starting Echo Bot with Exotel WebSocket URL: %s", wsURL)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create and run echo bot
	bot := NewEchoBot(wsURL)

	// Run bot in background
	botErr := make(chan error, 1)
	go func() {
		botErr <- bot.Run(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigCh:
		logger.Printf("Received signal %v, shutting down...", sig)
	case err := <-botErr:
		if err != nil {
			logger.Printf("Bot error: %v", err)
		}
	}

	// Graceful shutdown
	bot.Stop()
	cancel()
	logger.Println("Echo Bot stopped")
}
