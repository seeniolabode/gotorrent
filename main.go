package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/seeniolabode/gotorrent/internals/torrentx"
)

type ProgramConfig struct {
	torrent_path  string
	download_path string
}

func main() {

	config := ProgramConfig{
		torrent_path:  "./torrents/Anora (2024) [1080p] [WEBRip] [5.1] [YTS.MX].torrent",
		download_path: "downloads",
	}

	torrent_file, err := os.Open(config.torrent_path)

	if err != nil {
		log.Fatalf("Error reading file: %s", err)
	}

	defer torrent_file.Close()

	torrent, err := torrentx.Decode(torrent_file)
	if err != nil {
		log.Fatal(err.Error())
	}

	err = torrentx.ValidateTorrent(torrent)

	if err != nil {
		log.Fatalf("Invalid torrent: %s", err)
	}

	file_layout, err := torrentx.BuildFileLayout(torrent, torrentx.FileLayoutBuilderOptions{
		DownloadPath: config.download_path,
	})

	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	data, err := json.MarshalIndent(file_layout, "", " ")
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(string(data))

	// if torrent.Info.Length != nil {
	// 	fmt.Println("Single file torrent")
	// } else {
	// 	fmt.Println("Multi file torrent")
	// }

}
