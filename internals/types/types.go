package types

type DataBlock struct {
	PieceIndex int
	Begin      int
	Data       []byte
}

const BlockSize = 16 * 1024
