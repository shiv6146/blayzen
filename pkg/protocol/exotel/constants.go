// Package exotel provides types and constants for the Exotel WebSocket protocol.
// This protocol is used for real-time voice communication between SIP servers
// and Blayzen voice agents.
package exotel

// Event types for Exotel WebSocket protocol
const (
	// Server to Client events
	EventConnected = "connected"
	EventStart     = "start"
	EventMedia     = "media"
	EventStop      = "stop"
	EventMark      = "mark"
	EventDTMF      = "dtmf"

	// Client to Server events
	EventClear = "clear"
)

// Frame types used internally by Blayzen
const (
	FrameTypeAudioIn  = "audio_in"
	FrameTypeAudioOut = "audio_out"
	FrameTypeTextIn   = "text_in"
	FrameTypeTextOut  = "text_out"
	FrameTypeControl  = "control"
	FrameTypeError    = "error"
)

// Default configuration values
const (
	DefaultReadBufferSize  = 1024 * 10
	DefaultWriteBufferSize = 1024 * 10
	DefaultReadTimeout     = 60000 // milliseconds
	DefaultWriteTimeout    = 10000 // milliseconds
	DefaultPingInterval    = 30000 // milliseconds
)
