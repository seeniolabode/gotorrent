package scheduler

import (
	"errors"

	"github.com/seeniolabode/gotorrent/internals/peers"
	"github.com/seeniolabode/gotorrent/internals/types"
)

func (s *Scheduler) assignPiece(bitfield []byte) (peers.Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, index := range s.store.MissingPieces() {
		if !peers.HasPiece(bitfield, index) || s.assignments[index] != 0 {
			continue
		}
		piece, err := s.store.Piece(index)
		if err != nil {
			return peers.Assignment{}, err
		}
		s.nextAssignment++
		a := peers.Assignment{ID: s.nextAssignment, PieceIndex: index}
		for _, block := range piece.Blocks {
			if !block.Received {
				a.Blocks = append(a.Blocks, peers.Block{Begin: block.Begin, Length: block.Length})
			}
		}
		s.assignments[index] = a.ID
		return a, nil
	}
	return peers.Assignment{}, peers.ErrNoWork
}

func (s *Scheduler) requested(a peers.Assignment, b peers.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ID == 0 || s.assignments[a.PieceIndex] != a.ID {
		return errors.New("assignment has expired")
	}
	return s.store.MarkRequested(a.PieceIndex, b.Begin)
}

func (s *Scheduler) receive(a peers.Assignment, b types.DataBlock) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ID == 0 || s.assignments[a.PieceIndex] != a.ID {
		return false, nil
	}
	if b.PieceIndex != a.PieceIndex {
		return false, errors.New("block does not match assignment")
	}
	if err := s.store.Store(b); err != nil {
		return false, err
	}
	return s.store.HasPiece(b.PieceIndex), nil
}

func (s *Scheduler) release(a peers.Assignment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ID == 0 || s.assignments[a.PieceIndex] != a.ID {
		return
	}
	s.store.ReleaseRequests(a.PieceIndex)
	delete(s.assignments, a.PieceIndex)
	close(s.work)
	s.work = make(chan struct{})
}

func (s *Scheduler) workAvailable() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.work
}
