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

	err = bencodeTorrent.DownloadToFile("./debain.iso")
	if err != nil {
		panic(err)
	}

}
