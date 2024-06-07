package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// parseMessage parses the received message into metadata and binary data
func parseMessage(data []byte) ([]byte, []byte, error) {

	// Read metadata length (4 bytes, little-endian)
	metadataLength := binary.LittleEndian.Uint32(data[:4])

	// Read metadata
	metadataBytes := data[4 : 4+metadataLength]

	// Read binary data
	binaryData := data[4+metadataLength:]

	return metadataBytes, binaryData, nil
}

// createMessage combines metadata and binary data into a single byte slice
func createMessage(metadata any, binaryData []byte) ([]byte, error) {
	// Serialize metadata to JSON
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("error marshalling metadata: %v", err)
	}

	metadataLength := uint32(len(metadataBytes))

	// Create a buffer to hold the metadata length, metadata, and binary data
	var buffer bytes.Buffer

	// Write the metadata length (4 bytes, little-endian)
	err = binary.Write(&buffer, binary.LittleEndian, metadataLength)
	if err != nil {
		return nil, fmt.Errorf("error writing metadata length: %v", err)
	}

	// Write the metadata
	_, err = buffer.Write(metadataBytes)
	if err != nil {
		return nil, fmt.Errorf("error writing metadata: %v", err)
	}

	// Write the binary data
	_, err = buffer.Write(binaryData)
	if err != nil {
		return nil, fmt.Errorf("error writing binary data: %v", err)
	}

	return buffer.Bytes(), nil
}

// arrayBufferSerializer is a struct for serializing and deserializing ArrayBuffer messages
type arrayBufferSerializer[TSend, TReceive Chunked] struct {
}

// Serialize is a method to serialize a message
func (s arrayBufferSerializer[TSend, TReceive]) Serialize(snd TSend) ([]byte, error) {
	return createMessage(snd, snd.Payload())
}

// Deserialize is a method to deserialize a message
func (s arrayBufferSerializer[TSend, TReceive]) Deserialize(data []byte) (TReceive, error) {
	metadataBts, dataBts, err := parseMessage(data)
	var recv TReceive
	if err != nil {
		return recv, fmt.Errorf("error parsing message: %v", err)
	}

	if err := json.Unmarshal(metadataBts, &recv); err != nil {
		return recv, fmt.Errorf("error unmarshalling metadata: %v", err)
	}

	recv.WithPayload(dataBts)
	return recv, nil
}
