package scheduler

import (
	"errors"
	"github.com/seeniolabode/gotorrent/internals/peers"
	"github.com/seeniolabode/gotorrent/internals/types"
)

func (s *Scheduler) writeToStorage(b types.DataBlock) (isPieceComplete bool, err error) {
	if err := s.store.Store(b); err != nil {
		return false, err
	}

	piece := s.store.Pieces[b.PieceIndex]

	return piece.Complete, nil
}

func (s *Scheduler) assignPiece(b []byte) (index, length int, err error) {
	for _, pieceIndex := range s.store.MissingPieces() {
		if !peers.HasPiece(b, pieceIndex) {
			continue
		}

		piece := s.store.Pieces[pieceIndex]
		return piece.Index, piece.Length, nil
	}

	return 0, 0, errors.New("peer has no pieces we need")
}
