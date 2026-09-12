package fs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/seeniolabode/gotorrent/internals/torrentx"
)

func PrepareFileSystem(layout []torrentx.FileLayout) error {
	for _, file := range layout {
		dir := filepath.Dir(file.Path)

		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf(
				"create directory %q: %w",
				dir,
				err,
			)
		}

		f, err := os.OpenFile(
			file.Path,
			os.O_CREATE|os.O_RDWR,
			0644,
		)
		if err != nil {
			return fmt.Errorf(
				"create file %q: %w",
				file.Path,
				err,
			)
		}

		if err := f.Truncate(file.Length); err != nil {
			f.Close()

			return fmt.Errorf(
				"resize file %q: %w",
				file.Path,
				err,
			)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf(
				"close file %q: %w",
				file.Path,
				err,
			)
		}
	}

	return nil
}
