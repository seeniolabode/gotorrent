package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/seeniolabode/gotorrent/internals/peers"
	"github.com/seeniolabode/gotorrent/internals/types"
)

// Start blocks until completion or until all discovered peers are exhausted.
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
	harvest, err := s.tracker.Harvest()
	if err != nil {
		return fmt.Errorf("harvest peers: %w", err)
	}
	return s.runPeers(harvest.Peers, peers.HandshakeOptions{PeerID: harvest.PeerID, InfoHash: s.torrent.Metadata.InfoHash})
}

func (s *Scheduler) runPeers(discovered []types.Peer, identity peers.HandshakeOptions) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// A repeated tracker endpoint must not open duplicate sessions.
	unique := make([]types.Peer, 0, len(discovered))
	seen := make(map[string]bool)
	for _, peer := range discovered {
		key := fmt.Sprintf("%s:%d", peer.IP, peer.Port)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, peer)
		}
	}
	jobs := make(chan types.Peer, len(unique))
	for _, peer := range unique {
		jobs <- peer
	}
	close(jobs)
	var wg sync.WaitGroup
	var errorMu sync.Mutex
	var lastErr error
	for range min(s.maxPeers, len(unique)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for peer := range jobs {
				if ctx.Err() != nil {
					return
				}
				p := &peers.PeerConnection{Peer: peer}
				err := p.RunSession(ctx, peers.SessionOptions{
					HandshakeOptions: identity, PipelineLimit: s.pipelineLimit,
					RequestTimeout: s.requestTimeout, PieceCount: len(s.torrent.Torrent.Info.Pieces) / 20,
					Assign: s.assignPiece, Requested: s.requested, Receive: s.receive,
					Release: s.release, Work: s.workAvailable,
				})
				if len(s.store.MissingPieces()) == 0 {
					cancel()
					return
				}
				if err != nil {
					errorMu.Lock()
					lastErr = err
					errorMu.Unlock()
				}
			}
		}()
	}
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	for {
		work := s.workAvailable()
		if len(s.store.MissingPieces()) == 0 {
			cancel()
			<-finished
			return nil
		}
		select {
		case <-work:
		case <-finished:
			if len(s.store.MissingPieces()) == 0 {
				return nil
			}
			if lastErr != nil {
				return fmt.Errorf("download incomplete; discovered peers exhausted: %w", lastErr)
			}
			return errors.New("download incomplete; no usable peers")
		}
	}
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
