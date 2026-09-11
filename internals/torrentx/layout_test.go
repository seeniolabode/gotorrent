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

	_, err := BuildFileLayout(test_case, DirectoryLayoutBuilderOptions{
		PathPrefix: "Downloads",
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

	file_layout, err := BuildFileLayout(test_case, DirectoryLayoutBuilderOptions{
		PathPrefix: "Downloads",
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
