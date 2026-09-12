package peer

import (
	"time"

	"github.com/seeniolabode/gotorrent/internals/p2p/types"
)

const protocol = "BitTorrent protocol"

const (
	networkTimeout = time.Second * 10
)

type HandshakeResult struct {
	PeerID   types.PeerID
	Reserved [8]byte
}
