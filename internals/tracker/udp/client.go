package udp

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/seeniolabode/gotorrent/internals/torrentx"
	"github.com/seeniolabode/gotorrent/internals/tracker"
	"github.com/seeniolabode/gotorrent/internals/types"
)

const (
	udpProtocolID uint64 = 0x41727101980

	actionConnect  uint32 = 0
	actionAnnounce uint32 = 1
	actionError    uint32 = 3

	eventNone      uint32 = 0
	eventCompleted uint32 = 1
	eventStarted   uint32 = 2
	eventStopped   uint32 = 3

	networkTimeout = time.Second * 10
)

type UDPTrackerClient struct {
	PeerID       types.PeerID
	connectionID uint64
	Torrent      torrentx.ParsedTorrent
	conn         *net.UDPConn
	result       tracker.TrackingResult
}

func NewUDPTrackerClient(t torrentx.ParsedTorrent) (*UDPTrackerClient, error) {
	peerID, err := tracker.GeneratePeerID()
	if err != nil {
		return nil, err
	}

	return &UDPTrackerClient{
		PeerID:  peerID,
		Torrent: t,
	}, nil
}

func (c *UDPTrackerClient) init() error {
	if c.conn != nil {
		return errors.New("UDP client already initialized")
	}

	u, err := url.Parse(c.Torrent.Torrent.Announce)
	if err != nil {
		return err
	}

	if u.Scheme != "udp" {
		return fmt.Errorf("expected UDP tracker, got %q", u.Scheme)
	}

	addr, err := net.ResolveUDPAddr("udp", u.Host)
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}

	c.conn = conn

	return nil
}

func (c *UDPTrackerClient) close() error {
	if c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil

	return err
}

func generateTransactionID() (uint32, error) {
	var data [4]byte

	if _, err := rand.Read(data[:]); err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(data[:]), nil
}
