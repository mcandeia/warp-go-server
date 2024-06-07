package server

import (
	"fmt"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type duplexChan[TSend, TReceive Chunked] struct {
	opts   Options[TSend, TReceive]
	ws     *websocket.Conn
	closed *atomic.Bool
	done   chan bool
	send   chan<- TSend
	recv   <-chan TReceive
}

// Recv is a method to receive a message
func (c *duplexChan[TSend, TReceive]) Recv() <-chan TReceive {
	return c.recv
}

// Send is a method to send a message
func (c *duplexChan[TSend, TReceive]) Send(msg TSend) error {
	if c.closed.Load() {
		return fmt.Errorf("channel is closed and cannot send message")
	}
	select {
	case <-c.done:
		return fmt.Errorf("channel was closed")
	case c.send <- msg:
		return nil
	}
}

// Close is a method to check if the channel is closed
func (c *duplexChan[TSend, TReceive]) Close() bool {
	swapped := c.closed.CompareAndSwap(false, true)
	if swapped {
		close(c.done)
	}
	return swapped
}

// Closed is a method to return the closed channel
func (c *duplexChan[TSend, TReceive]) Closed() <-chan bool {
	return c.done
}

// DuplexChan creates a channel for sending and receiving messages
type DuplexChan[TSend, TReceive Chunked] interface {
	// Send is a method to send a message
	Send(TSend) error
	// Recv is a method to receive a message
	Recv() <-chan TReceive
	// Close is a method to close the channel
	Close() bool
	// Closed is a method to check if the channel is closed
	Closed() <-chan bool
}

// ChanSerializer is an interface for serializing and deserializing messages
type ChanSerializer[TSend, TReceive Chunked] interface {
	// Serialize is a method to serialize a message
	Serialize(TSend) ([]byte, error)
	// Deserialize is a method to deserialize a message
	Deserialize([]byte) (TReceive, error)
}

// Options is a struct to hold options for a channel
type Options[TSend, TReceive Chunked] struct {
	// serializer is a serializer for the channel
	serializer ChanSerializer[TSend, TReceive]
}

// Option is a type for options
type Option[TSend, TReceive Chunked] func(*Options[TSend, TReceive])

// WithSerializer is an option to set a serializer
func WithSerializer[TSend, TReceive Chunked](serializer ChanSerializer[TSend, TReceive]) Option[TSend, TReceive] {
	return func(o *Options[TSend, TReceive]) {
		o.serializer = serializer
	}
}

// NewDuplexChan creates a new channel for sending and receiving messages
func NewDuplexChan[TSend, TReceive Chunked](ws *websocket.Conn, options ...Option[TSend, TReceive]) DuplexChan[TSend, TReceive] {
	opts := Options[TSend, TReceive]{
		serializer: jsonSerializer[TSend, TReceive]{},
	}
	for _, option := range options {
		option(&opts)
	}
	done := make(chan bool, 1)
	send := make(chan TSend)
	recv := make(chan TReceive)
	dpChan := &duplexChan[TSend, TReceive]{
		opts:   opts,
		ws:     ws,
		closed: &atomic.Bool{},
		done:   done,
		recv:   recv,
		send:   send,
	}
	go func() {
		<-done
		close(send)
		close(recv)
		ws.Close()
	}()
	go func() {
		for {
			select {
			case <-done:
				return
			case message := <-send:
				msg, serErr := opts.serializer.Serialize(message)
				if serErr != nil {
					dpChan.Close()
					return
				}
				err := ws.WriteMessage(websocket.BinaryMessage, msg)
				if err != nil {
					dpChan.Close()
					return
				}
			}
		}
	}()

	go func() {
		for {
			messageType, message, err := ws.ReadMessage()
			if err != nil {
				dpChan.Close()
				return
			}
			if messageType == websocket.CloseMessage {
				dpChan.Close()
				return
			}
			recvMsg, err := opts.serializer.Deserialize(message)
			if err != nil {
				dpChan.Close()
				return
			}
			recv <- recvMsg
		}
	}()
	return dpChan
}
