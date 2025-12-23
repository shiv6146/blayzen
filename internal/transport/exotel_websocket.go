package transport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shiv6146/blayzen/internal/frame"
)

// ExotelWebSocketTransport implements BidirectionalTransport for Exotel's WebSocket protocol
// It handles all serialization/deserialization internally, exposing only Frame-based interface
type ExotelWebSocketTransport struct {
	*BaseTransport
	conn       *websocket.Conn
	ctx        context.Context
	cancel     context.CancelFunc
	inCh       chan *frame.Frame
	outCh      chan *frame.Frame
	writeMu    sync.Mutex
	closed     bool
	closedMu   sync.RWMutex
	serializer *ExotelMessageSerializer

	// Event handlers
	onConnected func()
	onStart     func(startMsg map[string]interface{})
	onStop      func()
	onError     func(error)
}

// ExotelMessageSerializer handles Exotel protocol serialization/deserialization
type ExotelMessageSerializer struct{}

// SerializeFrame converts a Frame to Exotel WebSocket message format
func (s *ExotelMessageSerializer) SerializeFrame(f *frame.Frame) ([]byte, error) {
	// Handle different frame types
	switch f.Type {
	case "audio_out":
		// Convert audio frame to Exotel media message
		mediaMsg := map[string]interface{}{
			"event": "media",
			"media": base64.StdEncoding.EncodeToString(f.Payload),
		}
		// Add metadata if present
		if timestamp, ok := f.Metadata["timestamp"]; ok {
			mediaMsg["timestamp"] = timestamp
		}
		if chunk, ok := f.Metadata["chunk"]; ok {
			mediaMsg["chunk"] = chunk
		}
		return json.Marshal(mediaMsg)

	case "stop":
		// Send stop message
		stopMsg := map[string]string{"event": "stop"}
		return json.Marshal(stopMsg)

	case "dtmf":
		// Send DTMF message
		if dtmf, ok := f.Metadata["dtmf"].(string); ok {
			dtmfMsg := map[string]string{"event": "dtmf", "dtmf": dtmf}
			return json.Marshal(dtmfMsg)
		}

	case "mark":
		// Send mark message
		if name, ok := f.Metadata["name"].(string); ok {
			markMsg := map[string]string{"event": "mark", "name": name}
			return json.Marshal(markMsg)
		}

	case "clear":
		// Send clear message
		clearMsg := map[string]string{"event": "clear"}
		return json.Marshal(clearMsg)

	default:
		// Unknown frame type, skip
		return nil, fmt.Errorf("unsupported frame type for Exotel serialization: %s", f.Type)
	}

	return nil, nil
}

// DeserializeMessage converts Exotel WebSocket message to Frame
func (s *ExotelMessageSerializer) DeserializeMessage(data []byte) (*frame.Frame, error) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("json unmarshal error: %w", err)
	}

	// Handle different message events
	event, ok := msg["event"].(string)
	if !ok {
		return nil, fmt.Errorf("missing event field in message")
	}

	switch event {
	case "connected":
		// Connected event
		f := frame.New("connected", data).
			WithMetadata("event", "connected")
		return f, nil

	case "start":
		// Start event with stream parameters
		f := frame.New("start", data).
			WithMetadata("event", "start")
		return f, nil

	case "media":
		// Media event with audio payload
		mediaStr, ok := msg["media"].(string)
		if !ok {
			return nil, fmt.Errorf("missing media field in media message")
		}

		// Decode base64 audio
		audio, err := base64.StdEncoding.DecodeString(mediaStr)
		if err != nil {
			return nil, fmt.Errorf("base64 decode error: %w", err)
		}

		// Create audio frame
		audioFrame := frame.New("audio_in", audio).
			WithMetadata("event", "media").
			WithMetadata("timestamp", msg["timestamp"]).
			WithMetadata("chunk", msg["chunk"])
		return audioFrame, nil

	case "dtmf":
		// DTMF event
		if dtmf, ok := msg["dtmf"].(string); ok {
			dtmfFrame := frame.New("dtmf", []byte(dtmf)).
				WithMetadata("event", "dtmf").
				WithMetadata("digit", dtmf)
			return dtmfFrame, nil
		}
		// Invalid DTMF format, treat as unknown
		f := frame.New("unknown", data).
			WithMetadata("event", event).
			WithMetadata("raw", msg)
		return f, nil

	case "stop":
		// Stop event
		stopFrame := frame.New("stop", data).
			WithMetadata("event", "stop")
		return stopFrame, nil

	case "mark":
		// Mark event
		if name, ok := msg["name"].(string); ok {
			markFrame := frame.New("mark", []byte(name)).
				WithMetadata("event", "mark").
				WithMetadata("name", name)
			return markFrame, nil
		}
		// Invalid mark format, treat as unknown
		f := frame.New("unknown", data).
			WithMetadata("event", event).
			WithMetadata("raw", msg)
		return f, nil

	case "clear":
		// Clear event
		clearFrame := frame.New("clear", nil).
			WithMetadata("event", "clear")
		return clearFrame, nil

	default:
		// Unknown event
		f := frame.New("unknown", data).
			WithMetadata("event", event).
			WithMetadata("raw", msg)
		return f, nil
	}
}

