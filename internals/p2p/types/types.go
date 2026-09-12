package types

import (
	"net"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

type InfoHash [20]byte

type PeerID [20]byte
