package scheduler

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/seeniolabode/gotorrent/internals/storage"
	"github.com/seeniolabode/gotorrent/internals/torrentx"
	"github.com/seeniolabode/gotorrent/internals/tracker"
	"github.com/seeniolabode/gotorrent/internals/tracker/udp"
)

type trackerClient interface {
	tracker.Tracker
	Run() (tracker.TrackingResult, error)
}

// Scheduler owns the torrent state and coordinates a single peer session at a time.
// A Scheduler must not be copied after first use.
type Scheduler struct {
	torrent torrentx.ParsedTorrent
	store   *storage.Storage
	tracker trackerClient
	running atomic.Bool
}

type NewSchedulerOptions struct {
	TorrentPath      string
	RelativeLocation string
}

// NewScheduler reads and validates the torrent and builds idle dependencies.
// It does not contact trackers or peers, create download files, or load piece state.
func NewScheduler(o NewSchedulerOptions) (*Scheduler, error) {
	if o.TorrentPath == "" {
		return nil, fmt.Errorf("torrent path is empty")
	}
	file, err := os.Open(o.TorrentPath)
	if err != nil {
		return nil, fmt.Errorf("open torrent: %w", err)
	}
	defer file.Close()

	torrent, err := torrentx.Parse(file, torrentx.ParseOptions{PathsPrefix: o.RelativeLocation})
	if err != nil {
		return nil, fmt.Errorf("parse torrent: %w", err)
	}
	store, err := storage.NewStorageHandler(torrent)
	if err != nil {
		return nil, fmt.Errorf("create storage: %w", err)
	}
	tracker, err := udp.NewUDPTrackerClient(torrent)
	if err != nil {
		return nil, fmt.Errorf("create tracker: %w", err)
	}
	return &Scheduler{torrent: torrent, store: store, tracker: tracker}, nil
}
