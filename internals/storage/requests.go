package storage

import "fmt"

// Piece returns a snapshot; callers cannot mutate authoritative block state.
func (s *Storage) Piece(index int) (PieceState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.Pieces) {
		return PieceState{}, fmt.Errorf("invalid piece index: %d", index)
	}
	piece := s.Pieces[index]
	piece.Blocks = append([]BlockState(nil), piece.Blocks...)
	return piece, nil
}

func (s *Storage) MarkRequested(index, begin int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.Pieces) {
		return fmt.Errorf("invalid piece index: %d", index)
	}
	for i := range s.Pieces[index].Blocks {
		block := &s.Pieces[index].Blocks[i]
		if block.Begin != begin {
			continue
		}
		if block.Received || block.Requested {
			return fmt.Errorf("block is already received or requested")
		}
		block.Requested = true
		return nil
	}
	return fmt.Errorf("invalid block offset: %d", begin)
}

// ReleaseRequests preserves received data so another peer can resume the piece.
func (s *Storage) ReleaseRequests(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.Pieces) {
		return
	}
	for i := range s.Pieces[index].Blocks {
		s.Pieces[index].Blocks[i].Requested = false
	}
}
