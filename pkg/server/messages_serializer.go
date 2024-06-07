package server

import (
	"fmt"
)

// messageSerializer is a struct for serializing and deserializing messages
type messageSerializer struct {
}

// Serialize is a method to serialize a message
func (s messageSerializer) Serialize(snd ServerMessage) ([]byte, error) {
	return createMessage(snd, snd.Payload())
}

// Deserialize is a method to deserialize a message
func (s messageSerializer) Deserialize(data []byte) (ClientMessage, error) {
	metadataBts, dataBts, err := parseMessage(data)
	if err != nil {
		var recv ClientMessage
		return recv, fmt.Errorf("error parsing message: %v", err)
	}

	clientMessage, err := UnmarshalClientMessage(metadataBts)
	if err != nil {
		var recv ClientMessage
		return recv, fmt.Errorf("error unmarshalling metadata: %v", err)
	}

	clientMessage.WithPayload(dataBts)
	return clientMessage, nil
}
