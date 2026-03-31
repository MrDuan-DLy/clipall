package main

import (
	"testing"

	"github.com/cespare/xxhash/v2"
)

func TestNewNodeInitialization(t *testing.T) {
	n := NewNode(9876, []string{"host-a:9876", "host-b:9876"})

	if n.listenPort != 9876 {
		t.Errorf("listenPort = %d, want 9876", n.listenPort)
	}
	if len(n.peers) != 2 {
		t.Fatalf("len(peers) = %d, want 2", len(n.peers))
	}
	if n.peers[0].addr != "host-a:9876" {
		t.Errorf("peers[0].addr = %q, want %q", n.peers[0].addr, "host-a:9876")
	}
	if n.peers[1].addr != "host-b:9876" {
		t.Errorf("peers[1].addr = %q, want %q", n.peers[1].addr, "host-b:9876")
	}
	if n.incoming == nil {
		t.Fatal("incoming channel is nil")
	}
}

func TestNewNodeNoPeers(t *testing.T) {
	n := NewNode(5555, nil)

	if n.listenPort != 5555 {
		t.Errorf("listenPort = %d, want 5555", n.listenPort)
	}
	if len(n.peers) != 0 {
		t.Fatalf("len(peers) = %d, want 0", len(n.peers))
	}
}

func TestIncomingTextRingBufferDedup(t *testing.T) {
	n := NewNode(9876, nil)

	payload := []byte("hello clipboard")
	id := xxhash.Sum64(payload)

	msg := Message{
		Type:      TypeText,
		ContentID: id,
		Payload:   payload,
	}

	// First time: not in ring.
	if n.ring.Contains(msg.ContentID) {
		t.Fatal("ring should not contain ID before Add")
	}

	// Simulate what the incoming handler does: add to ring.
	n.ring.Add(msg.ContentID)

	// Second time: ring should reject the duplicate.
	if !n.ring.Contains(msg.ContentID) {
		t.Fatal("ring should contain ID after Add")
	}
}

func TestIncomingImageRingBufferDedup(t *testing.T) {
	n := NewNode(9876, nil)

	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	id := xxhash.Sum64(payload)

	msg := Message{
		Type:      TypeImage,
		ContentID: id,
		Payload:   payload,
	}

	if n.ring.Contains(msg.ContentID) {
		t.Fatal("ring should not contain image ID before Add")
	}

	n.ring.Add(msg.ContentID)

	if !n.ring.Contains(msg.ContentID) {
		t.Fatal("ring should contain image ID after Add")
	}
}

func TestIncomingMixedTypeDedup(t *testing.T) {
	n := NewNode(9876, nil)

	textPayload := []byte("some text")
	textID := xxhash.Sum64(textPayload)

	imgPayload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	imgID := xxhash.Sum64(imgPayload)

	// Simulate receiving a text message, then an image message.
	n.ring.Add(textID)
	n.ring.Add(imgID)

	// Both should now be detected as duplicates.
	if !n.ring.Contains(textID) {
		t.Fatal("ring should contain text ID")
	}
	if !n.ring.Contains(imgID) {
		t.Fatal("ring should contain image ID")
	}

	// A new, different payload should not be blocked.
	otherPayload := []byte("different content")
	otherID := xxhash.Sum64(otherPayload)
	if n.ring.Contains(otherID) {
		t.Fatal("ring should not contain ID for content never added")
	}
}

func TestIncomingChannelCapacity(t *testing.T) {
	n := NewNode(9876, nil)

	// The incoming channel has a buffer of 32. Fill it without blocking.
	for i := 0; i < 32; i++ {
		msg := Message{
			Type:      TypeText,
			ContentID: uint64(i),
			Payload:   []byte("test"),
		}
		select {
		case n.incoming <- msg:
		default:
			t.Fatalf("incoming channel blocked at message %d, expected buffer of 32", i)
		}
	}

	// The 33rd send should not block (we verify by using select/default).
	select {
	case n.incoming <- Message{Type: TypeText, ContentID: 999, Payload: []byte("overflow")}:
		t.Fatal("incoming channel accepted message beyond capacity 32 without a reader")
	default:
		// Expected: channel is full.
	}
}

