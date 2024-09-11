package torrent

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"
)

const (
	connectAction     = 0
	announceAction    = 1
	connectionIDMagic = 0x41727101980
)

type UDPTrackerResponse struct {
	Action        uint32
	TransactionID uint32
	ConnectionID  uint64
	Interval      uint32
	Leechers      uint32
	Seeders       uint32
	Peers         []Peer
}

func udpTrackerRequest(announceURL string, infoHash []byte, peerID [20]byte, length int) (*UDPTrackerResponse, error) {
	u, err := url.Parse(announceURL)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("udp", u.Host)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	connectionID, err := udpConnect(conn)
	if err != nil {
		return nil, err
	}

	return udpAnnounce(conn, connectionID, infoHash, peerID, length)
}

func udpConnect(conn net.Conn) (uint64, error) {
	transactionID := genTransactionID()

	connectReq := make([]byte, 16)
	binary.BigEndian.PutUint64(connectReq[0:8], connectionIDMagic)
	binary.BigEndian.PutUint32(connectReq[8:12], connectAction)
	binary.BigEndian.PutUint32(connectReq[12:16], transactionID)

	conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, err := conn.Write(connectReq)
	if err != nil {
		return 0, err
	}

	connectResp := make([]byte, 16)
	_, err = conn.Read(connectResp)
	if err != nil {
		return 0, err
	}
	// fmt.Println("resp ", connectResp)
	conn.SetDeadline(time.Time{})
	if binary.BigEndian.Uint32(connectResp[0:4]) != 0 {
		return 0, errors.New("connect response action is not 0")
	}

	if binary.BigEndian.Uint32(connectResp[4:8]) != transactionID {
		return 0, errors.New("transaction ID mismatch")
	}

	return binary.BigEndian.Uint64(connectResp[8:16]), nil
}

func udpAnnounce(conn net.Conn, connectionID uint64, infoHash []byte, peerID [20]byte, totalLength int) (*UDPTrackerResponse, error) {
	transactionID := genTransactionID()

	// fmt.Println("start udp announce")
	announceReq := make([]byte, 98)
	binary.BigEndian.PutUint64(announceReq[0:8], connectionID)
	binary.BigEndian.PutUint32(announceReq[8:12], announceAction)
	binary.BigEndian.PutUint32(announceReq[12:16], transactionID)
	copy(announceReq[16:36], infoHash)
	copy(announceReq[36:56], peerID[:])
	binary.BigEndian.PutUint64(announceReq[56:64], 0)                   // downloaded
	binary.BigEndian.PutUint64(announceReq[64:72], uint64(totalLength)) // left
	binary.BigEndian.PutUint64(announceReq[72:80], 0)                   // uploaded
	binary.BigEndian.PutUint32(announceReq[80:84], 0)                   // event
	binary.BigEndian.PutUint32(announceReq[84:88], 0)                   // IP address
	binary.BigEndian.PutUint32(announceReq[88:92], genKey())
	binary.BigEndian.PutUint32(announceReq[92:96], 50)   // num_want
	binary.BigEndian.PutUint16(announceReq[96:98], 6881) // port

	conn.SetDeadline(time.Now().Add(15 * time.Second))
	_, err := conn.Write(announceReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send announce request: %v", err)
	}
	// fmt.Println("announceReq sent:", announceReq)

	// Read the full response
	respBuffer := make([]byte, 1024)
	n, err := conn.Read(respBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read announce response: %v", err)
	}
	fmt.Printf("Received %d bytes in response\n", n)

	if n < 20 {
		return nil, fmt.Errorf("announce response too short: %d bytes", n)
	}

	announceResp := respBuffer[:20]
	// fmt.Println("announceResp:", announceResp)

	if binary.BigEndian.Uint32(announceResp[0:4]) != 1 {
		return nil, errors.New("announce response action is not 1")
	}

	if binary.BigEndian.Uint32(announceResp[4:8]) != transactionID {
		return nil, errors.New("transaction ID mismatch")
	}

	resp := &UDPTrackerResponse{
		Action:        binary.BigEndian.Uint32(announceResp[0:4]),
		TransactionID: binary.BigEndian.Uint32(announceResp[4:8]),
		Interval:      binary.BigEndian.Uint32(announceResp[8:12]),
		Leechers:      binary.BigEndian.Uint32(announceResp[12:16]),
		Seeders:       binary.BigEndian.Uint32(announceResp[16:20]),
	}
	fmt.Printf("UDP response: Action=%d, TransactionID=%d, Interval=%d, Leechers=%d, Seeders=%d\n",
		resp.Action, resp.TransactionID, resp.Interval, resp.Leechers, resp.Seeders)

	// Parse peer information
	peerDataLen := n - 20
	if peerDataLen%6 != 0 {
		return nil, fmt.Errorf("peer data length (%d) is not a multiple of 6", peerDataLen)
	}

	numPeers := peerDataLen / 6
	fmt.Printf("Number of peers: %d\n", numPeers)

	for i := 0; i < numPeers; i++ {
		peerData := respBuffer[20+i*6 : 26+i*6]
		peer := Peer{
			PeerID: "",
			IP:     net.IP(peerData[0:4]).String(),
			Port:   binary.BigEndian.Uint16(peerData[4:6]),
		}
		resp.Peers = append(resp.Peers, peer)
		fmt.Printf("Peer %d: %s:%d\n", i+1, peer.IP, peer.Port)
	}

	return resp, nil
}

func genTransactionID() uint32 {
	return uint32(time.Now().UnixNano())
}

func genKey() uint32 {
	buf := make([]byte, 4)
	rand.Read(buf)
	return binary.BigEndian.Uint32(buf)
}
