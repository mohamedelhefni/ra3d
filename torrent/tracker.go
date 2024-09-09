package torrent

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Tracker struct {
	Type string
	File BencodeFile
}

type Peer struct {
	IP   net.IP
	Port uint16
}

func (tr *Tracker) DownloadToFile() error {
	var peerId [20]byte
	_, err := rand.Read(peerId[:])
	if err != nil {
		return err
	}
	_, err = tr.GetPeers(string(peerId[:]), 6881)
	if err != nil {
		return err
	}

	return nil
}

func (tr *Tracker) buildTrackingURL(peerId string, port int) (string, error) {
	base, err := url.Parse(tr.File.TrackerURL)
	if err != nil {
		return "", err
	}
  fmt.Println("info hash", tr.File.InfoHash)
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
	trackerURL, err := tr.buildTrackingURL(peerId, port)
	if err != nil {
		return []Peer{}, err
	}

	fmt.Println("tracker url", trackerURL)
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(trackerURL)
	if err != nil {
		return nil, err
	}
	// defer resp.Body.Close()
	fmt.Println("status is", resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []Peer{}, err
	}
	//Convert the body to type string
	sb := string(body)
  fmt.Println("body is", sb)


	return []Peer{}, nil
}
