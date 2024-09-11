package torrent

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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
	fmt.Println("peers are", peers)

	torrent := Torrent{
		PeerID: string(peerId[:]),
		Peers:  peers,
		File:   tr.File,
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

func (tr *Tracker) GetPeers(peerId string, port int) ([]Peer, error) {
	var peers []Peer
	announceArr := strings.Split(tr.File.Announce, ":")
	if announceArr[0] == "udp" {
		var PeerID [20]byte
		copy(PeerID[:], []byte(peerId))
		res, err := udpTrackerRequest(tr.File.Announce, []byte(tr.File.InfoHash), PeerID, tr.File.Length)
		if err != nil {
			return nil, err
		}
		peers = res.Peers
	} else {
		trackerURL, err := tr.buildTrackingURL(peerId, port)
		if err != nil {
			return []Peer{}, err
		}
		c := &http.Client{Timeout: 15 * time.Second}
		resp, err := c.Get(trackerURL)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		trackerResp, err := CustomUnmarshalBencode(body)
		if err != nil {
			return nil, err
		}
		peers = trackerResp.Peers.([]Peer)
	}
	return peers, nil
}
