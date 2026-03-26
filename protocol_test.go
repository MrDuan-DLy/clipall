package main

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeText(t *testing.T) {
	original := Message{
		Type:      TypeText,
		ContentID: 42,
		Payload:   []byte("hello clipboard"),
	}

	encoded := Encode(original)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Version != ProtocolVersion {
		t.Errorf("Version = %d, want %d", decoded.Version, ProtocolVersion)
	}
	if decoded.Type != TypeText {
		t.Errorf("Type = %d, want %d", decoded.Type, TypeText)
	}
	if decoded.ContentID != 42 {
		t.Errorf("ContentID = %d, want 42", decoded.ContentID)
	}
	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Errorf("Payload = %q, want %q", decoded.Payload, original.Payload)
	}
}

func TestEncodeDecodeImage(t *testing.T) {
	payload := make([]byte, 64*1024) // 64KB fake image
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	original := Message{
		Type:      TypeImage,
		ContentID: 99,
		Payload:   payload,
	}

	encoded := Encode(original)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != TypeImage {
		t.Errorf("Type = %d, want %d", decoded.Type, TypeImage)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Errorf("Payload mismatch: got %d bytes, want %d bytes", len(decoded.Payload), len(payload))
	}
}

func TestEncodeDecodeEmptyPayload(t *testing.T) {
	original := Message{
		Type:      TypePing,
		ContentID: 1,
		Payload:   []byte{},
	}

	encoded := Encode(original)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != TypePing {
		t.Errorf("Type = %d, want %d", decoded.Type, TypePing)
	}
	if len(decoded.Payload) != 0 {
		t.Errorf("Payload length = %d, want 0", len(decoded.Payload))
	}
}

func TestDecodeVersionMismatch(t *testing.T) {
	msg := Message{
		Type:      TypeText,
		ContentID: 1,
		Payload:   []byte("test"),
	}

	encoded := Encode(msg)
	encoded[0] = 99 // corrupt version byte

	_, err := Decode(bytes.NewReader(encoded))
	if err == nil {
		t.Fatal("expected error for version mismatch, got nil")
	}
}

func TestDecodePayloadTooLarge(t *testing.T) {
	msg := Message{
		Type:      TypeText,
		ContentID: 1,
		Payload:   []byte("small"),
	}

	encoded := Encode(msg)

	// Overwrite PayloadLen with a value exceeding MaxPayloadSize.
	// MaxPayloadSize is 10*1024*1024 = 10485760, so use 10485761.
	encoded[10] = 0x00
	encoded[11] = 0xA0
	encoded[12] = 0x00
	encoded[13] = 0x01

	_, err := Decode(bytes.NewReader(encoded))
	if err == nil {
		t.Fatal("expected error for payload too large, got nil")
	}
}

func TestEncodeDecodePreservesContentID(t *testing.T) {
	contentID := uint64(0xDEADBEEFCAFEBABE)

	original := Message{
		Type:      TypeText,
		ContentID: contentID,
		Payload:   []byte("id test"),
	}

	encoded := Encode(original)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ContentID != contentID {
		t.Errorf("ContentID = 0x%X, want 0x%X", decoded.ContentID, contentID)
	}
}
