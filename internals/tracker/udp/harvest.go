package udp

import (
	"errors"

	"github.com/seeniolabode/gotorrent/internals/tracker"
)

func (c *UDPTrackerClient) Harvest() (tracker.TrackerHarvest, error) {
	harvest := tracker.TrackerHarvest{
		PeerID:   c.PeerID,
		Infohash: c.Torrent.Metadata.InfoHash,
		Peers:    c.result.Peers,
	}

	if len(harvest.Peers) == 0 {
		return harvest, errors.New("Client has no peers")
	}

	return harvest, nil
}
