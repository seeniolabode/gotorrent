package torrentx

import "testing"

func TestErrorOnInvalidTorrent(t *testing.T) {
	var length int64 = 10

	test_case := Torrent{
		Info: TorrentInfo{
			Length: &length,
			Files:  []TorrentFile{},
		},
	}

	_, err := buildFileLayout(test_case, FileLayoutBuilderOptions{
		DownloadPath: "Downloads",
	})

	if err == nil {
		t.Fatalf("Expected error")
	}

}

func TestValidMultiFileLayout(t *testing.T) {
	var file_length int64 = 5

	test_case := Torrent{
		Info: TorrentInfo{
			Name: "My Folder",
			Files: []TorrentFile{
				{
					Length: file_length,
					Path:   []string{"Path", "Final Path", "file.ts"},
				},
				{
					Length: file_length,
					Path:   []string{"Path 2", "Final Path 2", "file.ts"},
				},
			},
		},
	}

	file_layout, err := buildFileLayout(test_case, FileLayoutBuilderOptions{
		DownloadPath: "Downloads",
	})

	if err != nil {
		t.Fatalf("Expected that file layout was built: %s", err)
	}

	first_file := file_layout[0]
	expected_path := "Downloads/My Folder/Path/Final Path/file.ts"

	if first_file.Path != expected_path {
		t.Fatalf("Got unexpected file path: %s, expected: %s", first_file.Path, expected_path)
	}

	if first_file.Length != file_length {
		t.Fatalf("Got unexpected file length")
	}

	if first_file.Offset != 0 {
		t.Fatalf("Expected first file offset to be 0")
	}

	second_file := file_layout[1]
	expected_path = "Downloads/My Folder/Path 2/Final Path 2/file.ts"

	if second_file.Path != expected_path {
		t.Fatalf("Got unexpected file path: %s, expected: %s", first_file.Path, expected_path)
	}

	if second_file.Length != file_length {
		t.Fatalf("Got unexpected file length")
	}

	if second_file.Offset != 5 {
		t.Fatalf("Expected second file offset to be 5")
	}

}

func TestSingleFileLayout(t *testing.T) {
	var file_length int64 = 5

	test_case := Torrent{
		Info: TorrentInfo{
			Name:   "My Folder",
			Length: &file_length,
		},
	}

	file_layout, err := buildFileLayout(test_case, FileLayoutBuilderOptions{
		DownloadPath: "Downloads",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if len(file_layout) != 1 {
		t.Fatalf("Expected a single file in file layout")
	}

	single_file_layout := file_layout[0]

	if single_file_layout.Length != file_length {
		t.Fatalf("Expected single file length to be %d", file_length)
	}

	expected_path := "Downloads/My Folder"
	if single_file_layout.Path != expected_path {
		t.Fatalf("Got layout: %s, expected: %s", single_file_layout.Path, expected_path)
	}
}
