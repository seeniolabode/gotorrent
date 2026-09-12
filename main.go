package main

import (
	"log"
	"os"

	"github.com/seeniolabode/gotorrent/internals/fs"
	"github.com/seeniolabode/gotorrent/internals/p2p/peer"
	"github.com/seeniolabode/gotorrent/internals/p2p/tracker/udp"
	"github.com/seeniolabode/gotorrent/internals/torrentx"
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

	torrent_file, err := os.Open(config.torrent_path)

	if err != nil {
		log.Fatalf("Error reading file: %s", err)
	}

	defer torrent_file.Close()

	torrent, err := torrentx.Parse(torrent_file, torrentx.ParseOptions{
		PathsPrefix: config.download_path,
	})

	if err != nil {
		log.Fatalf("Error parsing torrent file: %s", err)
	}

	udpTracker, err := udp.NewUDPTrackerClient(torrent)

	if err != nil {
		log.Fatalf("Error creating tracker client: %s", err)
	}

	_, err = udpTracker.Run()

	if err != nil {
		log.Fatalf("Error running torrent tracker: %s", err)
	}

	p2pClient, err := peer.NewP2PClientFromTracker(udpTracker)

	if err != nil {
		log.Fatalf("Error creating client from tracker: %s", err)
	}

	_, err = p2pClient.ConnectSinglePeer()

	if err != nil {
		log.Fatalf("Error connecting client: %s", err)
	}

	if err = fs.PrepareFileSystem(torrent.Layout); err != nil {
		log.Fatalf("Error setting up file layout: %s", err)
	}
}
