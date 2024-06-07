package server

import (
	"encoding/json"
)

// jsonSerializer is a struct for serializing and deserializing JSON messages
type jsonSerializer[TSend, TReceive Chunked] struct{}

// Serialize is a method to serialize a message
func (s jsonSerializer[TSend, TReceive]) Serialize(snd TSend) ([]byte, error) {
	return json.Marshal(snd)
}

// Deserialize is a method to deserialize a message
func (s jsonSerializer[TSend, TReceive]) Deserialize(data []byte) (TReceive, error) {
	var rcv TReceive
	err := json.Unmarshal(data, &rcv)
	return rcv, err
}
