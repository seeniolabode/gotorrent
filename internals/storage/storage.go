package storage

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/seeniolabode/gotorrent/internals/torrentx"
	"github.com/seeniolabode/gotorrent/internals/types"
)

type Storage struct {
	Pieces        []PieceState
	ParsedTorrent torrentx.ParsedTorrent
}

// Read

func (s *Storage) Load() error {
	if len(s.ParsedTorrent.Layout) == 0 {
		return errors.New("empty torrent")
	}

	info := s.ParsedTorrent.Torrent.Info
	pieceHashes := []byte(info.Pieces)

	if len(pieceHashes)%sha1.Size != 0 {
		return errors.New("invalid torrent piece hashes")
	}

	pieceCount := len(pieceHashes) / sha1.Size

	s.Pieces = make([]PieceState, pieceCount)

	for i := range pieceCount {
		pieceLength := s.pieceLength(i)
		pieceOffset := int64(i) * info.PieceLength

		data := make([]byte, pieceLength)

		if err := s.ReadAt(data, pieceOffset); err != nil {
			return fmt.Errorf("read piece %d: %w", i, err)
		}

		actualHash := sha1.Sum(data)
		expectedHash := pieceHashes[i*sha1.Size : (i+1)*sha1.Size]

		complete := bytes.Equal(
			actualHash[:],
			expectedHash,
		)

		s.Pieces[i] = PieceState{
			Index:    i,
			Length:   pieceLength,
			Blocks:   buildBlocks(pieceLength, complete),
			Complete: complete,
		}
	}

	return nil
}

func (s *Storage) ReadAt(buf []byte, offset int64) error {
	if len(s.ParsedTorrent.Layout) == 0 {
		return errors.New("empty torrent")
	}

	if len(buf) == 0 {
		return nil
	}

	if offset < 0 {
		return errors.New("offset cannot be negative")
	}

	end := offset + int64(len(buf))

	if end > s.ParsedTorrent.Metadata.TotalLength {
		return io.ErrUnexpectedEOF
	}

	remaining := buf
	currentOffset := offset

	for _, file := range s.ParsedTorrent.Layout {
		fileStart := file.Offset
		fileEnd := file.Offset + file.Length

		if currentOffset < fileStart || currentOffset >= fileEnd {
			continue
		}

		offsetInFile := currentOffset - fileStart

		available := file.Length - offsetInFile
		toRead := int64(len(remaining))

		if available < toRead {
			toRead = available
		}

		f, err := os.Open(file.Path)
		if err != nil {
			return fmt.Errorf("open %s: %w", file.Path, err)
		}

		n, readErr := f.ReadAt(
			remaining[:int(toRead)],
			offsetInFile,
		)

		closeErr := f.Close()

		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("read %s: %w", file.Path, readErr)
		}

		if closeErr != nil {
			return fmt.Errorf("close %s: %w", file.Path, closeErr)
		}

		if n != int(toRead) {
			return io.ErrUnexpectedEOF
		}

		remaining = remaining[int(toRead):]
		currentOffset += toRead

		if len(remaining) == 0 {
			return nil
		}
	}

	return io.ErrUnexpectedEOF
}

func (s *Storage) HasPiece(pieceIndex int) bool {
	if pieceIndex < 0 || pieceIndex >= len(s.Pieces) {
		return false
	}

	return s.Pieces[pieceIndex].Complete
}

func (s *Storage) MissingPieces() []int {
	var missing []int

	for _, piece := range s.Pieces {
		if !piece.Complete {
			missing = append(missing, piece.Index)
		}
	}

	return missing
}

// Write

func (s *Storage) Store(block types.DataBlock) error {
	if block.PieceIndex < 0 || block.PieceIndex >= len(s.Pieces) {
		return fmt.Errorf(
			"invalid piece index: %d",
			block.PieceIndex,
		)
	}

	piece := &s.Pieces[block.PieceIndex]

	if piece.Complete {
		return nil
	}

	if block.Begin < 0 {
		return errors.New("block begin cannot be negative")
	}

	if len(block.Data) == 0 {
		return errors.New("block data is empty")
	}

	if block.Begin+len(block.Data) > piece.Length {
		return fmt.Errorf(
			"block exceeds piece bounds: piece=%d begin=%d length=%d piece_length=%d",
			block.PieceIndex,
			block.Begin,
			len(block.Data),
			piece.Length,
		)
	}

	var blockState *BlockState

	for i := range piece.Blocks {
		if piece.Blocks[i].Begin == block.Begin {
			blockState = &piece.Blocks[i]
			break
		}
	}

	if blockState == nil {
		return fmt.Errorf(
			"unexpected block offset %d for piece %d",
			block.Begin,
			block.PieceIndex,
		)
	}

	if len(block.Data) != blockState.Length {
		return fmt.Errorf(
			"unexpected block length for piece %d at %d: got %d, expected %d",
			block.PieceIndex,
			block.Begin,
			len(block.Data),
			blockState.Length,
		)
	}

	if blockState.Received {
		return nil
	}

	pieceOffset :=
		int64(block.PieceIndex) *
			s.ParsedTorrent.Torrent.Info.PieceLength

	blockOffset :=
		pieceOffset +
			int64(block.Begin)

	if err := s.WriteAt(block.Data, blockOffset); err != nil {
		return fmt.Errorf(
			"write piece %d block %d: %w",
			block.PieceIndex,
			block.Begin,
			err,
		)
	}

	blockState.Received = true
	log.Printf("Block written: piece=%d begin=%d length=%d", block.PieceIndex, block.Begin, len(block.Data))

	if !pieceHasAllBlocksReceived(piece) {
		return nil
	}

	valid, err := s.verifyPiece(piece.Index)
	if err != nil {
		return err
	}

	if !valid {
		piece.Complete = false

		for i := range piece.Blocks {
			piece.Blocks[i].Received = false
		}

		return fmt.Errorf(
			"piece %d failed hash verification",
			piece.Index,
		)
	}

	piece.Complete = true
	log.Printf("Piece complete: piece=%d length=%d", piece.Index, piece.Length)

	return nil
}

