package transport

import (
	"github.com/shiv6146/blayzen/internal/frame"
)

// MockTransport implements a mock transport for testing
type MockTransport struct {
	inCh  chan *frame.Frame
	outCh chan *frame.Frame
}

// Name returns the transport name
func (t *MockTransport) Name() string {
	return "mock_transport"
}

// NewMockTransport creates a new mock transport
func NewMockTransport() *MockTransport {
	return &MockTransport{
		inCh:  make(chan *frame.Frame, 1024),
		outCh: make(chan *frame.Frame, 1024),
	}
}

// In returns the input channel for receiving frames (IncomingTransport interface)
func (t *MockTransport) In() <-chan *frame.Frame {
	return t.inCh
}

// Out returns the output channel for sending frames (OutgoingTransport interface)
func (t *MockTransport) Out() chan<- *frame.Frame {
	return t.outCh
}

// SendIn sends a frame to the input channel (used in tests)
func (t *MockTransport) SendIn(f *frame.Frame) {
	select {
	case t.inCh <- f:
	default:
		// Channel full, drop the frame
	}
}

// ReceiveOut receives a frame from the output channel (used in tests)
func (t *MockTransport) ReceiveOut() *frame.Frame {
	select {
	case f := <-t.outCh:
		return f
	default:
		return nil
	}
}

// Close closes the transport
func (t *MockTransport) Close() error {
	close(t.inCh)
	close(t.outCh)
	return nil
}
