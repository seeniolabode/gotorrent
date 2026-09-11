package torrentx

import (
	"testing"
)

func TestValidateFileLayoutWithCollidingPaths(t *testing.T) {
	file_layout := []FileLayout{
		{
			Path:   "Path 1/Path 2/Final Path",
			Offset: 0,
			Length: 5,
		},
		{
			Path:   "Path 1/Path 2/Final Path",
			Offset: 5,
			Length: 5,
		},
	}

	err := ValidateFileLayout(file_layout)

	if err == nil {
		t.Fatalf("Expected same path layout to fail")
	}
}

func TestValidateFileLayoutCollidingFileAndDirectory(t *testing.T) {
	file_layout := []FileLayout{
		{
			Path:   "Path 1/Path 2/DirFile",
			Offset: 0,
			Length: 5,
		},
		{
			Path:   "Path 1/Path 2/DirFile/File",
			Offset: 5,
			Length: 5,
		},
	}

	err := ValidateFileLayout(file_layout)

	if err == nil {
		t.Fatalf("Expected colliding file and directory layout to fail")
	}

}
