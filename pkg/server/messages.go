package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type noopData struct{}

func (noopData) Payload() []byte {
	return nil
}

func (noopData) WithPayload([]byte) {}

type RequestObject struct {
	ID               string
	RequestObject    *http.Request
	ResponseObject   http.ResponseWriter
	ResponseBodyChan chan []byte
	WebSocketChan    chan []byte
}

type RegisterMessage struct {
	noopData
	ID     string `json:"id"`
	Type   string `json:"type"` // should always be "register"
	APIKey string `json:"apiKey"`
	Domain string `json:"domain"`
}

type ResponseStartMessage struct {
	noopData
	Type          string            `json:"type"` // should always be "response-start"
	ID            string            `json:"id"`
	StatusCode    int               `json:"statusCode"`
	StatusMessage string            `json:"statusMessage"`
	Headers       map[string]string `json:"headers"`
}

type DataMessage struct {
	Type  string `json:"type"` // should always be "data"
	ID    string `json:"id"`
	Chunk []byte `json:"chunk"`
}

type DataEndMessage struct {
	noopData
	Type  string `json:"type"` // should always be "data-end"
	ID    string `json:"id"`
	Error any    `json:"error"`
}

type WSConnectionOpened struct {
	noopData
	Type string `json:"type"` // should always be "ws-opened"
	ID   string `json:"id"`
}

type WSMessage struct {
	noopData
	Type string `json:"type"` // should always be "ws-message"
	ID   string `json:"id"`
	Data []byte `json:"data"`
}

type WSConnectionClosed struct {
	noopData
	Type string `json:"type"` // should always be "ws-closed"
	ID   string `json:"id"`
}

// Chunked is an interface for chunking messages
type Chunked interface {
	// Chunk is a method to return a chunk of data
	Payload() []byte
	WithPayload([]byte)
}

type ClientMessage interface {
	Chunked
	Handle(*ServerConnState) error
	GetID() string
}

func (c *DataMessage) Payload() []byte {
	return c.Chunk
}

func (c *DataMessage) WithPayload(data []byte) {
	c.Chunk = data
}

func (WSMessage) Handle(conn *ServerConnState) error {
	return nil
}
func (WSConnectionClosed) Handle(conn *ServerConnState) error {
	return nil
}
func (WSConnectionOpened) Handle(conn *ServerConnState) error {
	return nil
}
func (s RegisterMessage) Handle(conn *ServerConnState) error {
	fmt.Printf("received register message %s %s %s\n", s.APIKey, s.Domain, s.ID)
	conn.LinkHost(s.Domain)
	return conn.Ch.Send(RegisteredMessage{
		Type:   "registered",
		Domain: s.Domain,
		ID:     s.ID,
	})
}
func (s ResponseStartMessage) Handle(conn *ServerConnState) error {
	val, ok := conn.OngoingRequests.Load(s.ID)
	if !ok {
		return fmt.Errorf("no ongoing request found for id %s", s.ID)
	}
	req := val.(*RequestObject)
	for headerKey, headerValue := range s.Headers {
		req.ResponseObject.Header().Set(headerKey, headerValue)
	}
	req.ResponseObject.Header().Set("transfer-encoding", "chunked")
	req.ResponseObject.WriteHeader(s.StatusCode)

	return nil
}

func (s DataMessage) Handle(conn *ServerConnState) error {
	val, ok := conn.OngoingRequests.Load(s.ID)
	if !ok {
		return fmt.Errorf("no ongoing request found for id %s", s.ID)
	}
	req := val.(*RequestObject)
	req.ResponseBodyChan <- s.Chunk
	return nil
}

func (s DataEndMessage) Handle(conn *ServerConnState) error {
	val, ok := conn.OngoingRequests.Load(s.ID)
	if !ok {
		return fmt.Errorf("no ongoing request found for id %s", s.ID)
	}
	req := val.(*RequestObject)
	close(req.ResponseBodyChan)
	return nil
}

func (s WSMessage) GetID() string {
	return s.ID
}

func (s WSConnectionClosed) GetID() string {
	return s.ID
}

func (s WSConnectionOpened) GetID() string {
	return s.ID
}

func (s RegisterMessage) GetID() string {
	return s.ID
}

func (s ResponseStartMessage) GetID() string {
	return s.ID
}

func (s DataMessage) GetID() string {
	return s.ID
}

func (s DataEndMessage) GetID() string {
	return s.ID
}

func UnmarshalClientMessage(data []byte) (ClientMessage, error) {
	var msgType struct {
		Type string `json:"type"`
	}

	// Unmarshal the "type" field
	if err := json.Unmarshal(data, &msgType); err != nil {
		return nil, err
	}

	// Unmarshal into the appropriate struct based on the type
	switch msgType.Type {
	case "register":
		var msg RegisterMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "response-start":
		var msg ResponseStartMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "data":
		var msg DataMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case "data-end":
		var msg DataEndMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "ws-opened":
		var msg WSConnectionOpened
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "ws-message":
		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "ws-closed":
		var msg WSConnectionClosed
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType.Type)
	}
}

type serverMessage struct {
}

func (serverMessage) IsServerMessage() {}

type RequestStartMessage struct {
	serverMessage
	noopData
	Type    string            `json:"type"` // should always be "request-start"
	Domain  string            `json:"domain"`
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	HasBody bool              `json:"hasBody"`
}

type RequestDataEndMessage struct {
	serverMessage
	noopData
	Type string `json:"type"` // should always be "request-end"
	ID   string `json:"id"`
}

func (c *RequestDataMessage) Payload() []byte {
	return c.Chunk
}

func (c *RequestDataMessage) WithPayload(data []byte) {
	c.Chunk = data
}

type RequestDataMessage struct {
	serverMessage
	noopData
	Type  string `json:"type"` // should always be "request-data"
	ID    string `json:"id"`
	Chunk []byte `json:"chunk"`
}

type ChunklessRequestDataMessage struct {
	serverMessage
	Type string `json:"type"` // should always be "request-data"
	ID   string `json:"id"`
}

func (p *RequestDataMessage) MarshalJSON() ([]byte, error) {
	res := ChunklessRequestDataMessage{
		Type: p.Type,
		ID:   p.ID,
	}

	return json.Marshal(&res)
}

type RegisteredMessage struct {
	serverMessage
	noopData
	Type   string `json:"type"` // should always be "registered"
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

type ErrorMessage struct {
	serverMessage
	Type    string `json:"type"` // should always be "error"
	Message string `json:"message"`
}

type ServerMessage interface {
	Chunked
	IsServerMessage()
}
