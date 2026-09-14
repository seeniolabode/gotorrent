package storage

type BlockState struct {
	Begin     int
	Length    int
	Received  bool
	Requested bool
}

type PieceState struct {
	Index    int
	Length   int
	Blocks   []BlockState
	Complete bool
}