// NewExotelWebSocketTransport creates a new Exotel WebSocket transport
func NewExotelWebSocketTransport(config *TransportConfig) *ExotelWebSocketTransport {
	if config == nil {
		config = &TransportConfig{
			ReadBufferSize:  1024 * 10,
			WriteBufferSize: 1024 * 10,
			ReadTimeout:     60000,
			WriteTimeout:    10000,
			PingInterval:    30000,
			Options:         make(map[string]interface{}),
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	t := &ExotelWebSocketTransport{
		BaseTransport: NewBaseTransport("exotel-websocket"),
		ctx:           ctx,
		cancel:        cancel,
		inCh:          make(chan *frame.Frame, config.ReadBufferSize),
		outCh:         make(chan *frame.Frame, config.WriteBufferSize),
		serializer:    &ExotelMessageSerializer{},
	}

	// Connect in background
	go t.connect(config)

	return t
}

// connect establishes the WebSocket connection
func (t *ExotelWebSocketTransport) connect(config *TransportConfig) {
	// Get URL from options
	url, ok := config.Options["url"].(string)
	if !ok {
		t.handleError(fmt.Errorf("url option is required"))
		return
	}

	// Get headers from options
	var headers http.Header
	if h, ok := config.Options["headers"]; ok {
		if hdr, ok := h.(http.Header); ok {
			headers = hdr
		}
	}

	dialer := websocket.Dialer{
		ReadBufferSize:  config.ReadBufferSize,
		WriteBufferSize: config.WriteBufferSize,
	}

	conn, _, err := dialer.Dial(url, headers)
	if err != nil {
		t.handleError(fmt.Errorf("websocket dial failed: %w", err))
		return
	}

	t.closedMu.Lock()
	t.conn = conn
	t.closedMu.Unlock()

	// Send connected message
	t.sendConnected()

	// Start read and write pumps
	var wg sync.WaitGroup
	wg.Add(2)

	go t.readPump(&wg, config)
	go t.writePump(&wg, config)

	// Wait for pumps to finish
	wg.Wait()
}

// In implements IncomingTransport interface
func (t *ExotelWebSocketTransport) In() <-chan *frame.Frame {
	return t.inCh
}

// Out implements OutgoingTransport interface
func (t *ExotelWebSocketTransport) Out() chan<- *frame.Frame {
	return t.outCh
}

// Close implements both IncomingTransport and OutgoingTransport interfaces
func (t *ExotelWebSocketTransport) Close() error {
	t.closedMu.Lock()
	if t.closed {
		t.closedMu.Unlock()
		return nil
	}
	t.closed = true
	t.closedMu.Unlock()

	t.cancel()

	// Close WebSocket connection
	t.closedMu.RLock()
	if t.conn != nil {
		t.conn.Close()
	}
	t.closedMu.RUnlock()

	// Close channels
	close(t.inCh)
	close(t.outCh)

	return nil
}

// sendConnected sends the connected event
func (t *ExotelWebSocketTransport) sendConnected() {
	connectedFrame := frame.New("connected", nil).
		WithMetadata("event", "connected")

	select {
	case t.inCh <- connectedFrame:
	default:
		connectedFrame.Release()
	}

	if t.onConnected != nil {
		t.onConnected()
	}
}

// readPump reads messages from WebSocket and converts them to frames
func (t *ExotelWebSocketTransport) readPump(wg *sync.WaitGroup, config *TransportConfig) {
	defer wg.Done()

	t.closedMu.RLock()
	conn := t.conn
	t.closedMu.RUnlock()

	if conn == nil {
		return
	}

	// Set read deadline and pong handler
	conn.SetReadDeadline(time.Now().Add(time.Duration(config.ReadTimeout) * time.Millisecond))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(time.Duration(config.ReadTimeout) * time.Millisecond))
		return nil
	})

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					t.handleError(fmt.Errorf("websocket read error: %w", err))
				}
				return
			}

			// Deserialize message to frame
			frame, err := t.serializer.DeserializeMessage(message)
			if err != nil {
				t.handleError(fmt.Errorf("deserialization error: %w", err))
				continue
			}

			// Handle specific events
			sendToPipeline := true
			if event, ok := frame.Metadata["event"].(string); ok {
				switch event {
				case "connected":
					if t.onConnected != nil {
						t.onConnected()
					}
					sendToPipeline = false // Don't send to pipeline
				case "start":
					if t.onStart != nil {
						t.onStart(frame.Metadata)
					}
					sendToPipeline = false // Don't send to pipeline
				case "stop":
					if t.onStop != nil {
						t.onStop()
					}
				}
			}

			// Send frame to pipeline if needed
			if sendToPipeline {
				select {
				case <-t.ctx.Done():
					frame.Release()
					return
				case t.inCh <- frame:
				}
			} else {
				frame.Release() // Release if not sending to pipeline
			}
		}
	}
}

