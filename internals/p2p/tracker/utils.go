package tracker

import (
	"crypto/rand"
)

func GeneratePeerID() ([20]byte, error) {
	var peerID [20]byte

	copy(peerID[:], []byte("-SL0001-"))

	_, err := rand.Read(peerID[8:])
	if err != nil {
		return [20]byte{}, err
	}

	return peerID, nil
}
