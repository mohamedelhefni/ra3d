package torrent

import (
	"crypto/rand"
	"net/http"
	"net/url"
	"ra3d/peers"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
)

type Tracker struct {
	Type string
	File TorrentFile
}

func (tr *Tracker) DownloadToFile() ([]byte, error) {
	var peerId [20]byte
	_, err := rand.Read(peerId[:])
	if err != nil {
		return nil, err
	}
	peers, err := tr.GetPeers(string(peerId[:]), 6881)
	if err != nil {
		return nil, err
	}

	torrent := Torrent{
		Peers: peers,
		File:  tr.File,
	}

	return torrent.Download()

}

func (tr *Tracker) buildTrackingURL(peerId string, port int) (string, error) {
	base, err := url.Parse(tr.File.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"peer_id":    []string{peerId},
		"info_hash":  []string{tr.File.InfoHash},
		"port":       []string{strconv.Itoa(port)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(tr.File.Length)},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}

func (tr *Tracker) GetPeers(peerId string, port int) ([]peers.Peer, error) {
	trackerURL, err := tr.buildTrackingURL(peerId, port)
	if err != nil {
		return []peers.Peer{}, err
	}

	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(trackerURL)
	if err != nil {
		return nil, err
	}

	trackerResp := bencodeTrackerResp{}
	err = bencode.Unmarshal(resp.Body, &trackerResp)
	if err != nil {
		return nil, err
	}

	return peers.Unmarshal([]byte(trackerResp.Peers))
}
