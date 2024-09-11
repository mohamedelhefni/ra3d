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

// CustomUnmarshalBencode implements custom unmarshalling for TrackerResponse
func CustomUnmarshalBencode(data []byte) (*bencodeTrackerResp, error) {
	reader := bytes.NewReader(data)
	dict, err := bencode.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bencode: %v", err)
	}

	response := &bencodeTrackerResp{}

	// Extract interval
	if interval, ok := dict.(map[string]interface{})["interval"].(int64); ok {
		response.Interval = int(interval)
	}

	// Extract peers
	peersData, ok := dict.(map[string]interface{})["peers"]
	if !ok {
		return nil, fmt.Errorf("peers data not found in response")
	}

	switch peers := peersData.(type) {
	case string:
		// Compact response
		response.Peers = parseCompactPeers([]byte(peers))
	case []interface{}:
		// Non-compact response
		response.Peers = parseNonCompactPeers(peers)
	default:
		return nil, fmt.Errorf("unexpected peers format")
	}

	return response, nil
}

// Function to parse compact peers
func parseCompactPeers(data []byte) []Peer {
	var peers []Peer
	for i := 0; i < len(data); i += 6 {
		ip := net.IP(data[i : i+4])
		port := binary.BigEndian.Uint16(data[i+4 : i+6])
		peers = append(peers, Peer{
			IP:   ip.String(),
			Port: port,
		})
	}
	return peers
}

// Function to parse non-compact peers
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

func UnmarshalPeers(peersBin []byte) ([]Peer, error) {
	const peerSize = 6 // 4 for IP, 2 for port
	numPeers := len(peersBin) / peerSize
	if len(peersBin)%peerSize != 0 {
		err := fmt.Errorf("Received malformed peers")
		return nil, err
	}
	peers := make([]Peer, numPeers)
	for i := 0; i < numPeers; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset+4]).String()
		peers[i].Port = binary.BigEndian.Uint16([]byte(peersBin[offset+4 : offset+6]))
	}
	return peers, nil
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP, strconv.Itoa(int(p.Port)))
}
