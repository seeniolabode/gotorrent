package peers

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/seeniolabode/gotorrent/internals/types"
)

var ErrNoWork = errors.New("no available piece")

// Block describes an exact wire request, including a possibly shorter final block.
type Block struct{ Begin, Length int }
type Assignment struct {
	ID         uint64
	PieceIndex int
	Blocks     []Block
}

type SessionOptions struct {
	HandshakeOptions
	PipelineLimit  int
	RequestTimeout time.Duration
	PieceCount     int
	Assign         func(bitfield []byte) (Assignment, error)
	Requested      func(Assignment, Block) error
	Receive        func(Assignment, types.DataBlock) (bool, error)
	Release        func(Assignment)
	// Work returns a broadcast channel closed when ownership changes.
	Work func() <-chan struct{}
}

// RunSession owns the connection and keeps all peer-local state on one event loop.
func (p *PeerConnection) RunSession(ctx context.Context, o SessionOptions) error {
	if o.PipelineLimit <= 0 || o.RequestTimeout <= 0 || o.PieceCount < 0 || o.Assign == nil || o.Requested == nil || o.Receive == nil || o.Release == nil || o.Work == nil {
		return errors.New("invalid session options")
	}
	address := net.JoinHostPort(p.IP.String(), fmt.Sprint(p.Port))
	conn, err := (&net.Dialer{Timeout: networkTimeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	p.conn = conn
	p.peerChoking = true
	p.Bitfield = make([]byte, (o.PieceCount+7)/8)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	defer p.Close()
	if err := conn.SetWriteDeadline(time.Now().Add(o.RequestTimeout)); err != nil {
		return err
	}
	if err := p.Handshake(o.HandshakeOptions); err != nil {
		return err
	}
	return p.pipeline(ctx, o)
}

func (p *PeerConnection) pipeline(ctx context.Context, o SessionOptions) error {
	type result struct {
		msg *Message
		err error
	}
	incoming := make(chan result)
	done := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			msg, err := p.ReadMessage()
			select {
			case incoming <- result{msg, err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() { close(done); p.conn.Close(); <-readerDone }()
	var assignment *Assignment
	pending := map[int]time.Time{}
	next := 0
	// Wire replies contain no assignment ID. Never reassign a released piece on
	// this connection, where old and new responses would be indistinguishable.
	retired := map[int]bool{}
	release := func() {
		if assignment != nil {
			retired[assignment.PieceIndex] = true
			o.Release(*assignment)
			assignment = nil
		}
		clear(pending)
		next = 0
	}
	defer release()
	idleSince := time.Now()
	pump := func() error {
		if p.peerChoking {
			return nil
		}
		if assignment == nil {
			available := append([]byte(nil), p.Bitfield...)
			for index := range retired {
				if index/8 < len(available) {
					available[index/8] &^= byte(1 << (7 - index%8))
				}
			}
			a, err := o.Assign(available)
			if errors.Is(err, ErrNoWork) {
				return nil
			}
			if err != nil {
				return err
			}
			assignment = &a
			next = 0
		}
		for len(pending) < o.PipelineLimit && next < len(assignment.Blocks) {
			b := assignment.Blocks[next]
			if err := o.Requested(*assignment, b); err != nil {
				return err
			}
			if err := p.conn.SetWriteDeadline(time.Now().Add(o.RequestTimeout)); err != nil {
				return err
			}
			request := make([]byte, 17)
			binary.BigEndian.PutUint32(request[:4], 13)
			request[4] = MessageRequest
			binary.BigEndian.PutUint32(request[5:9], uint32(assignment.PieceIndex))
			binary.BigEndian.PutUint32(request[9:13], uint32(b.Begin))
			binary.BigEndian.PutUint32(request[13:], uint32(b.Length))
			if _, err := p.conn.Write(request); err != nil {
				return err
			}
			pending[b.Begin] = time.Now()
			next++
		}
		return nil
	}
	interval := min(o.RequestTimeout/4, time.Second)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		work := o.Work()
		if err := pump(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-work:
		case now := <-ticker.C:
			for _, sent := range pending {
				if now.Sub(sent) >= o.RequestTimeout {
					return errors.New("block request timed out")
				}
			}
			if assignment == nil && now.Sub(idleSince) >= o.RequestTimeout {
				return errors.New("peer has no usable work or remains choked")
			}
		case r := <-incoming:
			if r.err != nil {
				return r.err
			}
			if r.msg == nil {
				continue
			}
			msg := r.msg
			switch msg.ID {
			case MessageBitfield:
				if len(msg.Payload) != len(p.Bitfield) {
					return errors.New("invalid bitfield length")
				}
				copy(p.Bitfield, msg.Payload)
				if err := p.conn.SetWriteDeadline(time.Now().Add(o.RequestTimeout)); err != nil {
					return err
				}
				if err := p.SendInterested(); err != nil {
					return err
				}
			case MessageHave:
				if len(msg.Payload) != 4 || int(binary.BigEndian.Uint32(msg.Payload)) >= o.PieceCount {
					return errors.New("invalid have")
				}
				if err := p.handleHave(msg.Payload); err != nil {
					return err
				}
				if err := p.conn.SetWriteDeadline(time.Now().Add(o.RequestTimeout)); err != nil {
					return err
				}
				if err := p.SendInterested(); err != nil {
					return err
				}
			case MessageChoke:
				p.peerChoking = true
				release()
				idleSince = time.Now()
			case MessageUnchoke:
				p.peerChoking = false
			case MessagePiece:
				b, err := parseBlock(msg.Payload)
				if err != nil {
					return err
				}
				if assignment == nil || b.PieceIndex != assignment.PieceIndex {
					continue
				}
				if _, ok := pending[b.Begin]; !ok {
					continue
				}
				complete, err := o.Receive(*assignment, b)
				if err != nil {
					return err
				}
				delete(pending, b.Begin)
				if complete {
					release()
					idleSince = time.Now()
				}
			}
		}
	}
}
