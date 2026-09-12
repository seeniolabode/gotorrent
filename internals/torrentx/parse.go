package torrentx

import "io"

type ParsedTorrent struct {
	Torrent  Torrent
	Layout   []FileLayout
	Metadata ParsedTorrentMetadata
}

type ParsedTorrentMetadata struct {
	InfoHash    [20]byte
	TotalLength int64
}

type ParseOptions struct {
	PathsPrefix string
}

func Parse(tr io.Reader, o ParseOptions) (ParsedTorrent, error) {
	decoded_torrent, info_hash, err := decode(tr)

	if err != nil {
		return ParsedTorrent{}, err
	}

	err = validateTorrent(decoded_torrent)

	if err != nil {
		return ParsedTorrent{}, err
	}

	torrent_file_layout, err := buildFileLayout(decoded_torrent, FileLayoutBuilderOptions{
		DownloadPath: o.PathsPrefix,
	})

	if err != nil {
		return ParsedTorrent{}, err
	}

	parsed_torrent := ParsedTorrent{
		Torrent: decoded_torrent,
		Layout:  torrent_file_layout,
		Metadata: ParsedTorrentMetadata{
			InfoHash:    info_hash,
			TotalLength: totalLenth(torrent_file_layout),
		},
	}

	return parsed_torrent, nil

}
