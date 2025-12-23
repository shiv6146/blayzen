package main

import (
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// Simple mock Exotel server for testing
type MockExotelServer struct {
	server   *http.Server
	upgrader websocket.Upgrader
}

func NewMockExotelServer() *MockExotelServer {
	return &MockExotelServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *MockExotelServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Client connected to mock Exotel server")

	// Send connected event
	connectedMsg := map[string]interface{}{
		"event": "connected",
	}
	if err := conn.WriteJSON(connectedMsg); err != nil {
		log.Printf("Error sending connected message: %v", err)
		return
	}

	// Send start event
	startMsg := map[string]interface{}{
		"event":      "start",
		"streamSid":  "mock_stream_123",
		"callSid":    "mock_call_456",
		"accountSid": "mock_account_789",
	}
	if err := conn.WriteJSON(startMsg); err != nil {
		log.Printf("Error sending start message: %v", err)
		return
	}

	// Simulate receiving audio and echoing it back
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		chunk := 0

		for {
			select {
			case <-ticker.C:
				// Send mock audio data
				audioData := make([]byte, 160) // 20ms of 8kHz audio
				for i := range audioData {
					audioData[i] = byte(i % 256)
				}

				mediaMsg := map[string]interface{}{
					"event":     "media",
					"streamSid": "mock_stream_123",
					"media":     base64.StdEncoding.EncodeToString(audioData),
					"timestamp": time.Now().UnixMilli(),
					"chunk":     chunk,
				}

				if err := conn.WriteJSON(mediaMsg); err != nil {
					log.Printf("Error sending media: %v", err)
					return
				}
				chunk++
			}
		}
	}()

	// Handle incoming messages (echo back audio_out frames)
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Read error: %v", err)
			break
		}

		// Log received messages
		if event, ok := msg["event"].(string); ok {
			log.Printf("Received event: %s", event)
		}
	}
}

func (s *MockExotelServer) Start(addr string) error {
	s.server = &http.Server{
		Addr:              addr,
		Handler:           http.HandlerFunc(s.handleWebSocket),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Mock Exotel server starting on %s", addr)
	return s.server.ListenAndServe()
}

func (s *MockExotelServer) Stop() {
	if s.server != nil {
		if err := s.server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}
}

func main() {
	// Start mock server
	mockServer := NewMockExotelServer()
	go func() {
		if err := mockServer.Start(":8080"); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Mock server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Set environment variable for echo bot
	if err := os.Setenv("EXOTEL_WS_URL", "ws://localhost:8080/ws"); err != nil {
		log.Printf("Warning: failed to set environment variable: %v", err)
	}

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Run echo bot (this would normally be in a separate process)
	// For demo purposes, we'll just keep the mock server running
	log.Println("Mock Exotel server is running.")
	log.Println("In another terminal, run:")
	log.Println("  cd examples/simple-echo")
	log.Println("  go run . -url ws://localhost:8080/ws")
	log.Println("")
	log.Println("The echo bot will connect and echo back audio with VAD detection.")
	log.Println("Press Ctrl+C to stop")

	<-sigCh
	log.Println("\nShutting down...")
	mockServer.Stop()
}
