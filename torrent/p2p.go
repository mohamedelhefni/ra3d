package torrent

import (
	"fmt"
	"ra3d/peers"
)

type Torrent struct {
	Peers []peers.Peer
	File  TorrentFile
}

func (t Torrent) Download() ([]byte, error) {
	fmt.Println("start  download for", t.File.DisplayName)
	return nil, nil
}
