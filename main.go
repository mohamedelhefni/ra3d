package main

import (
	"os"
	"ra3d/torrent"
	"ra3d/tui"
)

func main() {
	torrentFile := os.Args[1]

	tuiChan := make(chan tui.TorrentTui)
	tui := tui.TorrentTui{}

	bencodeTorrent := torrent.TorrentFile{}
	err := bencodeTorrent.Open(torrentFile)
	if err != nil {
		panic(err)
	}
	go tui.Listen(tuiChan)
	err = bencodeTorrent.DownloadToFile(tuiChan)
	if err != nil {
		panic(err)
	}

}
