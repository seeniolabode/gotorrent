package torrentx

import (
	"io"

	bencodego "github.com/jackpal/bencode-go"
)

func Decode(reader io.Reader) (Torrent, error) {
	var torrent Torrent

	if err := bencodego.Unmarshal(reader, &torrent); err != nil {
		return Torrent{}, err
	}

	return torrent, nil
}
