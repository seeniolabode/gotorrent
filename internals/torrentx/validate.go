package torrentx

import (
	"fmt"
)

func ValidateTorrent(t Torrent) error {
	info := t.Info

	if info.Name == "" {
		return fmt.Errorf("torrent name is empty")
	}

	if info.PieceLength <= 0 {
		return fmt.Errorf("piece length must be greater than zero")
	}

	if len(info.Pieces)%20 != 0 {
		return fmt.Errorf(
			"pieces length must be divisible by 20, got %d bytes",
			len(info.Pieces),
		)
	}

	hasLength := info.Length != nil
	hasFiles := info.Files != nil

	if hasLength == hasFiles {
		return fmt.Errorf("torrent must contain exactly one of length or files")
	}

	var totalLength int64

	if hasLength {
		if *info.Length < 0 {
			return fmt.Errorf("torrent length cannot be negative")
		}

		totalLength = *info.Length
	}

	if hasFiles {
		if len(info.Files) == 0 {
			return fmt.Errorf("multi-file torrent contains no files")
		}

		for i, file := range info.Files {
			if file.Length < 0 {
				return fmt.Errorf("file %d has negative length", i)
			}

			if len(file.Path) == 0 {
				return fmt.Errorf("file %d has an empty path", i)
			}

			for _, part := range file.Path {
				if part == "" {
					return fmt.Errorf("file %d contains an empty path component", i)
				}

				if part == "." || part == ".." {
					return fmt.Errorf(
						"file %d contains unsafe path component %q",
						i,
						part,
					)
				}
			}

			totalLength += file.Length
		}
	}

	pieceCount := int64(len(info.Pieces) / 20)

	var expectedPieceCount int64

	if totalLength > 0 {
		expectedPieceCount =
			(totalLength + info.PieceLength - 1) / info.PieceLength
	}

	if pieceCount != expectedPieceCount {
		return fmt.Errorf(
			"invalid piece count: expected %d, got %d",
			expectedPieceCount,
			pieceCount,
		)
	}

	return nil
}
