package server

import (
	"bytes"
	"io"
	"sync"
)

// WritableStream is a custom type that satisfies io.WriteCloser and io.Reader
type WritableStream struct {
	buf    bytes.Buffer
	mu     sync.Mutex
	closed bool
	cond   *sync.Cond
}

// NewWritableStream creates a new WritableStream
func NewWritableStream() *WritableStream {
	ws := &WritableStream{}
	ws.cond = sync.NewCond(&ws.mu)
	return ws
}

// Write writes data to the buffer
func (ws *WritableStream) Write(p []byte) (int, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := ws.buf.Write(p)
	ws.cond.Signal()
	return n, err
}

// Read reads data from the buffer
func (ws *WritableStream) Read(p []byte) (int, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	for ws.buf.Len() == 0 && !ws.closed {
		ws.cond.Wait()
	}

	if ws.buf.Len() == 0 && ws.closed {
		return 0, io.EOF
	}

	return ws.buf.Read(p)
}

// Close marks the stream as closed
func (ws *WritableStream) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.closed = true
	ws.cond.Broadcast()
	return nil
}
