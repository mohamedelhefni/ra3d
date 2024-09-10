package torrent

import (
	"encoding/base32"
	neturl "net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jackpal/bencode-go"
)

type Bencode interface {
	Decode(data string) (TorrentFile, error)
}

type bencodeTrackerResp struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

type TorrentFile struct {
	Announce    string
	DisplayName string
	Length      int
	InfoHash    string
	PieceHashes []string
	PieceLength int
}

func (tf *TorrentFile) Open(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	var bencodedTorrent bencodeTorrent
	err = bencode.Unmarshal(file, &bencodedTorrent)
	if err != nil {
		return err
	}

	tf.Announce = bencodedTorrent.Announce
	tf.DisplayName = bencodedTorrent.Info.Name
	tf.Length = bencodedTorrent.Info.Length
	hash, err := bencodedTorrent.Info.hash()
	if err != nil {
		return err
	}
	tf.InfoHash = hash
	hashes, err := bencodedTorrent.Info.splitHashes()
	if err != nil {
		return err
	}
	tf.PieceHashes = hashes
	tf.PieceLength = bencodedTorrent.Info.PieceLength
	return nil
}

func (tf *TorrentFile) DownloadToFile(outpath string) error {
	tracker := Tracker{
		Type: "file",
		File: *tf,
	}
	buf, err := tracker.DownloadToFile()
	if err != nil {
		return err
	}

	outFile, err := os.Create(outpath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = outFile.Write(buf)
	if err != nil {
		return err
	}
	return nil
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
