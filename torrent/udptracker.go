package torrent

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	connectAction     = 0
	announceAction    = 1
	errorAction       = 3
	connectionIDMagic = 0x41727101980
)

// udpTrackerRequest announces to a UDP tracker (BEP 15) and returns its peers.
func udpTrackerRequest(host, infoHash, peerID string, left int) ([]Peer, error) {
	conn, err := net.Dial("udp", host)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	connReq := make([]byte, 16)
	binary.BigEndian.PutUint64(connReq[0:8], connectionIDMagic)
	binary.BigEndian.PutUint32(connReq[8:12], connectAction)
	resp, err := udpRoundTrip(conn, connReq, 16)
	if err != nil {
		return nil, err
	}
	connectionID := binary.BigEndian.Uint64(resp[8:16])

	req := make([]byte, 98)
	binary.BigEndian.PutUint64(req[0:8], connectionID)
	binary.BigEndian.PutUint32(req[8:12], announceAction)
	copy(req[16:36], infoHash)
	copy(req[36:56], peerID)
	binary.BigEndian.PutUint64(req[56:64], 0)            // downloaded
	binary.BigEndian.PutUint64(req[64:72], uint64(left)) // left
	binary.BigEndian.PutUint64(req[72:80], 0)            // uploaded
	binary.BigEndian.PutUint32(req[80:84], 0)            // event
	binary.BigEndian.PutUint32(req[84:88], 0)            // IP address
	binary.BigEndian.PutUint32(req[88:92], randUint32()) // key
	binary.BigEndian.PutUint32(req[92:96], 200)          // num_want
	binary.BigEndian.PutUint16(req[96:98], listenPort)
	resp, err = udpRoundTrip(conn, req, 20)
	if err != nil {
		return nil, err
	}
	return parseCompactPeers(resp[20:]), nil
}

// udpRoundTrip fills in a fresh transaction id, sends req and returns a reply
// of at least minLen bytes with the matching action and transaction id.
// UDP is lossy, so the request is retried a few times.
func udpRoundTrip(conn net.Conn, req []byte, minLen int) ([]byte, error) {
	action := binary.BigEndian.Uint32(req[8:12])
	txID := randUint32()
	binary.BigEndian.PutUint32(req[12:16], txID)
	buf := make([]byte, 4096)

	var err error
	for try := 0; try < 3; try++ {
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err = conn.Write(req); err != nil {
			return nil, err
		}
		var n int
		n, err = conn.Read(buf)
		if err != nil {
			continue // timeout: resend
		}
		if n < 8 || binary.BigEndian.Uint32(buf[4:8]) != txID {
			err = errors.New("transaction ID mismatch")
			continue
		}
		switch binary.BigEndian.Uint32(buf[0:4]) {
		case action:
			if n < minLen {
				return nil, fmt.Errorf("tracker response too short: %d bytes", n)
			}
			return buf[:n], nil
		case errorAction:
			return nil, fmt.Errorf("tracker: %s", buf[8:n])
		default:
			return nil, errors.New("unexpected tracker action")
		}
	}
	return nil, err
}

func randUint32() uint32 {
	var b [4]byte
	rand.Read(b[:])
	return binary.BigEndian.Uint32(b[:])
}
