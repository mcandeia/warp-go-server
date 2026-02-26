// main_test.go
package server

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// --- Server routing tests ---

func TestConnectHandlerRequiresWebSocket(t *testing.T) {
	req, err := http.NewRequest("GET", "/_connect", nil)
	if err != nil {
		t.Fatal(err)
	}
	server := New()
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for non-WebSocket connection, got %v", status)
	}
}

func TestRequestHandlerUnknownHost(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	server := New()
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for unregistered host, got %v", status)
	}
}

func TestRequestHandlerRejectsWebSocket(t *testing.T) {
	req, err := http.NewRequest("GET", "/some-path", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("upgrade", "websocket")
	server := New()
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for WebSocket on request handler, got %v", status)
	}
}

func TestServerNew(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
}

func TestServerRoutes(t *testing.T) {
	s := New()
	mux := s.Routes()
	if mux == nil {
		t.Fatal("Routes() returned nil")
	}
}

// --- headerToMap tests ---

func TestHeaderToMapEmpty(t *testing.T) {
	result := headerToMap(http.Header{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestHeaderToMapSingleValue(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	result := headerToMap(header)
	if result["Content-Type"] != "application/json" {
		t.Errorf("expected 'application/json', got %q", result["Content-Type"])
	}
}

func TestHeaderToMapMultipleValues(t *testing.T) {
	header := http.Header{}
	header.Add("Accept", "text/html")
	header.Add("Accept", "application/json")
	result := headerToMap(header)
	val, ok := result["Accept"]
	if !ok {
		t.Fatal("expected 'Accept' key in result")
	}
	if val != "text/html,application/json" {
		t.Errorf("expected 'text/html,application/json', got %q", val)
	}
}

func TestHeaderToMapMultipleKeys(t *testing.T) {
	header := http.Header{}
	header.Set("X-Foo", "foo")
	header.Set("X-Bar", "bar")
	result := headerToMap(header)
	if result["X-Foo"] != "foo" {
		t.Errorf("expected 'foo', got %q", result["X-Foo"])
	}
	if result["X-Bar"] != "bar" {
		t.Errorf("expected 'bar', got %q", result["X-Bar"])
	}
}

// --- arraybuffer_serializer tests ---

func TestCreateAndParseRoundtrip(t *testing.T) {
	type meta struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	payload := []byte("hello binary data")
	m := meta{Type: "test", ID: "abc123"}

	encoded, err := createMessage(m, payload)
	if err != nil {
		t.Fatalf("createMessage failed: %v", err)
	}

	metaBts, dataBts, err := parseMessage(encoded)
	if err != nil {
		t.Fatalf("parseMessage failed: %v", err)
	}

	var decoded meta
	if err := json.Unmarshal(metaBts, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.Type != "test" || decoded.ID != "abc123" {
		t.Errorf("metadata mismatch: got %+v", decoded)
	}
	if string(dataBts) != string(payload) {
		t.Errorf("payload mismatch: got %q, want %q", dataBts, payload)
	}
}

func TestCreateMessageEmptyPayload(t *testing.T) {
	type meta struct {
		Type string `json:"type"`
	}
	m := meta{Type: "ping"}
	encoded, err := createMessage(m, nil)
	if err != nil {
		t.Fatalf("createMessage failed: %v", err)
	}

	// Verify the encoded length header is correct
	metaLen := binary.LittleEndian.Uint32(encoded[:4])
	metaBts := encoded[4 : 4+metaLen]

	var decoded meta
	if err := json.Unmarshal(metaBts, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.Type != "ping" {
		t.Errorf("expected type 'ping', got %q", decoded.Type)
	}
}

func TestParseMessageLengthHeader(t *testing.T) {
	type meta struct {
		Val string `json:"val"`
	}
	m := meta{Val: "test"}
	payload := []byte{1, 2, 3}
	encoded, err := createMessage(m, payload)
	if err != nil {
		t.Fatalf("createMessage failed: %v", err)
	}
	if len(encoded) < 4 {
		t.Fatal("encoded message too short")
	}
	metaLen := binary.LittleEndian.Uint32(encoded[:4])
	if int(metaLen)+4 > len(encoded) {
		t.Errorf("metadata length %d exceeds encoded length %d", metaLen, len(encoded))
	}
}

// --- UnmarshalClientMessage tests ---

func TestUnmarshalClientMessageRegister(t *testing.T) {
	data := []byte(`{"type":"register","id":"id1","apiKey":"key","domain":"example.com"}`)
	msg, err := UnmarshalClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reg, ok := msg.(RegisterMessage)
	if !ok {
		t.Fatalf("expected RegisterMessage, got %T", msg)
	}
	if reg.APIKey != "key" || reg.Domain != "example.com" || reg.ID != "id1" {
		t.Errorf("unexpected fields: %+v", reg)
	}
}

func TestUnmarshalClientMessageResponseStart(t *testing.T) {
	data := []byte(`{"type":"response-start","id":"req1","statusCode":200,"statusMessage":"OK","headers":{"Content-Type":"text/plain"}}`)
	msg, err := UnmarshalClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rs, ok := msg.(ResponseStartMessage)
	if !ok {
		t.Fatalf("expected ResponseStartMessage, got %T", msg)
	}
	if rs.StatusCode != 200 || rs.ID != "req1" {
		t.Errorf("unexpected fields: %+v", rs)
	}
	if rs.Headers["Content-Type"] != "text/plain" {
		t.Errorf("expected Content-Type header, got %v", rs.Headers)
	}
}

func TestUnmarshalClientMessageData(t *testing.T) {
	data := []byte(`{"type":"data","id":"req2","chunk":"aGVsbG8="}`)
	msg, err := UnmarshalClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dm, ok := msg.(*DataMessage)
	if !ok {
		t.Fatalf("expected *DataMessage, got %T", msg)
	}
	if dm.ID != "req2" {
		t.Errorf("expected ID 'req2', got %q", dm.ID)
	}
}

func TestUnmarshalClientMessageDataEnd(t *testing.T) {
	data := []byte(`{"type":"data-end","id":"req3"}`)
	msg, err := UnmarshalClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	de, ok := msg.(DataEndMessage)
	if !ok {
		t.Fatalf("expected DataEndMessage, got %T", msg)
	}
	if de.ID != "req3" {
		t.Errorf("expected ID 'req3', got %q", de.ID)
	}
}

func TestUnmarshalClientMessageWSOpened(t *testing.T) {
	data := []byte(`{"type":"ws-opened","id":"ws1"}`)
	msg, err := UnmarshalClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ws, ok := msg.(WSConnectionOpened)
	if !ok {
		t.Fatalf("expected WSConnectionOpened, got %T", msg)
	}
	if ws.GetID() != "ws1" {
		t.Errorf("expected ID 'ws1', got %q", ws.GetID())
	}
}

func TestUnmarshalClientMessageWSMessage(t *testing.T) {
	data := []byte(`{"type":"ws-message","id":"ws2","data":"dGVzdA=="}`)
	msg, err := UnmarshalClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wsm, ok := msg.(WSMessage)
	if !ok {
		t.Fatalf("expected WSMessage, got %T", msg)
	}
	if wsm.GetID() != "ws2" {
		t.Errorf("expected ID 'ws2', got %q", wsm.GetID())
	}
}

func TestUnmarshalClientMessageWSClosed(t *testing.T) {
	data := []byte(`{"type":"ws-closed","id":"ws3"}`)
	msg, err := UnmarshalClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wsc, ok := msg.(WSConnectionClosed)
	if !ok {
		t.Fatalf("expected WSConnectionClosed, got %T", msg)
	}
	if wsc.GetID() != "ws3" {
		t.Errorf("expected ID 'ws3', got %q", wsc.GetID())
	}
}

func TestUnmarshalClientMessageUnknownType(t *testing.T) {
	data := []byte(`{"type":"unknown-type","id":"x"}`)
	_, err := UnmarshalClientMessage(data)
	if err == nil {
		t.Fatal("expected error for unknown message type, got nil")
	}
}

func TestUnmarshalClientMessageInvalidJSON(t *testing.T) {
	data := []byte(`{not valid json}`)
	_, err := UnmarshalClientMessage(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// --- Message GetID tests ---

func TestMessageGetIDs(t *testing.T) {
	tests := []struct {
		name string
		msg  ClientMessage
		id   string
	}{
		{"RegisterMessage", RegisterMessage{ID: "r1"}, "r1"},
		{"ResponseStartMessage", ResponseStartMessage{ID: "rs1"}, "rs1"},
		{"DataMessage", &DataMessage{ID: "d1"}, "d1"},
		{"DataEndMessage", DataEndMessage{ID: "de1"}, "de1"},
		{"WSConnectionOpened", WSConnectionOpened{ID: "wo1"}, "wo1"},
		{"WSMessage", WSMessage{ID: "wm1"}, "wm1"},
		{"WSConnectionClosed", WSConnectionClosed{ID: "wc1"}, "wc1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.GetID(); got != tt.id {
				t.Errorf("GetID() = %q, want %q", got, tt.id)
			}
		})
	}
}

// --- DataMessage Payload tests ---

func TestDataMessagePayload(t *testing.T) {
	dm := &DataMessage{Chunk: []byte("test data")}
	if string(dm.Payload()) != "test data" {
		t.Errorf("expected 'test data', got %q", dm.Payload())
	}
}

func TestDataMessageWithPayload(t *testing.T) {
	dm := &DataMessage{}
	dm.WithPayload([]byte("new payload"))
	if string(dm.Chunk) != "new payload" {
		t.Errorf("expected 'new payload', got %q", dm.Chunk)
	}
}

func TestRequestDataMessagePayload(t *testing.T) {
	rdm := &RequestDataMessage{Chunk: []byte("req data")}
	if string(rdm.Payload()) != "req data" {
		t.Errorf("expected 'req data', got %q", rdm.Payload())
	}
}

func TestRequestDataMessageWithPayload(t *testing.T) {
	rdm := &RequestDataMessage{}
	rdm.WithPayload([]byte("set payload"))
	if string(rdm.Chunk) != "set payload" {
		t.Errorf("expected 'set payload', got %q", rdm.Chunk)
	}
}

// --- WritableStream tests ---

func TestWritableStreamWriteAndRead(t *testing.T) {
	ws := NewWritableStream()
	data := []byte("hello world")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ws.Write(data)
		ws.Close()
	}()

	buf := make([]byte, 64)
	n, err := ws.Read(buf)
	wg.Wait()

	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf[:n]) != string(data) {
		t.Errorf("expected %q, got %q", data, buf[:n])
	}
}

func TestWritableStreamWriteAfterClose(t *testing.T) {
	ws := NewWritableStream()
	ws.Close()
	_, err := ws.Write([]byte("data"))
	if err != io.ErrClosedPipe {
		t.Errorf("expected io.ErrClosedPipe, got %v", err)
	}
}

func TestWritableStreamReadAfterCloseEmpty(t *testing.T) {
	ws := NewWritableStream()
	ws.Close()
	buf := make([]byte, 8)
	_, err := ws.Read(buf)
	if err != io.EOF {
		t.Errorf("expected io.EOF on closed empty stream, got %v", err)
	}
}

func TestWritableStreamMultipleWrites(t *testing.T) {
	ws := NewWritableStream()

	go func() {
		ws.Write([]byte("first"))
		ws.Write([]byte("second"))
		ws.Close()
	}()

	var result []byte
	buf := make([]byte, 16)
	for {
		n, err := ws.Read(buf)
		result = append(result, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}

	expected := "firstsecond"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// --- RequestDataMessage JSON serialization test ---

func TestRequestDataMessageMarshalJSON(t *testing.T) {
	rdm := &RequestDataMessage{
		Type:  "request-data",
		ID:    "msg1",
		Chunk: []byte("should not appear in json"),
	}
	data, err := json.Marshal(rdm)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	// Chunk should not be in the JSON output (uses ChunklessRequestDataMessage)
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if _, ok := result["chunk"]; ok {
		t.Error("expected 'chunk' field to be absent from JSON output")
	}
	if result["type"] != "request-data" {
		t.Errorf("expected type 'request-data', got %v", result["type"])
	}
	if result["id"] != "msg1" {
		t.Errorf("expected id 'msg1', got %v", result["id"])
	}
}
