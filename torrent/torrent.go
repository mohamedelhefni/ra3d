package torrent

import (
	"os"
	"ra3d/tui"

	"github.com/jackpal/bencode-go"
)

type Bencode interface {
	Decode(data string) (TorrentFile, error)
}

type bencodeTrackerResp struct {
	Interval int         `bencode:"interval"`
	Peers    interface{} `bencode:"peers"`
}

type PeerResp struct {
	IP     string `bencode:"ip"`
	PeerID string `bencode:"peer id"`
	Port   int    `bencode:"port"`
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

func (tf *TorrentFile) DownloadToFile(tuiChan chan tui.TorrentTui) error {
	tracker := Tracker{
		Type: "file",
		File: *tf,
	}
	buf, err := tracker.DownloadToFile(tuiChan)
	if err != nil {
		return err
	}

	outFile, err := os.Create(tf.DisplayName)
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
