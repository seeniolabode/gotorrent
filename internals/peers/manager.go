package peers

import (
	"errors"

	"github.com/seeniolabode/gotorrent/internals/tracker"
	"github.com/seeniolabode/gotorrent/internals/types"
)

type PeerManager struct {
	PeerID   types.PeerID
	InfoHash types.InfoHash
	Peers    []PeerConnection
}

func (c *PeerManager) ConnectSinglePeer() (PeerConnection, error) {
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

func NewPeerManager(t tracker.Tracker, assignNextPiece AssignNextPieceHandler) (*PeerManager, error) {
	if assignNextPiece == nil {
		return nil, errors.New("missing piece assignment handler")
	}
	trackerHarvest, err := t.Harvest()

	if err != nil {
		return nil, err
	}

	var connected_peers []PeerConnection

	for i := range trackerHarvest.Peers {
		connected_peers = append(connected_peers, PeerConnection{
			handshake:       nil,
			conn:            nil,
			Peer:            trackerHarvest.Peers[i],
			assignNextPiece: assignNextPiece,
		})
	}

	manager := PeerManager{
		PeerID:   trackerHarvest.PeerID,
		InfoHash: trackerHarvest.Infohash,
		Peers:    connected_peers,
	}

	return &manager, nil

}
