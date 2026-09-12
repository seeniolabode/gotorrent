package torrentx

import (
	"bytes"
	"crypto/sha1"
	"io"

	bencodego "github.com/jackpal/bencode-go"
)

type rawTorrent struct {
	Announce     string               `bencode:"announce"`
	AnnounceList [][]string           `bencode:"announce-list"`
	RawInfo      bencodego.RawMessage `bencode:"info"`
}

func decode(reader io.Reader) (Torrent, [20]byte, error) {
	var raw rawTorrent

	if err := bencodego.Unmarshal(reader, &raw); err != nil {
		return Torrent{}, [20]byte{}, err
	}

	var info TorrentInfo

	if err := bencodego.Unmarshal(
		bytes.NewReader(raw.RawInfo),
		&info,
	); err != nil {
		return Torrent{}, [20]byte{}, err
	}

	torrent := Torrent{
		Announce:     raw.Announce,
		AnnounceList: raw.AnnounceList,
		Info:         info,
	}

	infoHash := sha1.Sum(raw.RawInfo)

	return torrent, infoHash, nil
}
