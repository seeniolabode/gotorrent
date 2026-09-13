package peers

import (
	"errors"
	"testing"

	"github.com/seeniolabode/gotorrent/internals/tracker"
	"github.com/seeniolabode/gotorrent/internals/types"
)

type assignmentTracker struct{}

func (assignmentTracker) Harvest() (tracker.TrackerHarvest, error) {
	return tracker.TrackerHarvest{Peers: make([]types.Peer, 2)}, nil
}

func TestClientPeersAssignUsingTheirCurrentBitfield(t *testing.T) {
	noWork := errors.New("no work")
	client, err := NewPeerManager(assignmentTracker{}, func(p *PeerConnection) (int, int, error) {
		for index := 0; index < 8; index++ {
			if HasPiece(p.Bitfield, index) {
				return index, 1024, nil
			}
		}
		return 0, 0, noWork
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := range client.Peers {
		p := &client.Peers[i]
		p.Bitfield = []byte{0x80 >> i}
		p.nextBegin = 512
		if err := p.AssignNextPiece(); err != nil {
			t.Fatal(err)
		}
		if p.currentPiece == nil || *p.currentPiece != i || p.pieceLength != 1024 || p.nextBegin != 0 {
			t.Fatalf("peer %d has incorrect assignment: %+v", i, p)
		}
	}

	// A copied peer, as returned by ConnectSinglePeer, must use its own live state.
	p := client.Peers[0]
	p.Bitfield = []byte{0x20}
	if err := p.AssignNextPiece(); err != nil {
		t.Fatal(err)
	}
	if *p.currentPiece != 2 || *client.Peers[0].currentPiece != 0 {
		t.Fatal("assignment did not use the requesting peer")
	}
	p.Bitfield = nil
	p.nextBegin = 512
	if err := p.AssignNextPiece(); !errors.Is(err, noWork) {
		t.Fatalf("expected selector error, got %v", err)
	}
	if *p.currentPiece != 2 || p.pieceLength != 1024 || p.nextBegin != 512 {
		t.Fatal("failed assignment changed piece state")
	}
}

func TestMissingPieceAssignmentHandler(t *testing.T) {
	if _, err := NewPeerManager(assignmentTracker{}, nil); err == nil {
		t.Fatal("expected missing handler error")
	}
	var p PeerConnection
	if err := p.AssignNextPiece(); err == nil {
		t.Fatal("expected missing handler error for zero-value peer")
	}
}
