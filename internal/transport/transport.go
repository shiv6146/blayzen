package transport

import (
	"context"
	"io"

	"github.com/shiva/blayzen/internal/frame"
)

// IncomingTransport defines the interface for data sources that provide frames to the pipeline
type IncomingTransport interface {
	// In returns a channel that provides frames from the transport
	In() <-chan *frame.Frame

	// Close gracefully shuts down the transport and releases resources
	Close() error

	// Name returns a human-readable name for the transport
	Name() string
}

// OutgoingTransport defines the interface for data sinks that consume frames from the pipeline
type OutgoingTransport interface {
	// Out returns a channel that the pipeline writes frames to
	Out() chan<- *frame.Frame

	// Close gracefully shuts down the transport and releases resources
	Close() error

	// Name returns a human-readable name for the transport
	Name() string
}

// BidirectionalTransport combines both incoming and outgoing capabilities
type BidirectionalTransport interface {
	IncomingTransport
	OutgoingTransport
}

// MessageSerializer is an interface for transports that need to serialize/deserialize messages
type MessageSerializer interface {
	// SerializeFrame converts a Frame to the transport's wire format
	SerializeFrame(*frame.Frame) ([]byte, error)

	// DeserializeMessage converts the transport's wire format to a Frame
	DeserializeMessage([]byte) (*frame.Frame, error)
}

// TransportConfig holds common configuration for transports
type TransportConfig struct {
	// Buffer sizes for internal channels
	ReadBufferSize  int
	WriteBufferSize int

	// Timeouts
	ReadTimeout  int // milliseconds
	WriteTimeout int // milliseconds
	PingInterval int // milliseconds

	// Protocol-specific options
	Options map[string]interface{}
}

// BaseTransport provides common functionality for transport implementations
type BaseTransport struct {
	name string
}

// NewBaseTransport creates a new base transport with the given name
func NewBaseTransport(name string) *BaseTransport {
	return &BaseTransport{name: name}
}

// Name returns the transport name
func (t *BaseTransport) Name() string {
	return t.name
}

// ChannelIncomingTransport is a simple transport that uses a provided channel
type ChannelIncomingTransport struct {
	*BaseTransport
	ch <-chan *frame.Frame
}

// NewChannelIncomingTransport creates a new channel-based incoming transport
func NewChannelIncomingTransport(name string, ch <-chan *frame.Frame) *ChannelIncomingTransport {
	return &ChannelIncomingTransport{
		BaseTransport: NewBaseTransport(name),
		ch:            ch,
	}
}

// In returns the input channel
func (t *ChannelIncomingTransport) In() <-chan *frame.Frame {
	return t.ch
}

// Close implements the IncomingTransport interface
func (t *ChannelIncomingTransport) Close() error {
	// Nothing to close for a channel transport
	return nil
}

// ChannelOutgoingTransport is a simple transport that uses a provided channel
type ChannelOutgoingTransport struct {
	*BaseTransport
	ch chan<- *frame.Frame
}

// NewChannelOutgoingTransport creates a new channel-based outgoing transport
func NewChannelOutgoingTransport(name string, ch chan<- *frame.Frame) *ChannelOutgoingTransport {
	return &ChannelOutgoingTransport{
		BaseTransport: NewBaseTransport(name),
		ch:            ch,
	}
}

// Out returns the output channel
func (t *ChannelOutgoingTransport) Out() chan<- *frame.Frame {
	return t.ch
}

// Close implements the OutgoingTransport interface
func (t *ChannelOutgoingTransport) Close() error {
	// Close the channel to signal completion
	close(t.ch)
	return nil
}

// ReaderIncomingTransport adapts an io.Reader to provide frames
type ReaderIncomingTransport struct {
	*BaseTransport
	ctx    context.Context
	reader io.Reader
	ch     chan *frame.Frame
}

// NewReaderIncomingTransport creates a new reader-based incoming transport
func NewReaderIncomingTransport(ctx context.Context, name string, reader io.Reader, bufferSize int) *ReaderIncomingTransport {
	t := &ReaderIncomingTransport{
		BaseTransport: NewBaseTransport(name),
		ctx:           ctx,
		reader:        reader,
		ch:            make(chan *frame.Frame, bufferSize),
	}

	// Start reading in background
	go t.readLoop()

	return t
}

// In returns the input channel
func (t *ReaderIncomingTransport) In() <-chan *frame.Frame {
	return t.ch
}

// Close implements the IncomingTransport interface
func (t *ReaderIncomingTransport) Close() error {
	close(t.ch)
	return nil
}

// readLoop continuously reads from the reader and creates frames
func (t *ReaderIncomingTransport) readLoop() {
	defer close(t.ch)

	buf := make([]byte, 1024) // 1KB buffer

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			n, err := t.reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					frame := frame.New("error", nil).WithError(err)
					select {
					case t.ch <- frame:
					case <-t.ctx.Done():
						return
					}
				}
				return
			}

			if n > 0 {
				payload := make([]byte, n)
				copy(payload, buf[:n])
				frame := frame.New("audio_in", payload)
				select {
				case t.ch <- frame:
				case <-t.ctx.Done():
					frame.Release()
					return
				}
			}
		}
	}
}

// WriterOutgoingTransport adapts an io.Writer to consume frames
type WriterOutgoingTransport struct {
	*BaseTransport
	ctx    context.Context
	writer io.Writer
	ch     chan *frame.Frame
}

// NewWriterOutgoingTransport creates a new writer-based outgoing transport
func NewWriterOutgoingTransport(ctx context.Context, name string, writer io.Writer, bufferSize int) *WriterOutgoingTransport {
	t := &WriterOutgoingTransport{
		BaseTransport: NewBaseTransport(name),
		ctx:           ctx,
		writer:        writer,
		ch:            make(chan *frame.Frame, bufferSize),
	}

	// Start writing in background
	go t.writeLoop()

	return t
}

// Out returns the output channel
func (t *WriterOutgoingTransport) Out() chan<- *frame.Frame {
	return t.ch
}

// Close implements the OutgoingTransport interface
func (t *WriterOutgoingTransport) Close() error {
	close(t.ch)
	return nil
}

// writeLoop continuously writes frames to the writer
func (t *WriterOutgoingTransport) writeLoop() {
	for frame := range t.ch {
		if frame.Err != nil {
			continue // Skip error frames
		}

		if len(frame.Payload) > 0 {
			_, err := t.writer.Write(frame.Payload)
			if err != nil {
				// Log error or handle it as needed
				continue
			}
		}

		frame.Release()
	}
}
