package torrentx

type Torrent struct {
	Announce     string      `bencode:"announce"`
	AnnounceList [][]string  `bencode:"announce-list"`
	Info         TorrentInfo `bencode:"info"`
}

type TorrentInfo struct {
	Name        string `bencode:"name"`
	PieceLength int64  `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`

	// Single-file torrent
	Length *int64 `bencode:"length"`

	// Multi-file torrent
	Files []TorrentFile `bencode:"files"`
}

type TorrentFile struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}
