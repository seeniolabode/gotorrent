package main

import (
	"log"

	"github.com/seeniolabode/gotorrent/internals/scheduler"
)

type ProgramConfig struct {
	torrent_path  string
	download_path string
}

func main() {

	config := ProgramConfig{
		torrent_path:  "./torrents/The Beatles-With The Beatles.torrent",
		download_path: "downloads",
	}

	scheduler, err := scheduler.NewScheduler(scheduler.NewSchedulerOptions{
		TorrentPath:      config.torrent_path,
		RelativeLocation: config.download_path,
	})

	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	if err := scheduler.Start(); err != nil {
		log.Fatalf("Error running scheduler: %s", err)
	}
}
