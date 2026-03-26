package main

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
)

// Peer manages an outgoing TCP connection to a remote clipall instance.
// It dials the remote, sends messages, and reconnects on failure.
type Peer struct {
	addr   string
	conn   net.Conn
	mu     sync.Mutex
	sendCh chan Message
}

func NewPeer(addr string) *Peer {
	return &Peer{
		addr:   addr,
		sendCh: make(chan Message, 16),
	}
}

// Run dials the peer and sends messages in a loop. Reconnects on failure.
// Blocks until ctx is cancelled.
func (p *Peer) Run(ctx context.Context) {
	for {
		if err := p.connect(ctx); err != nil {
			return // context cancelled
		}
		p.writeLoop(ctx)
		// writeLoop returned means connection lost, retry
		p.mu.Lock()
		if p.conn != nil {
			p.conn.Close()
			p.conn = nil
		}
		p.mu.Unlock()
	}
}

func (p *Peer) connect(ctx context.Context) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		dialer := net.Dialer{Timeout: 5 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", p.addr)
		if err == nil {
			p.mu.Lock()
			p.conn = conn
			p.mu.Unlock()
			log.Printf("[peer] connected to %s", p.addr)
			return nil
		}

		log.Printf("[peer] dial %s: %v, retry in %v", p.addr, err, backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (p *Peer) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-p.sendCh:
			data := Encode(msg)
			p.mu.Lock()
			conn := p.conn
			p.mu.Unlock()
			if conn == nil {
				return
			}
			if _, err := conn.Write(data); err != nil {
				log.Printf("[peer] write to %s: %v", p.addr, err)
				return
			}
		}
	}
}

// Send queues a message for sending. Non-blocking; drops if channel is full.
func (p *Peer) Send(msg Message) {
	select {
	case p.sendCh <- msg:
	default:
		log.Printf("[peer] send buffer full for %s, dropping", p.addr)
	}
}

// Close shuts down the peer connection.
func (p *Peer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}
