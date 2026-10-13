package torrent

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const listenPort = 6881

var httpClient = &http.Client{Timeout: 15 * time.Second}

// announce asks every tracker for peers concurrently and returns the union.
func (t *Torrent) announce(left int) []Peer {
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		seen  = map[string]bool{}
		peers []Peer
	)
	for _, tr := range t.File.Trackers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ps, err := getPeers(tr, t.File.InfoHash, t.PeerID, left)
			if err != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, p := range ps {
				if !seen[p.String()] {
					seen[p.String()] = true
					peers = append(peers, p)
				}
			}
		}()
	}
	wg.Wait()
	return peers
}

func getPeers(announce, infoHash, peerID string, left int) ([]Peer, error) {
	u, err := url.Parse(announce)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "udp":
		return udpTrackerRequest(u.Host, infoHash, peerID, left)
	case "http", "https":
		q := u.Query()
		q.Set("peer_id", peerID)
		q.Set("info_hash", infoHash)
		q.Set("port", strconv.Itoa(listenPort))
		q.Set("uploaded", "0")
		q.Set("downloaded", "0")
		q.Set("compact", "1")
		q.Set("left", strconv.Itoa(left))
		u.RawQuery = q.Encode()

		resp, err := httpClient.Get(u.String())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, err
		}
		return parseTrackerResponse(body)
	}
	return nil, fmt.Errorf("unsupported tracker %q", announce)
}
