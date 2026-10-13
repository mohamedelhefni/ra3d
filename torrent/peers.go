package torrent

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"

	"github.com/jackpal/bencode-go"
)

type Peer struct {
	PeerID string
	IP     string
	Port   uint16
}

// parseTrackerResponse extracts peers from an HTTP tracker's bencoded reply.
func parseTrackerResponse(data []byte) ([]Peer, error) {
	decoded, err := bencode.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode bencode: %v", err)
	}
	dict, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected tracker response")
	}
	if reason, ok := dict["failure reason"].(string); ok {
		return nil, fmt.Errorf("tracker: %s", reason)
	}

	switch peers := dict["peers"].(type) {
	case string: // compact
		return parseCompactPeers([]byte(peers)), nil
	case []interface{}:
		return parseNonCompactPeers(peers), nil
	default:
		return nil, fmt.Errorf("peers data not found in response")
	}
}

// parseCompactPeers parses 6-byte (4 IP, 2 port) peer entries.
func parseCompactPeers(data []byte) []Peer {
	var peers []Peer
	for i := 0; i+6 <= len(data); i += 6 {
		peers = append(peers, Peer{
			IP:   net.IP(data[i : i+4]).String(),
			Port: binary.BigEndian.Uint16(data[i+4 : i+6]),
		})
	}
	return peers
}

func parseNonCompactPeers(data []interface{}) []Peer {
	var peers []Peer
	for _, peerData := range data {
		if peerMap, ok := peerData.(map[string]interface{}); ok {
			peer := Peer{}
			if ip, ok := peerMap["ip"].(string); ok {
				peer.IP = ip
			}
			if port, ok := peerMap["port"].(int64); ok {
				peer.Port = uint16(port)
			}
			if peerID, ok := peerMap["peer id"].(string); ok {
				peer.PeerID = peerID
			}
			peers = append(peers, peer)
		}
	}
	return peers
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP, strconv.Itoa(int(p.Port)))
}
