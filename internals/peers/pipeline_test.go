package peers

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/seeniolabode/gotorrent/internals/types"
)

func readWire(t *testing.T, c net.Conn) *Message {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(time.Second))
	p := &PeerConnection{conn: c}
	m, err := p.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func writeWire(t *testing.T, c net.Conn, id byte, payload []byte) {
	t.Helper()
	c.SetWriteDeadline(time.Now().Add(time.Second))
	b := make([]byte, 5+len(payload))
	binary.BigEndian.PutUint32(b, uint32(1+len(payload)))
	b[4] = id
	copy(b[5:], payload)
	if _, err := c.Write(b); err != nil {
		t.Fatal(err)
	}
}
func piecePayload(begin int) []byte {
	b := make([]byte, 9)
	binary.BigEndian.PutUint32(b[4:], uint32(begin))
	b[8] = 'x'
	return b
}

func TestPipelineRefillsOutOfOrderAndDiscardsReleasedReplies(t *testing.T) {
	client, remote := net.Pipe()
	defer remote.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received := make(chan int, 4)
	released := make(chan Assignment, 2)
	work := make(chan struct{})
	assigned := false
	o := SessionOptions{PipelineLimit: 2, RequestTimeout: time.Second, PieceCount: 1,
		Assign: func(b []byte) (Assignment, error) {
			if assigned || !HasPiece(b, 0) {
				return Assignment{}, ErrNoWork
			}
			assigned = true
			return Assignment{ID: 1, PieceIndex: 0, Blocks: []Block{{0, 1}, {1, 1}, {2, 1}}}, nil
		},
		Requested: func(Assignment, Block) error { return nil },
		Receive:   func(_ Assignment, b types.DataBlock) (bool, error) { received <- b.Begin; return false, nil },
		Release:   func(a Assignment) { released <- a }, Work: func() <-chan struct{} { return work },
	}
	p := &PeerConnection{conn: client, peerChoking: true, Bitfield: make([]byte, 1)}
	result := make(chan error, 1)
	go func() { result <- p.pipeline(ctx, o) }()
	writeWire(t, remote, MessageBitfield, []byte{0x80})
	if readWire(t, remote).ID != MessageInterested {
		t.Fatal("missing interested")
	}
	writeWire(t, remote, MessageUnchoke, nil)
	for i := 0; i < 2; i++ {
		m := readWire(t, remote)
		if m.ID != MessageRequest || int(binary.BigEndian.Uint32(m.Payload[4:8])) != i {
			t.Fatalf("unexpected request: %+v", m)
		}
	}
	remote.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	var probe [1]byte
	if _, err := remote.Read(probe[:]); err == nil {
		t.Fatal("pipeline exceeded limit")
	}
	writeWire(t, remote, MessagePiece, piecePayload(1))
	m := readWire(t, remote)
	if binary.BigEndian.Uint32(m.Payload[4:8]) != 2 {
		t.Fatal("slot was not refilled")
	}
	if <-received != 1 {
		t.Fatal("out of order response not received")
	}
	writeWire(t, remote, MessageChoke, nil)
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("choke did not release")
	}
	writeWire(t, remote, MessagePiece, piecePayload(0))
	writeWire(t, remote, MessageUnchoke, nil)
	cancel()
	select {
	case <-result:
	case <-time.After(time.Second):
		t.Fatal("session did not stop")
	}
	if len(received) != 0 {
		t.Fatal("accepted stale response")
	}
}

func TestPipelineTimeoutReleasesAssignment(t *testing.T) {
	client, remote := net.Pipe()
	defer remote.Close()
	released := make(chan struct{}, 1)
	work := make(chan struct{})
	p := &PeerConnection{conn: client, Bitfield: []byte{0x80}}
	result := make(chan error, 1)
	go func() {
		result <- p.pipeline(context.Background(), SessionOptions{
			PipelineLimit: 1, RequestTimeout: 30 * time.Millisecond,
			Assign:    func([]byte) (Assignment, error) { return Assignment{ID: 1, Blocks: []Block{{0, 1}}}, nil },
			Requested: func(Assignment, Block) error { return nil }, Receive: func(Assignment, types.DataBlock) (bool, error) { return false, nil },
			Release: func(Assignment) { released <- struct{}{} }, Work: func() <-chan struct{} { return work },
		})
	}()
	if readWire(t, remote).ID != MessageRequest {
		t.Fatal("expected request")
	}
	select {
	case err := <-result:
		if err == nil || errors.Is(err, io.EOF) {
			t.Fatalf("expected timeout, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request never expired")
	}
	if len(released) != 1 {
		t.Fatal("timed out assignment not released")
	}
}
