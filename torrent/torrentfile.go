package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"

	"github.com/jackpal/bencode-go"
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
		hashes[i] = string(buf[i*hashLen : (i+1)*hashLen])
	}
	return hashes, nil
}

type bencodeTorrent struct {
	Announce string      `bencode:"announce"`
	Info     bencodeInfo `bencode:"info"`
}

type BencodeMagnet struct {
	InfoHash    string
	DisplayName string
	Length      int
	TrackerURL  string
}