func (s *Storage) WriteAt(data []byte, offset int64) error {
	if len(s.ParsedTorrent.Layout) == 0 {
		return errors.New("empty torrent")
	}

	if len(data) == 0 {
		return nil
	}

	if offset < 0 {
		return errors.New("offset cannot be negative")
	}

	end := offset + int64(len(data))

	if end > s.ParsedTorrent.Metadata.TotalLength {
		return io.ErrShortWrite
	}

	remaining := data
	currentOffset := offset

	for _, file := range s.ParsedTorrent.Layout {
		fileStart := file.Offset
		fileEnd := file.Offset + file.Length

		if currentOffset < fileStart || currentOffset >= fileEnd {
			continue
		}

		offsetInFile := currentOffset - fileStart

		available := file.Length - offsetInFile
		toWrite := int64(len(remaining))

		if available < toWrite {
			toWrite = available
		}

		f, err := os.OpenFile(
			file.Path,
			os.O_WRONLY,
			0644,
		)
		if err != nil {
			return fmt.Errorf("open %s: %w", file.Path, err)
		}

		n, writeErr := f.WriteAt(
			remaining[:int(toWrite)],
			offsetInFile,
		)

		closeErr := f.Close()

		if writeErr != nil {
			return fmt.Errorf("write %s: %w", file.Path, writeErr)
		}

		if closeErr != nil {
			return fmt.Errorf("close %s: %w", file.Path, closeErr)
		}

		if n != int(toWrite) {
			return io.ErrShortWrite
		}

		remaining = remaining[int(toWrite):]
		currentOffset += toWrite

		if len(remaining) == 0 {
			return nil
		}
	}

	return io.ErrShortWrite
}

// Piece helpers

func (s *Storage) pieceLength(pieceIndex int) int {
	info := s.ParsedTorrent.Torrent.Info

	pieceOffset := int64(pieceIndex) * info.PieceLength
	remaining := s.ParsedTorrent.Metadata.TotalLength - pieceOffset

	if remaining < info.PieceLength {
		return int(remaining)
	}

	return int(info.PieceLength)
}

func buildBlocks(pieceLength int, received bool) []BlockState {
	blockCount := (pieceLength + types.BlockSize - 1) / types.BlockSize

	blocks := make([]BlockState, 0, blockCount)

	for begin := 0; begin < pieceLength; begin += types.BlockSize {
		length := types.BlockSize

		if remaining := pieceLength - begin; remaining < types.BlockSize {
			length = remaining
		}

		blocks = append(blocks, BlockState{
			Begin:    begin,
			Length:   length,
			Received: received,
		})
	}

	return blocks
}

func pieceHasAllBlocksReceived(piece *PieceState) bool {
	for _, block := range piece.Blocks {
		if !block.Received {
			return false
		}
	}

	return true
}

func (s *Storage) verifyPiece(pieceIndex int) (bool, error) {
	if pieceIndex < 0 || pieceIndex >= len(s.Pieces) {
		return false, fmt.Errorf(
			"invalid piece index: %d",
			pieceIndex,
		)
	}

	piece := &s.Pieces[pieceIndex]

	data := make([]byte, piece.Length)

	pieceOffset :=
		int64(pieceIndex) *
			s.ParsedTorrent.Torrent.Info.PieceLength

	if err := s.ReadAt(data, pieceOffset); err != nil {
		return false, fmt.Errorf(
			"read piece %d for verification: %w",
			pieceIndex,
			err,
		)
	}

	actualHash := sha1.Sum(data)

	pieceHashes := []byte(
		s.ParsedTorrent.Torrent.Info.Pieces,
	)

	hashStart := pieceIndex * sha1.Size
	hashEnd := hashStart + sha1.Size

	if hashEnd > len(pieceHashes) {
		return false, errors.New(
			"piece hash outside torrent metadata",
		)
	}

	expectedHash := pieceHashes[hashStart:hashEnd]

	return bytes.Equal(
		actualHash[:],
		expectedHash,
	), nil
}

// Utils

func (s *Storage) PrepareFileSystem() error {
	for _, file := range s.ParsedTorrent.Layout {
		dir := filepath.Dir(file.Path)

		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf(
				"create directory %q: %w",
				dir,
				err,
			)
		}

		info, err := os.Stat(file.Path)

		exists := err == nil

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(
				"check file %q: %w",
				file.Path,
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
				"open file %q: %w",
				file.Path,
				err,
			)
		}

		if !exists || info.Size() != file.Length {
			if err := f.Truncate(file.Length); err != nil {
				f.Close()

				return fmt.Errorf(
					"resize file %q: %w",
					file.Path,
					err,
				)
			}
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

func NewStorageHandler(t torrentx.ParsedTorrent) (*Storage, error) {
	s := Storage{
		ParsedTorrent: t,
		Pieces:        nil,
	}

	return &s, nil
}
