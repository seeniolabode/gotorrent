package torrentx

import (
	"errors"
	"strings"
)

type torrent_type int

const (
	single_file_torrent_type torrent_type = iota
	multi_file_torrent_type
	unknown_file_torrent_type
)

type FileLayout struct {
	Path   string
	Offset int64
	Length int64
}

type DirectoryLayoutBuilderOptions struct {
	PathPrefix string
}

func BuildFileLayout(t Torrent, o DirectoryLayoutBuilderOptions) ([]FileLayout, error) {
	t_type, err := getTorrentType(t)

	if err != nil {
		return []FileLayout{}, err
	}

	if t_type == single_file_torrent_type {
		single_file_layout := FileLayout{
			Path:   buildPath([]string{o.PathPrefix, t.Info.Name}),
			Offset: 0,
			Length: *t.Info.Length,
		}
		return []FileLayout{single_file_layout}, nil
	}

	var offset int64
	var layout []FileLayout

	for _, file := range t.Info.Files {
		path := buildPath(append([]string{o.PathPrefix, t.Info.Name}, file.Path...))

		file_layout := FileLayout{
			Path:   path,
			Offset: offset,
			Length: file.Length,
		}

		layout = append(layout, file_layout)

		offset += file.Length
	}

	return layout, nil
}

func getTorrentType(t Torrent) (torrent_type, error) {
	info := t.Info

	hasLength := info.Length != nil
	hasFiles := info.Files != nil

	if hasLength == hasFiles {
		return unknown_file_torrent_type, errors.New("Invalid torrent")
	}

	if hasLength {
		return single_file_torrent_type, nil
	}

	return multi_file_torrent_type, nil

}

func buildPath(path []string) string {
	return strings.Join(path, "/")
}
