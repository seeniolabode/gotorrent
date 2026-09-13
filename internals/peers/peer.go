package peers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/seeniolabode/gotorrent/internals/types"
)

type PeerConnection struct {
	types.Peer

	conn      net.Conn
	handshake *HandshakeResult
	Bitfield  []byte

	amInterested bool

	peerChoking    bool
	peerInterested bool

	assignNextPiece AssignNextPieceHandler

	currentPiece *int
	nextBegin    int
	pieceLength  int
}

type HandshakeOptions struct {
	PeerID   types.PeerID
	InfoHash types.InfoHash
}

type OnBlockHandler = func(types.DataBlock) (isPieceComplete bool, err error)

// IsInterestedHandler reports whether the peer has pieces the consumer wants.
type IsInterestedHandler = func(*PeerConnection) bool

// AssignNextPieceHandler selects work using the requesting peer’s current state.
type AssignNextPieceHandler = func(*PeerConnection) (piece, pieceLength int, err error)

type ListenOptions struct {
	OnBlock      OnBlockHandler
	IsInterested IsInterestedHandler
}

type UseOptions struct {
	HandshakeOptions
	ListenOptions
}

// Messaging
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
	p.peerChoking = true

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

func (p *PeerConnection) ReadMessage() (*Message, error) {
	lengthBuf := make([]byte, 4)

	if _, err := io.ReadFull(p.conn, lengthBuf); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBuf)

	if length == 0 {
		// keep-alive
		return nil, nil
	}

	messageBuf := make([]byte, length)

	if _, err := io.ReadFull(p.conn, messageBuf); err != nil {
		return nil, err
	}

	return &Message{
		ID:      messageBuf[0],
		Payload: messageBuf[1:],
	}, nil

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

func (p *PeerConnection) Listen(o ListenOptions) error {
	log.Printf("Listening to peer: peer=%s", net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))))

	for {
		msg, err := p.ReadMessage()
		if err != nil {
			return err
		}

		if msg == nil {
			// keep-alive
			continue
		}

		switch msg.ID {
		case MessageBitfield:
			p.Bitfield = msg.Payload
			if err := p.updateInterest(o.IsInterested); err != nil {
				return err
			}

		case MessageHave:
			if err := p.handleHave(msg.Payload); err != nil {
				return err
			}
			if err := p.updateInterest(o.IsInterested); err != nil {
				return err
			}

		case MessageChoke:
			p.peerChoking = true
			log.Printf("Peer choked us: peer=%s", net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))))

		case MessageUnchoke:
			p.peerChoking = false
			log.Printf("Peer unchoked us: peer=%s", net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))))

			if p.currentPiece != nil {
				if err := p.RequestPiece(); err != nil {
					return err
				}
			}

		case MessageInterested:
			p.peerInterested = true

		case MessageNotInterested:
			p.peerInterested = false

		case MessagePiece:
			block, err := parseBlock(msg.Payload)
			if err != nil {
				return err
			}

			pieceComplete, err := o.OnBlock(block)
			if err != nil {
				return err
			}

			if !pieceComplete {
				p.nextBegin = block.Begin + len(block.Data)

				if err := p.RequestPiece(); err != nil {
					return err
				}

				continue
			}

			// Current piece is finished.
			if err := p.AssignNextPiece(); err != nil {
				// No more work available for this peer.
				return nil
			}

			// Start the newly assigned piece.
			if err := p.RequestPiece(); err != nil {
				return err
			}
		}
	}
}

// updateInterest reacts to availability changes without replacing an active piece.
func (p *PeerConnection) updateInterest(isInterested IsInterestedHandler) error {
	if isInterested == nil {
		log.Printf("Interest check skipped: peer=%s reason=no handler configured", net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))))
		return nil
	}
	interested := isInterested(p)
	log.Printf("Peer interest evaluated: peer=%s interested=%t bitfield_bytes=%d", net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))), interested, len(p.Bitfield))
	if !interested {
		return nil
	}

	if err := p.SendInterested(); err != nil {
		return fmt.Errorf("send interested: %w", err)
	}

	if p.currentPiece != nil {
		return nil
	}

	if err := p.AssignNextPiece(); err != nil {
		return fmt.Errorf("assign piece: %w", err)
	}

	if !p.peerChoking {
		return p.RequestPiece()
	}

	log.Printf("Waiting for unchoke: peer=%s piece=%d", net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))), *p.currentPiece)
	return nil
}

func (p *PeerConnection) SendInterested() error {
	if p.amInterested {
		return nil
	}

	buf := make([]byte, 5)

	binary.BigEndian.PutUint32(buf[0:4], 1)
	buf[4] = MessageInterested

	_, err := p.conn.Write(buf)

	if err == nil {
		p.amInterested = true
		log.Printf("Interested sent: peer=%s", net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))))
	}

	return err
}