func TestIncomingDrainBothTypes(t *testing.T) {
	n := NewNode(9876, nil)

	textMsg := Message{
		Type:      TypeText,
		ContentID: 1,
		Payload:   []byte("hello"),
	}
	imgMsg := Message{
		Type:      TypeImage,
		ContentID: 2,
		Payload:   []byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG SOI marker
	}

	n.incoming <- textMsg
	n.incoming <- imgMsg

	// Drain and verify both types arrive in order.
	got1 := <-n.incoming
	if got1.Type != TypeText {
		t.Errorf("first message type = 0x%02x, want 0x%02x (TypeText)", got1.Type, TypeText)
	}
	if got1.ContentID != 1 {
		t.Errorf("first message ContentID = %d, want 1", got1.ContentID)
	}

	got2 := <-n.incoming
	if got2.Type != TypeImage {
		t.Errorf("second message type = 0x%02x, want 0x%02x (TypeImage)", got2.Type, TypeImage)
	}
	if got2.ContentID != 2 {
		t.Errorf("second message ContentID = %d, want 2", got2.ContentID)
	}
}

func TestRingBufferPreventsLoopAcrossTypes(t *testing.T) {
	// Simulate the full loop-prevention scenario: a node receives content,
	// adds it to the ring, then sees the same content from the clipboard
	// watcher. Both text and image types should be caught.
	n := NewNode(9876, nil)

	textData := []byte("clipboard text")
	imgData := make([]byte, 512)
	for i := range imgData {
		imgData[i] = byte(i % 256)
	}

	textID := xxhash.Sum64(textData)
	imgID := xxhash.Sum64(imgData)

	// Simulate incoming handler adding to ring.
	n.ring.Add(textID)
	n.ring.Add(imgID)

	// Simulate clipboard watcher producing the same content.
	// The node's event loop checks ring.Contains before sending to peers.
	if !n.ring.Contains(xxhash.Sum64(textData)) {
		t.Error("text echo should be caught by ring buffer")
	}
	if !n.ring.Contains(xxhash.Sum64(imgData)) {
		t.Error("image echo should be caught by ring buffer")
	}
}

func TestPeerSendQueueing(t *testing.T) {
	p := NewPeer("fake-host:9876")

	msg := Message{
		Type:      TypeImage,
		ContentID: 42,
		Payload:   []byte{0x89, 0x50, 0x4E, 0x47},
	}

	// Send should succeed (queued in the channel buffer of 16).
	p.Send(msg)

	select {
	case got := <-p.sendCh:
		if got.Type != TypeImage {
			t.Errorf("queued message type = 0x%02x, want 0x%02x", got.Type, TypeImage)
		}
		if got.ContentID != 42 {
			t.Errorf("queued message ContentID = %d, want 42", got.ContentID)
		}
	default:
		t.Fatal("expected message in peer send channel")
	}
}

func TestPeerSendDropsWhenFull(t *testing.T) {
	p := NewPeer("fake-host:9876")

	// Fill the send channel (capacity 16).
	for i := 0; i < 16; i++ {
		p.Send(Message{Type: TypeText, ContentID: uint64(i), Payload: []byte("x")})
	}

	// The 17th message should be silently dropped, not block.
	p.Send(Message{Type: TypeText, ContentID: 999, Payload: []byte("dropped")})

	// Verify all 16 buffered messages are the originals.
	for i := 0; i < 16; i++ {
		got := <-p.sendCh
		if got.ContentID != uint64(i) {
			t.Errorf("message %d: ContentID = %d, want %d", i, got.ContentID, i)
		}
	}
}
