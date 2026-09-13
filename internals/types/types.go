package types

import (
	"net"
)

type DataBlock struct {
	PieceIndex int
	Begin      int
	Data       []byte
}

const BlockSize = 16 * 1024

type Peer struct {
	IP   net.IP
	Port uint16
}

type InfoHash [20]byte

type PeerID [20]byte
