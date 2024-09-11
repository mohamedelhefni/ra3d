package main

import (
	"os"
	"ra3d/torrent"
)

func main() {
	torrentFile := os.Args[1]
	downloadFile := os.Args[2]
	bencodeTorrent := torrent.TorrentFile{}
	err := bencodeTorrent.Open(torrentFile)
	if err != nil {
		panic(err)
	}

	err = bencodeTorrent.DownloadToFile(downloadFile)
	if err != nil {
		panic(err)
	}

}
