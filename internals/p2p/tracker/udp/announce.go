package udp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/seeniolabode/gotorrent/internals/p2p/tracker"
	"github.com/seeniolabode/gotorrent/internals/p2p/types"
)

func (c *UDPTrackerClient) announce() (tracker.TrackingResult, error) {
	if c.conn == nil {
		return tracker.TrackingResult{}, errors.New("UDP connection is not open")
	}

	transactionID, err := generateTransactionID()
	if err != nil {
		return tracker.TrackingResult{}, err
	}

	key, err := generateTransactionID()
	if err != nil {
		return tracker.TrackingResult{}, err
	}

	request := make([]byte, 98)

	// 0-7: connection ID
	binary.BigEndian.PutUint64(
		request[0:8],
		c.connectionID,
	)

	// 8-11: action = announce
	binary.BigEndian.PutUint32(
		request[8:12],
		actionAnnounce,
	)

	// 12-15: transaction ID
	binary.BigEndian.PutUint32(
		request[12:16],
		transactionID,
	)

	// 16-35: info hash
	copy(
		request[16:36],
		c.Torrent.Metadata.InfoHash[:],
	)

	// 36-55: peer ID
	copy(
		request[36:56],
		c.PeerID[:],
	)

	// 56-63: downloaded
	binary.BigEndian.PutUint64(
		request[56:64],
		0,
	)

	// 64-71: bytes remaining
	binary.BigEndian.PutUint64(
		request[64:72],
		uint64(c.Torrent.Metadata.TotalLength),
	)

	// 72-79: uploaded
	binary.BigEndian.PutUint64(
		request[72:80],
		0,
	)

	// 80-83: event = started
	binary.BigEndian.PutUint32(
		request[80:84],
		eventStarted,
	)

	// 84-87: IP address = 0 means use source IP
	binary.BigEndian.PutUint32(
		request[84:88],
		0,
	)

	// 88-91: random key
	binary.BigEndian.PutUint32(
		request[88:92],
		key,
	)

	// 92-95: num_want = -1
	binary.BigEndian.PutUint32(
		request[92:96],
		^uint32(0),
	)

	// 96-97: port we're advertising
	// NOTE: not yet listening on any port
	binary.BigEndian.PutUint16(
		request[96:98],
		0,
	)

	if _, err := c.conn.Write(request); err != nil {
		return tracker.TrackingResult{}, fmt.Errorf(
			"send announce request: %w",
			err,
		)
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(networkTimeout)); err != nil {
		return tracker.TrackingResult{}, err
	}

	defer c.conn.SetReadDeadline(time.Time{})

	// Announce responses are variable length because peers follow
	// the 20-byte response header.
	response := make([]byte, 65535)

	n, err := c.conn.Read(response)
	if err != nil {
		return tracker.TrackingResult{}, fmt.Errorf(
			"read announce response: %w",
			err,
		)
	}

	response = response[:n]

	// Need at least action + transaction ID before we can
	// determine what kind of response this is.
	if len(response) < 8 {
		return tracker.TrackingResult{}, fmt.Errorf(
			"tracker response too short: %d bytes",
			len(response),
		)
	}

	action := binary.BigEndian.Uint32(response[0:4])

	responseTransactionID :=
		binary.BigEndian.Uint32(response[4:8])

	if responseTransactionID != transactionID {
		return tracker.TrackingResult{},
			errors.New("tracker transaction ID does not match")
	}

	if action == actionError {
		return tracker.TrackingResult{}, fmt.Errorf(
			"tracker error: %s",
			string(response[8:]),
		)
	}

	if action != actionAnnounce {
		return tracker.TrackingResult{}, fmt.Errorf(
			"unexpected tracker action: %d",
			action,
		)
	}

	// Successful announce response has a 20-byte header.
	if len(response) < 20 {
		return tracker.TrackingResult{}, fmt.Errorf(
			"announce response too short: %d bytes",
			len(response),
		)
	}

	interval := binary.BigEndian.Uint32(response[8:12])
	leechers := binary.BigEndian.Uint32(response[12:16])
	seeders := binary.BigEndian.Uint32(response[16:20])

	peers, err := parsePeers(response[20:])
	if err != nil {
		return tracker.TrackingResult{}, err
	}

	return tracker.TrackingResult{
		Interval: interval,
		Leechers: leechers,
		Seeders:  seeders,
		Peers:    peers,
	}, nil
}

func parsePeers(data []byte) ([]types.Peer, error) {
	if len(data)%6 != 0 {
		return nil, fmt.Errorf(
			"invalid compact peer data length: %d",
			len(data),
		)
	}

	peers := make([]types.Peer, 0, len(data)/6)

	for i := 0; i < len(data); i += 6 {
		ip := net.IPv4(
			data[i],
			data[i+1],
			data[i+2],
			data[i+3],
		)
		port := binary.BigEndian.Uint16(
			data[i+4 : i+6],
		)

		peers = append(peers, types.Peer{
			IP:   ip,
			Port: port,
		})
	}

	return peers, nil
}
