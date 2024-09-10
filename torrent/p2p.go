package torrent

import (
	"fmt"
)

// MaxBlockSize is the largest number of bytes a request can ask for
const MaxBlockSize = 16384

// MaxBacklog is the number of unfulfilled requests a client can have in its pipeline
const MaxBacklog = 5

type Torrent struct {
	Peers []Peer
	File  TorrentFile
}

type pieceWork struct {
	index  int
	hash   string
	length int
}

type pieceResult struct {
	index int
	buf   []byte
}

type pieceProgress struct {
	index      int
	buf        []byte
	downloaded int
	requested  int
	backlog    int
	// client *Client
}

func (t Torrent) Download() ([]byte, error) {
	fmt.Println("start  download for", t.File.DisplayName)

	return nil, nil
}
