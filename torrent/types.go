package torrent

import (
	"encoding/base32"
	neturl "net/url"
	"strconv"
	"strings"
)

type Bencode interface {
	Decode(data string) (BencodeFile, error)
}

type BencodeFile struct {
	InfoHash    string
	DisplayName string
	Length      int
	TrackerURL  string
}

type BencodeInfo struct {
	Pieces      string
	PieceLength int
	Length      int
	Name        string
}

type BencodeTorrent struct {
	Annouce string
	Info    BencodeInfo
}




type BencodeMagnet struct {
	InfoHash    string
	DisplayName string
	Length      int
	TrackerURL  string
}

// magnet:?xt=urn:btih:DPIIR3URM2QGFT2K6COPTFZA7JXBUMJT&dn=debian-12.7.0-amd64-netinst.iso&xl=661651456&tr=http%3A%2F%2Fbttracker.debian.org%3A6969%2Fannounce
func (bm *BencodeMagnet) Decode(url string) (BencodeFile, error) {
	var file BencodeFile
	colArr := strings.Split(url, ":")
	hash := strings.Split(colArr[3], "&")[0]
	encoded, err := base32.StdEncoding.DecodeString(hash)
	if err != nil {
		return file, err
	}
	file.InfoHash = string(encoded)
	parsed, err := neturl.Parse(url)
	if err != nil {
		return file, err
	}
	file.DisplayName = parsed.Query().Get("dn")
	fileLen, err := strconv.Atoi(parsed.Query().Get("xl"))
	if err != nil {
		return file, err
	}
	file.Length = fileLen
	file.TrackerURL = parsed.Query().Get("tr")
	return file, nil
}
