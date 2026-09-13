package peers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
)

type messageConnection struct {
	net.Conn
	input  bytes.Buffer
	output bytes.Buffer
}

func (c *messageConnection) Read(p []byte) (int, error)  { return c.input.Read(p) }
func (c *messageConnection) Write(p []byte) (int, error) { return c.output.Write(p) }

func (c *messageConnection) queue(id byte, payload ...byte) {
	binary.Write(&c.input, binary.BigEndian, uint32(1+len(payload)))
	c.input.WriteByte(id)
	c.input.Write(payload)
}

func TestListenInterest(t *testing.T) {
	for _, interested := range []bool{false, true} {
		for _, unchokeFirst := range []bool{false, true} {
			t.Run(strconv.FormatBool(interested)+"/unchoke_first="+strconv.FormatBool(unchokeFirst), func(t *testing.T) {
				conn := &messageConnection{}
				if unchokeFirst {
					conn.queue(MessageUnchoke)
				}
				conn.queue(MessageBitfield, 0x80)
				// Repeated availability must not replace the active assignment.
				conn.queue(MessageHave, 0, 0, 0, 0)
				if !unchokeFirst {
					conn.queue(MessageUnchoke)
				}

				assignments := 0
				p := PeerConnection{conn: conn, peerChoking: true}
				p.assignNextPiece = func(requester *PeerConnection) (int, int, error) {
					if requester != &p || !HasPiece(requester.Bitfield, 0) {
						t.Fatal("selector received incorrect peer state")
					}
					assignments++
					return 0, 1024, nil
				}
				calls := 0
				err := p.Listen(ListenOptions{IsInterested: func(requester *PeerConnection) bool {
					calls++
					if requester != &p || !HasPiece(requester.Bitfield, 0) {
						t.Fatal("interest callback received incorrect peer state")
					}
					return interested
				}})
				if !errors.Is(err, io.EOF) {
					t.Fatalf("expected end of messages, got %v", err)
				}
				if calls != 2 {
					t.Fatalf("interest callback called %d times", calls)
				}
				if !interested {
					if assignments != 0 || conn.output.Len() != 0 {
						t.Fatal("unwanted peer received work or messages")
					}
					return
				}
				want := []byte{0, 0, 0, 1, MessageInterested, 0, 0, 0, 13, MessageRequest,
					0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0}
				if assignments != 1 || !bytes.Equal(conn.output.Bytes(), want) {
					t.Fatalf("assignments=%d messages=%v", assignments, conn.output.Bytes())
				}
			})
		}
	}
}

func TestInterestAssignmentError(t *testing.T) {
	conn := &messageConnection{}
	conn.queue(MessageBitfield, 0x80)
	selectionErr := errors.New("selection failed")
	p := PeerConnection{conn: conn, peerChoking: true,
		assignNextPiece: func(*PeerConnection) (int, int, error) { return 0, 0, selectionErr },
	}
	err := p.Listen(ListenOptions{IsInterested: func(*PeerConnection) bool { return true }})
	if !errors.Is(err, selectionErr) {
		t.Fatalf("expected assignment error, got %v", err)
	}
	if p.currentPiece != nil {
		t.Fatal("failed selection assigned a piece")
	}
}
