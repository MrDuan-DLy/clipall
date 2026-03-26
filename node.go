package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/cespare/xxhash/v2"
)

// Node is the core orchestrator. It watches the local clipboard, sends changes
// to peers, and writes incoming clipboard data from peers.
const writeCooldown = 500 * time.Millisecond

type Node struct {
	listenPort int
	peers      []*Peer
	incoming   chan Message
	ring       RingBuffer
	lastWrite  time.Time // when we last wrote to clipboard from a remote
}

func NewNode(listenPort int, peerAddrs []string) *Node {
	n := &Node{
		listenPort: listenPort,
		incoming:   make(chan Message, 32),
	}
	for _, addr := range peerAddrs {
		n.peers = append(n.peers, NewPeer(addr))
	}
	return n
}

// Run starts the node: listener, peer connections, clipboard watcher, and the
// main event loop. Blocks until ctx is cancelled.
func (n *Node) Run(ctx context.Context) error {
	if err := initClipboard(); err != nil {
		return fmt.Errorf("clipboard init: %w", err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", n.listenPort))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()
	log.Printf("[node] listening on :%d", n.listenPort)

	// Accept incoming connections (peers dialing us).
	go n.acceptLoop(ctx, listener)

	// Dial all configured peers (our outgoing connections for sending).
	for _, p := range n.peers {
		go p.Run(ctx)
	}

	// Watch local clipboard for changes.
	clipCh := watchText(ctx)
	log.Printf("[node] watching clipboard, %d peer(s) configured", len(n.peers))

	// Main event loop.
	for {
		select {
		case <-ctx.Done():
			for _, p := range n.peers {
				p.Close()
			}
			return nil

		case data := <-clipCh:
			if len(data) == 0 {
				continue
			}
			// Cooldown: ignore Watch events shortly after we wrote to clipboard.
			// This prevents echo-back when the clipboard round-trip (write→read)
			// produces slightly different bytes (e.g. UTF-8→UTF-16→UTF-8 on Windows).
			if time.Since(n.lastWrite) < writeCooldown {
				log.Printf("[node] ignoring clipboard event during cooldown (%d bytes)", len(data))
				continue
			}
			id := xxhash.Sum64(data)
			if n.ring.Contains(id) {
				log.Printf("[node] ignoring clipboard event (in ring buffer), hash=%016x", id)
				continue
			}
			msg := Message{
				Type:      TypeText,
				ContentID: id,
				Payload:   data,
			}
			for _, p := range n.peers {
				p.Send(msg)
			}
			log.Printf("[node] sent %d bytes to %d peer(s), hash=%016x, preview=%s",
				len(data), len(n.peers), id, debugPreview(data))

		case msg := <-n.incoming:
			if n.ring.Contains(msg.ContentID) {
				log.Printf("[node] ignoring incoming (in ring buffer), hash=%016x", msg.ContentID)
				continue
			}
			n.ring.Add(msg.ContentID)
			switch msg.Type {
			case TypeText:
				n.lastWrite = time.Now()
				writeText(msg.Payload)
				// Verify: read back and check if clipboard matches what we wrote.
				readback := readText()
				if string(readback) == string(msg.Payload) {
					log.Printf("[node] received %d bytes, write verified OK, hash=%016x, preview=%s",
						len(msg.Payload), msg.ContentID, debugPreview(msg.Payload))
				} else {
					log.Printf("[node] WARNING: clipboard write mismatch! wrote %d bytes, read back %d bytes",
						len(msg.Payload), len(readback))
					log.Printf("[node]   wrote:    %s", debugHex(msg.Payload))
					log.Printf("[node]   readback: %s", debugHex(readback))
					// Add readback hash to ring too, so the mismatched Watch event is caught.
					n.ring.Add(xxhash.Sum64(readback))
				}
			default:
				log.Printf("[node] ignoring message type 0x%02x", msg.Type)
			}
		}
	}
}

// acceptLoop accepts incoming TCP connections and starts a reader for each.
func (n *Node) acceptLoop(ctx context.Context, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[node] accept: %v", err)
			continue
		}
		log.Printf("[node] accepted connection from %s", conn.RemoteAddr())
		go n.handleConn(ctx, conn)
	}
}

// handleConn reads messages from an accepted connection until it closes or errors.
func (n *Node) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	for {
		msg, err := Decode(conn)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[node] connection from %s closed: %v", conn.RemoteAddr(), err)
			return
		}
		select {
		case n.incoming <- msg:
		case <-ctx.Done():
			return
		}
	}
}

// debugPreview returns the first 40 chars of data for logging.
func debugPreview(data []byte) string {
	s := string(data)
	if len(s) > 40 {
		return fmt.Sprintf("%q...", s[:40])
	}
	return fmt.Sprintf("%q", s)
}

// debugHex returns a hex dump of the first 32 bytes.
func debugHex(data []byte) string {
	if len(data) > 32 {
		return hex.EncodeToString(data[:32]) + "..."
	}
	return hex.EncodeToString(data)
}
