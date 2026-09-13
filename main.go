package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/seeniolabode/gotorrent/internals/p2p/peer"
	"github.com/seeniolabode/gotorrent/internals/p2p/tracker/udp"
	"github.com/seeniolabode/gotorrent/internals/storage"
	"github.com/seeniolabode/gotorrent/internals/torrentx"
	"github.com/seeniolabode/gotorrent/internals/types"
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

	fmt.Println("Running tracker")

	tracking_result, err := udpTracker.Run()

	if err != nil {
		log.Fatalf("Error running torrent tracker: %s", err)
	}

	fmt.Printf("Found %d peers\n", len(tracking_result.Peers))

	storageHandler, err := storage.NewStorageHandler(torrent)

	if err != nil {
		log.Fatalf("Couldn't create storage handler")
	}

	fmt.Println("Preparing file system")

	if err = storageHandler.PrepareFileSystem(); err != nil {
		log.Fatalf("Error setting up file layout: %s", err)
	}

	fmt.Println("File system prepared")

	if err := storageHandler.Load(); err != nil {
		log.Fatalf("Error loading piece state: %s", err)
	}
	log.Printf("Piece state loaded: total=%d missing=%d", len(storageHandler.Pieces), len(storageHandler.MissingPieces()))

	p2pClient, err := peer.NewP2PClientFromTracker(udpTracker,
		func(p *peer.PeerConnection) (index, length int, err error) {
			for _, pieceIndex := range storageHandler.MissingPieces() {
				if !peer.HasPiece(p.Bitfield, pieceIndex) {
					continue
				}

				piece := storageHandler.Pieces[pieceIndex]
				return piece.Index, piece.Length, nil
			}

			return 0, 0, errors.New("peer has no pieces we need")
		},
	)

	if err != nil {
		log.Fatalf("Error creating client from tracker: %s", err)
	}

	fmt.Println("Attempting to connect to a peer")

	peerConnection, err := p2pClient.ConnectSinglePeer()

	if err != nil {
		log.Fatalf("Error connecting client: %s", err)
	}

	fmt.Printf("Connected to peer with IP: %s\n", peerConnection.IP)

	err = peerConnection.Use(peer.UseOptions{
		HandshakeOptions: peer.HandshakeOptions{
			PeerID:   p2pClient.PeerID,
			InfoHash: torrent.Metadata.InfoHash,
		},
		ListenOptions: peer.ListenOptions{
			OnBlock: func(b types.DataBlock) (bool, error) {
				if err := storageHandler.Store(b); err != nil {
					return false, err
				}

				piece := storageHandler.Pieces[b.PieceIndex]

				return piece.Complete, nil
			},
			IsInterested: func(p *peer.PeerConnection) bool {
				for _, piece := range storageHandler.MissingPieces() {
					if peer.HasPiece(p.Bitfield, piece) {
						return true
					}
				}

				return false
			},
		},
	})

	if err != nil {
		log.Fatalf("Error: %s", err)
	}
}
