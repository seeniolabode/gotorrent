package scheduler

import (
	"errors"
	"fmt"

	"github.com/seeniolabode/gotorrent/internals/peers"
)

// Start prepares and verifies local files, announces to the tracker, and runs one
// peer session synchronously. A nil result does not guarantee a complete download.
// Calls may be retried after return; overlapping calls are rejected.
func (s *Scheduler) Start() error {
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("scheduler is already running")
	}
	defer s.running.Store(false)
	if s.store == nil || s.tracker == nil {
		return errors.New("scheduler must be created with NewScheduler")
	}
	if err := s.prepareStorage(); err != nil {
		return err
	}
	if len(s.store.MissingPieces()) == 0 {
		return nil
	}
	if _, err := s.tracker.Run(); err != nil {
		return fmt.Errorf("run tracker: %w", err)
	}
	manager, err := peers.NewPeerManager(s.tracker, s.assignPiece)
	if err != nil {
		return fmt.Errorf("create peer manager: %w", err)
	}
	connection, err := manager.ConnectSinglePeer()
	if err != nil {
		return fmt.Errorf("connect peer: %w", err)
	}
	return connection.Use(peers.UseOptions{
		HandshakeOptions: peers.HandshakeOptions{
			PeerID:   manager.PeerID,
			InfoHash: s.torrent.Metadata.InfoHash,
		},
		ListenOptions: peers.ListenOptions{
			OnBlock:      s.writeToStorage,
			IsInterested: s.isInterested,
		},
	})
}

func (s *Scheduler) prepareStorage() error {
	if err := s.store.PrepareFileSystem(); err != nil {
		return fmt.Errorf("prepare storage: %w", err)
	}
	if err := s.store.Load(); err != nil {
		return fmt.Errorf("load piece state: %w", err)
	}
	return nil
}

func (s *Scheduler) isInterested(p *peers.PeerConnection) bool {
	for _, piece := range s.store.MissingPieces() {
		if peers.HasPiece(p.Bitfield, piece) {
			return true
		}
	}
	return false
}
