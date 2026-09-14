package scheduler

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seeniolabode/gotorrent/internals/storage"
	"github.com/seeniolabode/gotorrent/internals/torrentx"
	"github.com/seeniolabode/gotorrent/internals/tracker"
	"github.com/seeniolabode/gotorrent/internals/tracker/udp"
)

type trackerClient interface {
	tracker.Tracker
	Run() (tracker.TrackingResult, error)
}

// Scheduler owns the torrent state and coordinates concurrent peer sessions.
// A Scheduler must not be copied after first use.
type Scheduler struct {
	torrent        torrentx.ParsedTorrent
	store          *storage.Storage
	tracker        trackerClient
	running        atomic.Bool
	mu             sync.Mutex
	assignments    map[int]uint64
	nextAssignment uint64
	work           chan struct{}
	pipelineLimit  int
	maxPeers       int
	requestTimeout time.Duration
}

type NewSchedulerOptions struct {
	TorrentPath      string
	RelativeLocation string
	// PipelineLimit caps outstanding block requests per peer; zero uses 5.
	PipelineLimit int
	// MaxPeers caps concurrent sessions; zero uses 8.
	MaxPeers int
	// RequestTimeout bounds block requests, writes, and idle sessions; zero uses 30s.
	RequestTimeout time.Duration
}

// NewScheduler reads and validates the torrent and builds idle dependencies.
// It does not contact trackers or peers, create download files, or load piece state.
func NewScheduler(o NewSchedulerOptions) (*Scheduler, error) {
	if o.PipelineLimit < 0 || o.MaxPeers < 0 || o.RequestTimeout < 0 {
		return nil, fmt.Errorf("scheduler limits cannot be negative")
	}
	if o.PipelineLimit == 0 {
		o.PipelineLimit = 5
	}
	if o.MaxPeers == 0 {
		o.MaxPeers = 8
	}
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 30 * time.Second
	}
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
	return &Scheduler{torrent: torrent, store: store, tracker: tracker,
		assignments: make(map[int]uint64), work: make(chan struct{}),
		pipelineLimit: o.PipelineLimit, maxPeers: o.MaxPeers, requestTimeout: o.RequestTimeout}, nil
}