// writePump writes frames to WebSocket
func (t *ExotelWebSocketTransport) writePump(wg *sync.WaitGroup, config *TransportConfig) {
	defer wg.Done()

	ticker := time.NewTicker(time.Duration(config.PingInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case frame, ok := <-t.outCh:
			if !ok {
				return
			}

			// Serialize frame to message
			data, err := t.serializer.SerializeFrame(frame)
			if err != nil {
				t.handleError(fmt.Errorf("serialization error: %w", err))
				frame.Release()
				continue
			}

			if data == nil {
				// Frame not supported for serialization, skip
				frame.Release()
				continue
			}

			// Send message
			t.closedMu.RLock()
			conn := t.conn
			t.closedMu.RUnlock()

			if conn != nil {
				t.writeMu.Lock()
				conn.SetWriteDeadline(time.Now().Add(time.Duration(config.WriteTimeout) * time.Millisecond))
				err := conn.WriteMessage(websocket.TextMessage, data)
				t.writeMu.Unlock()

				if err != nil {
					t.handleError(fmt.Errorf("websocket write error: %w", err))
					frame.Release()
					return
				}
			}

			frame.Release()

		case <-ticker.C:
			// Send ping
			t.closedMu.RLock()
			conn := t.conn
			t.closedMu.RUnlock()

			if conn != nil {
				t.writeMu.Lock()
				conn.SetWriteDeadline(time.Now().Add(time.Duration(config.WriteTimeout) * time.Millisecond))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					t.writeMu.Unlock()
					t.handleError(fmt.Errorf("websocket ping error: %w", err))
					return
				}
				t.writeMu.Unlock()
			}
		}
	}
}

// handleError handles errors
func (t *ExotelWebSocketTransport) handleError(err error) {
	if t.onError != nil {
		t.onError(err)
	} else {
		log.Printf("ExotelWebSocketTransport error: %v", err)
	}
}

// SetConnectedHandler sets the handler for connected events
func (t *ExotelWebSocketTransport) SetConnectedHandler(handler func()) {
	t.onConnected = handler
}

// SetStartHandler sets the handler for start events
func (t *ExotelWebSocketTransport) SetStartHandler(handler func(map[string]interface{})) {
	t.onStart = handler
}

// SetStopHandler sets the handler for stop events
func (t *ExotelWebSocketTransport) SetStopHandler(handler func()) {
	t.onStop = handler
}

// SetErrorHandler sets the handler for errors
func (t *ExotelWebSocketTransport) SetErrorHandler(handler func(error)) {
	t.onError = handler
}
