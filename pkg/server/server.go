package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ServerConnState struct {
	ClientID        string
	Ch              DuplexChan[ServerMessage, ClientMessage]
	OngoingRequests sync.Map
	LinkHost        func(string)
}

// ServerOpts is a struct to hold server options
type ServerOpts struct {
}

// Server is a struct to hold server options
type Server struct {
	serverStates   sync.Map
	hostToClientID sync.Map
}

// Routes is a method to return a ServeMux
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/_connect", s.onWSConnect)
	mux.HandleFunc("/*", s.onRequest)
	return mux
}

// headerToMap transforms http.Header into a map[string]string
func headerToMap(header http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range header {
		// Join multiple values with a comma
		result[key] = strings.Join(values, ",")
	}
	return result
}
func (s *Server) onRequest(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("upgrade") == "websocket" {
		http.Error(w, "Websocket not supported", http.StatusBadRequest)
		return
	}
	host := r.Host
	clientID, ok := s.hostToClientID.Load(host)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	serverStateAny, okStates := s.serverStates.Load(clientID)
	if !okStates {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	serverState := serverStateAny.(*ServerConnState)
	messageID := uuid.New().String()
	hasBody := r.Body != nil
	respBodyChan := make(chan []byte)
	if !hasBody {
		close(respBodyChan)
	}
	responseEnd := sync.WaitGroup{}
	responseEnd.Add(1)
	go func() {
		defer responseEnd.Done()
		for {
			select {
			case <-r.Context().Done():
				return
			case chunk, ok := <-respBodyChan:
				if !ok {
					return
				}
				_, err := w.Write(chunk)
				if err != nil {
					return
				}
			}
		}
	}()
	serverState.OngoingRequests.Store(messageID, &RequestObject{
		ID:               messageID,
		RequestObject:    r,
		ResponseObject:   w,
		ResponseBodyChan: respBodyChan,
	})

	if err := serverState.Ch.Send(&RequestStartMessage{
		Type:    "request-start",
		Domain:  host,
		ID:      messageID,
		Method:  r.Method,
		HasBody: hasBody,
		URL:     r.URL.Path + r.URL.RawQuery,
		Headers: headerToMap(r.Header),
	}); err != nil {
		fmt.Fprintf(w, "error %v", err)
		return
	}
	defer r.Body.Close()

	buf := make([]byte, 1024)
	for {
		_, err := r.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				// End of the body, break the loop
				break
			}
			return
		}
		if err := serverState.Ch.Send(&RequestDataMessage{
			Chunk: buf,
			ID:    messageID,
			Type:  "request-data",
		}); err != nil {
			return
		}
	}
	err := serverState.Ch.Send(&RequestDataEndMessage{
		ID:   messageID,
		Type: "request-end",
	})
	if err != nil {
		return
	}
	responseEnd.Wait()
}

// Shows how to use templates with template functions and data
func (s *Server) onWSConnect(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("upgrade") != "websocket" {
		http.Error(w, "Expected WebSocket", http.StatusBadRequest)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade websocket connection", http.StatusInternalServerError)
		return
	}
	ch := NewDuplexChan(ws, WithSerializer(messageSerializer{}))
	defer ch.Close()

	clientID := uuid.New().String()
	hosts := []string{}
	recv := ch.Recv()
	state := ServerConnState{
		ClientID: clientID,
		Ch:       ch,
		LinkHost: func(host string) {
			hosts = append(hosts, host)
			s.hostToClientID.Store(host, clientID)
		},
	}
	s.serverStates.Store(clientID, &state)
	defer s.serverStates.Delete(clientID)
	defer func() {
		for _, host := range hosts {
			s.hostToClientID.CompareAndDelete(host, clientID)
		}
	}()
	wg := sync.WaitGroup{}
	defer wg.Wait()
	for {
		select {
		case <-ch.Closed():
			return
		case <-r.Context().Done():
			return
		case message := <-recv:
			if message == nil {
				return
			}
			err := message.Handle(&state)
			if err != nil {
				fmt.Printf("error handling message: %v", err)
				state.OngoingRequests.Delete(message.GetID())
			}
		}
	}
}

// New is a function to return a new Server
func New() *Server {
	return &Server{}
}
