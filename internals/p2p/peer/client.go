package peer

import (
	"errors"

	"github.com/seeniolabode/gotorrent/internals/p2p/tracker"
	"github.com/seeniolabode/gotorrent/internals/p2p/types"
)

type P2PClient struct {
	PeerID   types.PeerID
	InfoHash types.InfoHash
	Peers    []PeerConnection
}

func (c *P2PClient) ConnectSinglePeer() (PeerConnection, error) {
	for _, p := range c.Peers {
		if p.IsConnected() && p.IsHandshaken() {
			return p, nil
		}

		if err := p.Connect(); err != nil {
			continue
		}

		err := p.Handshake(HandshakeOptions{
			PeerID:   c.PeerID,
			InfoHash: c.InfoHash,
		})
		if err != nil {
			p.Close()
			continue
		}

		return p, nil
	}

	return PeerConnection{}, errors.New("could not connect and handshake with any peer")
}

func NewP2PClientFromTracker(t tracker.Tracker, assignNextPiece AssignNextPieceHandler) (*P2PClient, error) {
	if assignNextPiece == nil {
		return nil, errors.New("no piece assignment handler configured")
	}

	tracker, err := t.Harvest()

	if err != nil {
		return nil, err
	}

	var connected_peers []PeerConnection

	for i := range tracker.Peers {
		connected_peers = append(connected_peers, PeerConnection{
			handshake:       nil,
			conn:            nil,
			Peer:            tracker.Peers[i],
			assignNextPiece: assignNextPiece,
		})
	}

	client := P2PClient{
		PeerID:   tracker.PeerID,
		InfoHash: tracker.Infohash,
		Peers:    connected_peers,
	}

	return &client, nil

}
