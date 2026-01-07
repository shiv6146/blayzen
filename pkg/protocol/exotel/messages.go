package exotel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Message represents a generic Exotel WebSocket message
type Message struct {
	Event string `json:"event"`
}

// ConnectedMessage is sent when the WebSocket connection is established
type ConnectedMessage struct {
	Event string `json:"event"` // "connected"
}

// StartMessage is sent when a call starts, containing call metadata
type StartMessage struct {
	Event      string                 `json:"event"`      // "start"
	StreamSID  string                 `json:"streamSid"`  // Unique stream identifier
	CallSID    string                 `json:"callSid"`    // Call identifier
	AccountSID string                 `json:"accountSid"` // Account identifier
	From       string                 `json:"from"`       // Caller number
	To         string                 `json:"to"`         // Called number
	CustomData map[string]interface{} `json:"customData"` // Custom metadata
}

// MediaMessage contains audio data (base64 encoded)
type MediaMessage struct {
	Event     string `json:"event"` // "media"
	StreamSID string `json:"streamSid,omitempty"`
	Media     Media  `json:"media"`
}

// Media contains the audio payload and metadata
type Media struct {
	Payload   string `json:"payload"`   // Base64 encoded audio (μ-law 8kHz)
	Chunk     int    `json:"chunk"`     // Chunk sequence number
	Timestamp int64  `json:"timestamp"` // Timestamp in milliseconds
}

// StopMessage is sent when a call ends
type StopMessage struct {
	Event     string `json:"event"` // "stop"
	StreamSID string `json:"streamSid,omitempty"`
}

// DTMFMessage contains DTMF digit information
type DTMFMessage struct {
	Event string `json:"event"` // "dtmf"
	DTMF  string `json:"dtmf"`  // DTMF digit (0-9, *, #)
}

// MarkMessage is used for synchronization markers
type MarkMessage struct {
	Event string `json:"event"` // "mark"
	Name  string `json:"name"`  // Marker name
}

// ClearMessage requests clearing the audio buffer
type ClearMessage struct {
	Event string `json:"event"` // "clear"
}

// OutboundMediaMessage is sent from client to server with audio response
type OutboundMediaMessage struct {
	Event     string `json:"event"` // "media"
	Media     string `json:"media"` // Base64 encoded audio
	Timestamp int64  `json:"timestamp,omitempty"`
	Chunk     int    `json:"chunk,omitempty"`
}

// NewConnectedMessage creates a new connected message
func NewConnectedMessage() *ConnectedMessage {
	return &ConnectedMessage{Event: EventConnected}
}

// NewStartMessage creates a new start message with call metadata
func NewStartMessage(streamSID, callSID, accountSID, from, to string) *StartMessage {
	return &StartMessage{
		Event:      EventStart,
		StreamSID:  streamSID,
		CallSID:    callSID,
		AccountSID: accountSID,
		From:       from,
		To:         to,
		CustomData: make(map[string]interface{}),
	}
}

// NewMediaMessage creates a new media message with audio payload
func NewMediaMessage(streamSID string, audioData []byte, chunk int, timestamp int64) *MediaMessage {
	return &MediaMessage{
		Event:     EventMedia,
		StreamSID: streamSID,
		Media: Media{
			Payload:   base64.StdEncoding.EncodeToString(audioData),
			Chunk:     chunk,
			Timestamp: timestamp,
		},
	}
}

// NewStopMessage creates a new stop message
func NewStopMessage(streamSID string) *StopMessage {
	return &StopMessage{
		Event:     EventStop,
		StreamSID: streamSID,
	}
}

// NewClearMessage creates a new clear message
func NewClearMessage() *ClearMessage {
	return &ClearMessage{Event: EventClear}
}

// NewDTMFMessage creates a new DTMF message
func NewDTMFMessage(digit string) *DTMFMessage {
	return &DTMFMessage{
		Event: EventDTMF,
		DTMF:  digit,
	}
}

// NewMarkMessage creates a new mark message
func NewMarkMessage(name string) *MarkMessage {
	return &MarkMessage{
		Event: EventMark,
		Name:  name,
	}
}

// DecodeAudio decodes the base64 audio payload from a MediaMessage
func (m *MediaMessage) DecodeAudio() ([]byte, error) {
	return base64.StdEncoding.DecodeString(m.Media.Payload)
}

// ParseMessage parses a raw JSON message and returns the appropriate type
func ParseMessage(data []byte) (interface{}, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	switch msg.Event {
	case EventConnected:
		var m ConnectedMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil

	case EventStart:
		var m StartMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil

	case EventMedia:
		var m MediaMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil

	case EventStop:
		var m StopMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil

	case EventDTMF:
		var m DTMFMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil

	case EventMark:
		var m MarkMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil

	case EventClear:
		var m ClearMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", msg.Event)
	}
}
