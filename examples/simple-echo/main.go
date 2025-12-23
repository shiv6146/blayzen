package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shiva/blayzen/internal/frame"
	"github.com/shiva/blayzen/internal/pipeline"
	"github.com/shiva/blayzen/internal/transport"
	"github.com/shiva/blayzen/pkg/processor"
)

var (
	wsURL    = flag.String("url", "", "Exotel WebSocket URL (or set EXOTEL_WS_URL env var)")
	delay    = flag.Duration("delay", 100*time.Millisecond, "Echo delay")
	logLevel = flag.String("log-level", "info", "Log level: debug, info, warn, error")
)

// SimpleEchoBot just echoes all audio frames
type SimpleEchoBot struct {
	pipeline  *pipeline.Pipeline
	transport *transport.ExotelWebSocketTransport
}

// NewSimpleEchoBot creates a new simple echo bot
func NewSimpleEchoBot(wsURL string, delay time.Duration) *SimpleEchoBot {
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

	// Create simple echo processor that echoes all audio
	echoProcessor := processor.NewTransformProcessor("echo", func(f *frame.Frame) *frame.Frame {
		if f.Type == "audio_in" {
			// Simulate processing delay
			time.Sleep(delay)

			// Create echo frame
			echoFrame := frame.New("audio_out", f.Payload).
				WithMetadata("echo", true).
				WithMetadata("original_frame_id", f.ID).
				WithMetadata("echo_delay_ms", delay.Milliseconds())

			// Release original frame
			f.Release()

			return echoFrame
		}
		return f
	})

	// Build pipeline
	pipeline, err := pipeline.NewBuilder().
		WithIncoming(wsTransport).
		AddProcessor(echoProcessor).
		WithOutgoing(wsTransport).
		Build()
	if err != nil {
		log.Fatalf("Failed to build pipeline: %v", err)
	}

	return &SimpleEchoBot{
		pipeline:  pipeline,
		transport: wsTransport,
	}
}

// Run starts the simple echo bot
func (bot *SimpleEchoBot) Run(ctx context.Context) error {
	// Set up event handlers
	bot.transport.SetConnectedHandler(func() {
		log.Println("✅ Connected to Exotel WebSocket")
	})

	bot.transport.SetStartHandler(func(startMsg map[string]interface{}) {
		log.Printf("🚀 Stream started: %+v", startMsg)
	})

	bot.transport.SetStopHandler(func() {
		log.Println("⏹️  Stream stopped")
	})

	bot.transport.SetErrorHandler(func(err error) {
		log.Printf("❌ Transport error: %v", err)
	})

	// Set up error handler for pipeline
	bot.pipeline.WithErrorHandler(func(frame *frame.Frame) {
		log.Printf("❌ Pipeline error: %v", frame.Err)
	})

	// Run pipeline
	log.Printf("🤖 Starting Simple Echo Bot (delay: %v)...", *delay)
	return bot.pipeline.Run(ctx)
}

// Stop gracefully stops the echo bot
func (bot *SimpleEchoBot) Stop() {
	log.Println("🛑 Stopping Simple Echo Bot...")
	bot.pipeline.Stop()
	bot.transport.Close()
}

func main() {
	flag.Parse()

	// Set log level
	switch *logLevel {
	case "debug":
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	case "info":
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	case "warn", "error":
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		log.SetOutput(os.Stderr)
	}

	// Get WebSocket URL from flag or environment
	url := *wsURL
	if url == "" {
		url = os.Getenv("EXOTEL_WS_URL")
	}

	if url == "" {
		fmt.Printf("Usage:\n")
		fmt.Printf("  %s -url ws://localhost:8080/ws\n", os.Args[0])
		fmt.Printf("  or set EXOTEL_WS_URL environment variable\n")
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create and run echo bot
	bot := NewSimpleEchoBot(url, *delay)

	// Run bot in background
	botErr := make(chan error, 1)
	go func() {
		botErr <- bot.Run(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigCh:
		log.Printf("\n📡 Received signal %v, shutting down...", sig)
	case err := <-botErr:
		if err != nil && err != context.Canceled {
			log.Printf("❌ Bot error: %v", err)
		}
	}

	// Graceful shutdown
	bot.Stop()
	cancel()
	log.Println("✅ Simple Echo Bot shutdown complete")
}
