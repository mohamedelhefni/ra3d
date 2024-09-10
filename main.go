package main

import (
	"ra3d/torrent"
)

func main() {
	bencodeTorrent := torrent.TorrentFile{}
	err := bencodeTorrent.Open("./debain.torrent")
	if err != nil {
		panic(err)
	}

	_, err = bencodeTorrent.DownloadToFile()
	if err != nil {
		panic(err)
	}

}
