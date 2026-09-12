package udp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

func (c *UDPTrackerClient) connect() error {
	if c.conn == nil {
		return errors.New("UDP client is not initialized")
	}

	connectionID, err := c.connectTracker()
	if err != nil {
		return err
	}

	c.connectionID = connectionID

	return nil
}

func (c *UDPTrackerClient) connectTracker() (uint64, error) {
	transactionID, err := generateTransactionID()
	if err != nil {
		return 0, err
	}

	request := make([]byte, 16)

	binary.BigEndian.PutUint64(request[0:8], udpProtocolID)
	binary.BigEndian.PutUint32(request[8:12], actionConnect)
	binary.BigEndian.PutUint32(request[12:16], transactionID)

	if _, err := c.conn.Write(request); err != nil {
		return 0, fmt.Errorf("send tracker connect request: %w", err)
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(networkTimeout)); err != nil {
		return 0, err
	}
	defer c.conn.SetReadDeadline(time.Time{})

	response := make([]byte, 2048)

	n, err := c.conn.Read(response)
	if err != nil {
		return 0, fmt.Errorf("read tracker connect response: %w", err)
	}

	response = response[:n]

	if len(response) < 8 {
		return 0, fmt.Errorf(
			"tracker response too short: %d bytes",
			len(response),
		)
	}

	action := binary.BigEndian.Uint32(response[0:4])
	responseTransactionID := binary.BigEndian.Uint32(response[4:8])

	if responseTransactionID != transactionID {
		return 0, errors.New("tracker transaction ID does not match request")
	}

	if action == actionError {
		return 0, fmt.Errorf(
			"tracker error: %s",
			string(response[8:]),
		)
	}

	if action != actionConnect {
		return 0, fmt.Errorf(
			"unexpected tracker action: %d",
			action,
		)
	}

	if len(response) < 16 {
		return 0, fmt.Errorf(
			"connect response too short: %d bytes",
			len(response),
		)
	}

	return binary.BigEndian.Uint64(response[8:16]), nil
}
