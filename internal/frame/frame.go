package frame

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Frame represents the unified data structure that flows through the pipeline.
// It can carry audio, text, or any other data type with metadata.
type Frame struct {
	// ID uniquely identifies this frame within a session
	ID string `json:"id"`

	// Type indicates the frame type: "audio_in", "audio_out", "text_in", "text_out", "error"
	Type string `json:"type"`

	// Payload contains the actual data (audio bytes, text JSON, etc.)
	Payload []byte `json:"payload"`

	// Metadata holds additional information like timestamps, confidence scores, etc.
	Metadata map[string]interface{} `json:"metadata"`

	// Done signals the end of a stream
	Done bool `json:"done"`

	// Err holds any error that occurred during processing
	Err error `json:"error,omitempty"`

	// Timestamp when the frame was created
	Timestamp time.Time `json:"timestamp"`
}

// FramePool provides a pool for reusing Frame objects to reduce GC pressure
var FramePool = sync.Pool{
	New: func() interface{} {
		return &Frame{
			Metadata: make(map[string]interface{}),
		}
	},
}

// New creates a new Frame with the given type and payload
func New(frameType string, payload []byte) *Frame {
	frame := FramePool.Get().(*Frame)
	frame.ID = uuid.New().String()
	frame.Type = frameType
	frame.Payload = payload
	frame.Timestamp = time.Now()
	frame.Done = false
	frame.Err = nil

	// Clear metadata from previous use
	for k := range frame.Metadata {
		delete(frame.Metadata, k)
	}

	return frame
}

// Release returns the Frame to the pool for reuse
func (f *Frame) Release() {
	if f != nil {
		FramePool.Put(f)
	}
}

// WithMetadata adds metadata to the frame
func (f *Frame) WithMetadata(key string, value interface{}) *Frame {
	if f.Metadata == nil {
		f.Metadata = make(map[string]interface{})
	}
	f.Metadata[key] = value
	return f
}

// WithDone marks the frame as end-of-stream
func (f *Frame) WithDone() *Frame {
	f.Done = true
	return f
}

// WithError sets an error on the frame
func (f *Frame) WithError(err error) *Frame {
	f.Err = err
	return f
}

// Clone creates a shallow copy of the frame
func (f *Frame) Clone() *Frame {
	clone := New(f.Type, append([]byte(nil), f.Payload...))

	// Copy metadata
	for k, v := range f.Metadata {
		clone.Metadata[k] = v
	}

	clone.Done = f.Done
	clone.Err = f.Err
	clone.Timestamp = f.Timestamp

	return clone
}
