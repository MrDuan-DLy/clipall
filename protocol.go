package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

const ProtocolVersion = 1

type MessageType byte

const (
	TypeText  MessageType = 0x01
	TypeImage MessageType = 0x02
	TypePing  MessageType = 0x03
	TypePong  MessageType = 0x04
)

const MaxPayloadSize = 10 * 1024 * 1024 // 10MB
const headerSize = 14

type Message struct {
	Version   byte
	Type      MessageType
	ContentID uint64
	Payload   []byte
}

// Encode serializes a Message to wire format.
// Always sets Version to ProtocolVersion regardless of the input value.
func Encode(msg Message) []byte {
	payloadLen := uint32(len(msg.Payload))
	buf := make([]byte, headerSize+int(payloadLen))

	buf[0] = ProtocolVersion
	buf[1] = byte(msg.Type)
	binary.BigEndian.PutUint64(buf[2:10], msg.ContentID)
	binary.BigEndian.PutUint32(buf[10:14], payloadLen)
	copy(buf[14:], msg.Payload)

	return buf
}

// Decode reads a single Message from r.
// Returns an error if the version does not match ProtocolVersion, the message
// type is unknown, or the payload length exceeds MaxPayloadSize.
func Decode(r io.Reader) (Message, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return Message{}, fmt.Errorf("reading header: %w", err)
	}

	version := header[0]
	if version != ProtocolVersion {
		return Message{}, fmt.Errorf("version mismatch: got %d, want %d", version, ProtocolVersion)
	}

	msgType := MessageType(header[1])
	switch msgType {
	case TypeText, TypeImage, TypePing, TypePong:
	default:
		return Message{}, fmt.Errorf("unknown message type: 0x%02x", msgType)
	}

	contentID := binary.BigEndian.Uint64(header[2:10])
	payloadLen := binary.BigEndian.Uint32(header[10:14])

	if payloadLen > MaxPayloadSize {
		return Message{}, fmt.Errorf("payload too large: %d bytes exceeds max %d", payloadLen, MaxPayloadSize)
	}

	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return Message{}, fmt.Errorf("reading payload: %w", err)
		}
	}

	return Message{
		Version:   version,
		Type:      msgType,
		ContentID: contentID,
		Payload:   payload,
	}, nil
}
