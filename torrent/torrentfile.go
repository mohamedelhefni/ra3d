package torrent

import (
	"bytes"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"github.com/jackpal/bencode-go"
	neturl "net/url"
	"strconv"
	"strings"
)

type bencodeInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

func (bi *bencodeInfo) hash() (string, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *bi)
	if err != nil {
		return "", err
	}
	h := sha1.Sum(buf.Bytes())
	return string(h[:]), nil
}

func (bi *bencodeInfo) splitHashes() ([]string, error) {
	hashLen := 20 // sha1 length
	buf := []byte(bi.Pieces)
	if len(buf)%hashLen != 0 {
		err := fmt.Errorf("Received malformed pieces of length %d", len(buf))
		return nil, err
	}
	numHashes := len(buf) / hashLen
	hashes := make([]string, numHashes)
	for i := 0; i < numHashes; i++ {
		hashes[i] = string(buf[i*hashLen : (i+1)*hashLen][:])
	}
	return hashes, nil
}

type bencodeTorrent struct {
	Announce string      `bencode:"announce"`
	Info     bencodeInfo `bencode:"info"`
}

// TODO: write implementation for magnet link to download the file info
type BencodeMagnet struct {
	InfoHash    string
	DisplayName string
	Length      int
	TrackerURL  string
}

// magnet:?xt=urn:btih:DPIIR3URM2QGFT2K6COPTFZA7JXBUMJT&dn=debian-12.7.0-amd64-netinst.iso&xl=661651456&tr=http%3A%2F%2Fbttracker.debian.org%3A6969%2Fannounce
func (bm *BencodeMagnet) Decode(url string) (TorrentFile, error) {
	var file TorrentFile
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
	file.Announce = parsed.Query().Get("tr")
	return file, nil
}
