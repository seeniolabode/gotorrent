package torrentx

import (
	"errors"
	"path/filepath"
)

type torrentType int

const (
	singleFileTorrent torrentType = iota
	multiFileTorrent
)

type FileLayout struct {
	Path   string
	Offset int64
	Length int64
}

type FileLayoutBuilderOptions struct {
	DownloadPath string
}

func BuildFileLayout(t Torrent, opts FileLayoutBuilderOptions) ([]FileLayout, error) {
	torrentType, err := getTorrentType(t)
	if err != nil {
		return nil, err
	}

	if torrentType == singleFileTorrent {
		return []FileLayout{
			{
				Path: filepath.Join(
					opts.DownloadPath,
					t.Info.Name,
				),
				Offset: 0,
				Length: *t.Info.Length,
			},
		}, nil
	}

	layout := make([]FileLayout, 0, len(t.Info.Files))

	var offset int64

	for _, file := range t.Info.Files {
		parts := append(
			[]string{
				opts.DownloadPath,
				t.Info.Name,
			},
			file.Path...,
		)

		layout = append(layout, FileLayout{
			Path:   filepath.Join(parts...),
			Offset: offset,
			Length: file.Length,
		})

		offset += file.Length
	}

	return layout, nil
}

func getTorrentType(t Torrent) (torrentType, error) {
	hasLength := t.Info.Length != nil
	hasFiles := t.Info.Files != nil

	if hasLength == hasFiles {
		return 0, errors.New(
			"invalid torrent: expected exactly one of length or files",
		)
	}

	if hasLength {
		return singleFileTorrent, nil
	}

	return multiFileTorrent, nil
}
