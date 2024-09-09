package main

import (
	"ra3d/torrent"
)

func main() {
	magnet := "magnet:?xt=urn:btih:DPIIR3URM2QGFT2K6COPTFZA7JXBUMJT&dn=debian-12.7.0-amd64-netinst.iso&xl=661651456&tr=http%3A%2F%2Fbttracker.debian.org%3A6969%2Fannounce"
	bencode := torrent.BencodeMagnet{}
	file, err := bencode.Decode(magnet)
	if err != nil {
		panic(err)
	}
	tracker := torrent.Tracker{File: file}
  err = tracker.DownloadToFile()
  if err != nil {
    panic(err)
  }
}
