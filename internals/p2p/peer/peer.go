package peer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/seeniolabode/gotorrent/internals/p2p/types"
)

type PeerConnection struct {
	types.Peer

	conn      net.Conn
	handshake *HandshakeResult
}

type HandshakeOptions struct {
	PeerID   types.PeerID
	InfoHash types.InfoHash
}

func (p *PeerConnection) Connect() error {
	if p.conn != nil {
		return nil
	}

	address := net.JoinHostPort(
		p.IP.String(),
		strconv.Itoa(int(p.Port)),
	)

	conn, err := net.DialTimeout(
		"tcp",
		address,
		10*time.Second,
	)
	if err != nil {
		return err
	}

	p.conn = conn
	p.handshake = nil

	return nil
}

func (p *PeerConnection) Handshake(o HandshakeOptions) error {
	if p.conn == nil {
		return errors.New("peer is not connected")
	}

	if p.handshake != nil {
		return nil
	}

	if o.InfoHash == (types.InfoHash{}) {
		return errors.New("no info hash provided")
	}

	if o.PeerID == (types.PeerID{}) {
		return errors.New("no peer ID provided")
	}

	request := make([]byte, 68)

	// 0: protocol string length
	request[0] = byte(len(protocol))

	// 1-19: protocol string
	copy(request[1:20], protocol)

	// 20-27: reserved extension bytes
	// Leave zero for now.

	// 28-47: torrent info hash
	copy(request[28:48], o.InfoHash[:])

	// 48-67: our peer ID
	copy(request[48:68], o.PeerID[:])

	if _, err := p.conn.Write(request); err != nil {
		return fmt.Errorf("send handshake: %w", err)
	}

	if err := p.conn.SetReadDeadline(
		time.Now().Add(networkTimeout),
	); err != nil {
		return err
	}
	defer p.conn.SetReadDeadline(time.Time{})

	response := make([]byte, 68)

	if _, err := io.ReadFull(p.conn, response); err != nil {
		return fmt.Errorf("read handshake: %w", err)
	}

	if response[0] != byte(len(protocol)) {
		return fmt.Errorf(
			"unexpected protocol length: %d",
			response[0],
		)
	}

	if string(response[1:20]) != protocol {
		return errors.New(
			"peer is not speaking BitTorrent protocol",
		)
	}

	if !bytes.Equal(response[28:48], o.InfoHash[:]) {
		return errors.New(
			"peer info hash does not match torrent",
		)
	}

	var result HandshakeResult

	copy(result.Reserved[:], response[20:28])
	copy(result.PeerID[:], response[48:68])

	p.handshake = &result

	return nil
}

func (p *PeerConnection) IsConnected() bool {
	return p.conn != nil
}

func (p *PeerConnection) IsHandshaken() bool {
	return p.handshake != nil
}

func (p *PeerConnection) Close() error {
	if p.conn == nil {
		return nil
	}

	err := p.conn.Close()

	p.conn = nil
	p.handshake = nil

	return err
}
