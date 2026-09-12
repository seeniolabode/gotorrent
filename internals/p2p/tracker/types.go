package tracker

import (
	"github.com/seeniolabode/gotorrent/internals/p2p/types"
)

type TrackerHarvest struct {
	PeerID   types.PeerID
	Infohash types.InfoHash
	Peers    []types.Peer
}

type Tracker interface {
	Harvest() (TrackerHarvest, error)
}

type TrackingResult struct {
	Interval uint32
	Leechers uint32
	Seeders  uint32
	Peers    []types.Peer
}
