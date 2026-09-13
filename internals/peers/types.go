package peers

import (
	"time"

	"github.com/seeniolabode/gotorrent/internals/types"
)

const protocol = "BitTorrent protocol"

const (
	networkTimeout = time.Second * 10
)

type HandshakeResult struct {
	PeerID   types.PeerID
	Reserved [8]byte
}

type Message struct {
	ID      byte
	Payload []byte
}

const (
	MessageChoke         byte = 0
	MessageUnchoke       byte = 1
	MessageInterested    byte = 2
	MessageNotInterested byte = 3
	MessageHave          byte = 4
	MessageBitfield      byte = 5
	MessageRequest       byte = 6
	MessagePiece         byte = 7
	MessageCancel        byte = 8
)
