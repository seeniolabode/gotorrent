package storage

import (
	"crypto/sha1"
	"path/filepath"
	"testing"

	"github.com/seeniolabode/gotorrent/internals/torrentx"
	"github.com/seeniolabode/gotorrent/internals/types"
)

func TestLoadPreparedStorageMakesMissingPiecesAvailable(t *testing.T) {
	data := []byte("test piece")
	hash := sha1.Sum(data)
	length := int64(len(data))
	s, err := NewStorageHandler(torrentx.ParsedTorrent{
		Torrent: torrentx.Torrent{Info: torrentx.TorrentInfo{
			PieceLength: length, Pieces: string(hash[:]), Length: &length,
		}},
		Layout:   []torrentx.FileLayout{{Path: filepath.Join(t.TempDir(), "download"), Length: length}},
		Metadata: torrentx.ParsedTorrentMetadata{TotalLength: length},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PrepareFileSystem(); err != nil {
		t.Fatal(err)
	}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	missing := s.MissingPieces()
	if len(missing) != 1 || missing[0] != 0 || s.Pieces[0].Length != len(data) {
		t.Fatalf("expected piece 0 available for assignment, got %+v", s.Pieces)
	}
	if err := s.Store(types.DataBlock{PieceIndex: 0, Begin: 0, Data: data}); err != nil {
		t.Fatal(err)
	}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if len(s.MissingPieces()) != 0 || !s.Pieces[0].Complete {
		t.Fatal("completed piece was not restored from disk")
	}
}
