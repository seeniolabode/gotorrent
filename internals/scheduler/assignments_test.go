package scheduler

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"github.com/seeniolabode/gotorrent/internals/storage"
	"github.com/seeniolabode/gotorrent/internals/torrentx"
	"path/filepath"
	"sync"
	"testing"

	"github.com/seeniolabode/gotorrent/internals/peers"
	"github.com/seeniolabode/gotorrent/internals/types"
)

func TestExclusiveAssignmentsAndStaleRelease(t *testing.T) {
	s, _ := newTestScheduler(t)
	if err := s.prepareStorage(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	winners := make(chan peers.Assignment, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := s.assignPiece([]byte{0x80})
			if err == nil {
				winners <- a
			} else if !errors.Is(err, peers.ErrNoWork) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	close(winners)
	if len(winners) != 1 {
		t.Fatalf("got %d owners", len(winners))
	}
	old := <-winners
	if err := s.requested(old, old.Blocks[0]); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := s.store.Piece(0)
	if !snapshot.Blocks[0].Requested || snapshot.Blocks[0].Received {
		t.Fatal("request state incorrect")
	}
	notification := s.workAvailable()
	s.release(old)
	select {
	case <-notification:
	default:
		t.Fatal("no work notification")
	}
	next, err := s.assignPiece([]byte{0x80})
	if err != nil {
		t.Fatal(err)
	}
	s.release(old)
	if s.assignments[0] != next.ID {
		t.Fatal("stale release removed new owner")
	}
	if complete, err := s.receive(old, types.DataBlock{PieceIndex: 0, Data: []byte("test")}); err != nil || complete {
		t.Fatal("stale response accepted")
	}
	snapshot, _ = s.store.Piece(0)
	if snapshot.Blocks[0].Requested || snapshot.Blocks[0].Received {
		t.Fatal("outstanding block not reset")
	}
}

func TestHashFailureResetsPiece(t *testing.T) {
	s, _ := newTestScheduler(t)
	if err := s.prepareStorage(); err != nil {
		t.Fatal(err)
	}
	a, _ := s.assignPiece([]byte{0x80})
	if err := s.requested(a, a.Blocks[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := s.receive(a, types.DataBlock{PieceIndex: 0, Data: []byte("bad!")}); err == nil {
		t.Fatal("invalid hash accepted")
	}
	s.release(a)
	state, _ := s.store.Piece(0)
	if state.Complete || state.Blocks[0].Received || state.Blocks[0].Requested {
		t.Fatal("failed piece was not reset")
	}
	if _, err := s.assignPiece([]byte{0x80}); err != nil {
		t.Fatal(err)
	}
}

func TestReassignmentIncludesOnlyUnreceivedBlocks(t *testing.T) {
	s, _ := newTestScheduler(t)
	data := bytes.Repeat([]byte{'x'}, types.BlockSize+7)
	hash := sha1.Sum(data)
	length := int64(len(data))
	var err error
	s.store, err = storage.NewStorageHandler(torrentx.ParsedTorrent{
		Torrent:  torrentx.Torrent{Info: torrentx.TorrentInfo{PieceLength: length, Pieces: string(hash[:]), Length: &length}},
		Layout:   []torrentx.FileLayout{{Path: filepath.Join(t.TempDir(), "data"), Length: length}},
		Metadata: torrentx.ParsedTorrentMetadata{TotalLength: length},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.prepareStorage(); err != nil {
		t.Fatal(err)
	}
	a, err := s.assignPiece([]byte{0x80})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range a.Blocks {
		if err = s.requested(a, b); err != nil {
			t.Fatal(err)
		}
	}
	if done, err := s.receive(a, types.DataBlock{PieceIndex: 0, Data: data[:types.BlockSize]}); err != nil || done {
		t.Fatalf("partial store: %v %v", done, err)
	}
	s.release(a)
	next, err := s.assignPiece([]byte{0x80})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Blocks) != 1 || next.Blocks[0].Begin != types.BlockSize || next.Blocks[0].Length != 7 {
		t.Fatalf("incorrect resume: %+v", next)
	}
	if err = s.requested(next, next.Blocks[0]); err != nil {
		t.Fatal(err)
	}
	if done, err := s.receive(next, types.DataBlock{PieceIndex: 0, Begin: types.BlockSize, Data: data[types.BlockSize:]}); err != nil || !done {
		t.Fatalf("resume completion: %v %v", done, err)
	}
}
