package scheduler

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/seeniolabode/gotorrent/internals/tracker"
)

type failingTracker struct {
	calls int
	err   error
}

func (f *failingTracker) Run() (tracker.TrackingResult, error) {
	f.calls++
	return tracker.TrackingResult{}, f.err
}
func (f *failingTracker) Harvest() (tracker.TrackerHarvest, error) {
	return tracker.TrackerHarvest{}, errors.New("unexpected harvest")
}

func newTestScheduler(t *testing.T) (*Scheduler, string) {
	t.Helper()
	root := t.TempDir()
	torrentPath := filepath.Join(root, "test.torrent")
	hash := sha1.Sum([]byte("test"))
	data := fmt.Sprintf("d8:announce17:udp://127.0.0.1:14:infod5:filesld6:lengthi4e4:pathl8:test.bineee4:name4:test12:piece lengthi4e6:pieces20:%see", hash[:])
	if err := os.WriteFile(torrentPath, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "download")
	s, err := NewScheduler(NewSchedulerOptions{TorrentPath: torrentPath, RelativeLocation: destination})
	if err != nil {
		t.Fatal(err)
	}
	return s, destination
}

func TestConstructorLeavesDownloadsUntouched(t *testing.T) {
	s, destination := newTestScheduler(t)
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("constructor created download directory: %v", err)
	}
	if s.store == nil || s.tracker == nil || s.store.Pieces != nil {
		t.Fatal("expected idle dependencies without loaded piece state")
	}
}

func TestStartPreparesStorageAndCanRetryTrackerFailure(t *testing.T) {
	s, _ := newTestScheduler(t)
	failure := errors.New("tracker unavailable")
	fake := &failingTracker{err: failure}
	s.tracker = fake
	for range 2 {
		if err := s.Start(); !errors.Is(err, failure) {
			t.Fatalf("got %v", err)
		}
	}
	if fake.calls != 2 {
		t.Fatalf("tracker called %d times", fake.calls)
	}
	if len(s.store.MissingPieces()) != 1 {
		t.Fatal("piece state was not loaded")
	}
	if _, err := os.Stat(s.torrent.Layout[0].Path); err != nil {
		t.Fatal(err)
	}
}

func TestStartSkipsTrackerForCompletedDownload(t *testing.T) {
	s, _ := newTestScheduler(t)
	if err := s.store.PrepareFileSystem(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.torrent.Layout[0].Path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	fake := &failingTracker{err: errors.New("must not run")}
	s.tracker = fake
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 0 {
		t.Fatal("announced completed download")
	}
}

func TestStartRejectsOverlappingRun(t *testing.T) {
	s, destination := newTestScheduler(t)
	s.running.Store(true)
	if err := s.Start(); err == nil {
		t.Fatal("expected overlapping start to fail")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("overlapping start touched storage")
	}
}