// AssignNextPiece selects a piece and resets its block offset without sending a request.
// If selection fails, the current assignment is left unchanged.
func (p *PeerConnection) AssignNextPiece() error {
	if p.assignNextPiece == nil {
		return errors.New("no piece assignment handler configured")
	}

	pieceIndex, pieceLength, err := p.assignNextPiece(p)
	if err != nil {
		return err
	}

	p.currentPiece = &pieceIndex
	p.pieceLength = pieceLength
	p.nextBegin = 0
	log.Printf("Piece assigned: peer=%s piece=%d length=%d", net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))), pieceIndex, pieceLength)

	return nil
}

func (p *PeerConnection) RequestPiece() (err error) {
	address := net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
	pieceIndex := -1
	if p.currentPiece != nil {
		pieceIndex = *p.currentPiece
	}
	log.Printf("Piece request attempted: peer=%s piece=%d begin=%d choking=%t", address, pieceIndex, p.nextBegin, p.peerChoking)
	defer func() {
		if err != nil {
			log.Printf("Piece request failed: peer=%s piece=%d begin=%d error=%v", address, pieceIndex, p.nextBegin, err)
		}
	}()

	if p.currentPiece == nil {
		return errors.New("no piece assigned")
	}

	if p.peerChoking {
		return errors.New("peer is choking us")
	}

	if p.nextBegin >= p.pieceLength {
		return errors.New("piece fully requested")
	}

	length := types.BlockSize

	remaining := p.pieceLength - p.nextBegin
	if remaining < types.BlockSize {
		length = remaining
	}

	buf := make([]byte, 17)

	binary.BigEndian.PutUint32(buf[0:4], 13)
	buf[4] = MessageRequest

	binary.BigEndian.PutUint32(
		buf[5:9],
		uint32(*p.currentPiece),
	)

	binary.BigEndian.PutUint32(
		buf[9:13],
		uint32(p.nextBegin),
	)

	binary.BigEndian.PutUint32(
		buf[13:17],
		uint32(length),
	)

	if _, err := p.conn.Write(buf); err != nil {
		return fmt.Errorf("request block: %w", err)
	}

	log.Printf("Block requested: peer=%s piece=%d begin=%d length=%d",
		net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port))),
		*p.currentPiece, p.nextBegin, length)

	return nil
}

func (p *PeerConnection) Use(o UseOptions) error {
	if err := p.Connect(); err != nil {
		return fmt.Errorf("connect peer: %w", err)
	}

	defer p.Close()

	if err := p.Handshake(o.HandshakeOptions); err != nil {
		return fmt.Errorf("handshake peer: %w", err)
	}

	if err := p.Listen(o.ListenOptions); err != nil {
		return fmt.Errorf("listen to peer: %w", err)
	}

	return nil
}

// Message Handlers
func (p *PeerConnection) handleHave(payload []byte) error {
	if len(payload) != 4 {
		return errors.New("invalid have message")
	}

	pieceIndex := binary.BigEndian.Uint32(payload)

	return p.markPieceAvailable(int(pieceIndex))
}

func (p *PeerConnection) markPieceAvailable(pieceIndex int) error {
	if pieceIndex < 0 {
		return errors.New("invalid piece index")
	}

	byteIndex := pieceIndex / 8
	bitIndex := pieceIndex % 8

	if byteIndex >= len(p.Bitfield) {
		return errors.New("piece index outside bitfield")
	}

	mask := byte(1 << (7 - bitIndex))

	p.Bitfield[byteIndex] |= mask

	return nil
}

// Utility Methods
func (p *PeerConnection) IsConnected() bool {
	return p.conn != nil
}

func (p *PeerConnection) IsHandshaken() bool {
	return p.handshake != nil
}

// Utilities
func HasPiece(bitfield []byte, pieceIndex int) bool {
	byteIndex := pieceIndex / 8
	bitIndex := pieceIndex % 8

	if byteIndex >= len(bitfield) {
		return false
	}

	mask := byte(1 << (7 - bitIndex))

	return bitfield[byteIndex]&mask != 0
}

func parseBlock(payload []byte) (types.DataBlock, error) {
	if len(payload) < 8 {
		return types.DataBlock{}, errors.New("invalid piece message")
	}

	pieceIndex := binary.BigEndian.Uint32(payload[0:4])
	begin := binary.BigEndian.Uint32(payload[4:8])

	return types.DataBlock{
		PieceIndex: int(pieceIndex),
		Begin:      int(begin),
		Data:       payload[8:],
	}, nil
}
