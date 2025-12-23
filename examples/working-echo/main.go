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

	"github.com/shiv6146/blayzen/internal/frame"
	"github.com/shiv6146/blayzen/internal/pipeline"
	"github.com/shiv6146/blayzen/internal/transport"
	"github.com/shiv6146/blayzen/pkg/processor"
)

var (
	wsURL   = flag.String("url", "", "Exotel WebSocket URL")
	delay   = flag.Duration("delay", 100*time.Millisecond, "Echo delay")
	verbose = flag.Bool("verbose", false, "Verbose logging")
)

func main() {
	flag.Parse()

	if *wsURL == "" {
		fmt.Printf("Usage: %s -url ws://localhost:8080/ws [-delay 50ms]\n", os.Args[0])
		os.Exit(1)
	}

	logger := log.New(os.Stdout, "[ECHOBOT] ", log.LstdFlags|log.Lmicroseconds)

	// Create incoming transport (for receiving from Exotel)
	inConfig := &transport.TransportConfig{
		ReadBufferSize:  1024 * 10,
		WriteBufferSize: 1024 * 10,
		ReadTimeout:     60000,
		WriteTimeout:    10000,
		PingInterval:    30000,
		Options: map[string]interface{}{
			"url": *wsURL,
		},
	}
	inTransport := transport.NewExotelWebSocketTransport(inConfig)

	// Create outgoing transport (for sending to Exotel)
	outTransport := transport.NewExotelWebSocketTransport(inConfig)

	// Create echo processor that only processes audio
	echoProcessor := processor.NewTransformProcessor("echo", func(f *frame.Frame) *frame.Frame {
		if *verbose {
			logger.Printf("Processing frame: %s (payload size: %d)", f.Type, len(f.Payload))
		}

		// Only echo audio frames
		if f.Type == "audio_in" {
			// Simulate processing delay
			time.Sleep(*delay)

			// Create echo frame
			echoFrame := frame.New("audio_out", f.Payload).
				WithMetadata("echo", true).
				WithMetadata("original_frame_id", f.ID).
				WithMetadata("echo_delay_ms", delay.Milliseconds())

			if *verbose {
				logger.Printf("Created echo frame with %d bytes of audio", len(f.Payload))
			}

			// Release original frame
			f.Release()
			return echoFrame
		}

		// Pass through all other frames
		return f
	})

	// Build pipeline
	pipeline, err := pipeline.NewBuilder().
		WithIncoming(inTransport).
		AddProcessor(echoProcessor).
		WithOutgoing(outTransport).
		Build()
	if err != nil {
		log.Fatalf("Failed to build pipeline: %v", err)
	}

	// Set up event handlers
	inTransport.SetConnectedHandler(func() {
		logger.Println("✅ Connected to Exotel")
	})
	inTransport.SetStartHandler(func(startMsg map[string]interface{}) {
		logger.Printf("🚀 Stream started")
	})
	inTransport.SetStopHandler(func() {
		logger.Println("⏹️  Stream stopped")
	})
	inTransport.SetErrorHandler(func(err error) {
		logger.Printf("❌ Transport error: %v", err)
	})

	pipeline.WithErrorHandler(func(frame *frame.Frame) {
		logger.Printf("❌ Pipeline error: %v", frame.Err)
	})

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start pipeline in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- pipeline.Run(ctx)
	}()

	logger.Printf("🤖 Echo Bot started (delay: %v)", *delay)
	logger.Println("Press Ctrl+C to stop")

	// Wait for signal or error
	select {
	case sig := <-sigCh:
		logger.Printf("\n📡 Received signal %v, shutting down...", sig)
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			logger.Printf("❌ Pipeline error: %v", err)
		}
	}

	// Graceful shutdown
	logger.Println("🛑 Shutting down...")
	pipeline.Stop()
	if err := inTransport.Close(); err != nil {
		logger.Printf("Error closing input transport: %v", err)
	}
	if err := outTransport.Close(); err != nil {
		logger.Printf("Error closing output transport: %v", err)
	}
	cancel()
	logger.Println("✅ Shutdown complete")
}
