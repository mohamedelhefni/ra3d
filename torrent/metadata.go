package torrent

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/jackpal/bencode-go"
)

// utMetadataID is the extended message id we ask peers to use for
// ut_metadata messages sent to us (BEP 9).
const utMetadataID = 1

const maxMetadataSize = 16 << 20

// fetchMetadata downloads the info dict for a magnet link from peers.
func (t *Torrent) fetchMetadata() error {
	const attempts = 3
	for attempt := 1; attempt <= attempts; attempt++ {
		if t.stopped() {
			return ErrStopped
		}
		t.setStatus(fmt.Sprintf("Finding peers (attempt %d/%d)", attempt, attempts))
		peers := t.announce(1) // left=0 would make trackers treat us as a seeder
		t.update(func(s *Stats) { s.KnownPeers = len(peers) })

		t.setStatus(fmt.Sprintf("Fetching metadata from %d peers", len(peers)))
		results := make(chan []byte, len(peers))
		for _, p := range peers {
			go func() {
				raw, err := fetchMetadataFrom(p, t.File.InfoHash, t.PeerID)
				if err != nil {
					raw = nil
				}
				results <- raw
			}()
		}
		for range peers {
			if raw := <-results; raw != nil {
				return t.File.setInfo(raw)
			}
		}
		if attempt == attempts {
			break
		}
		_, stop, _ := t.chans()
		select {
		case <-stop:
			return ErrStopped
		case <-time.After(10 * time.Second):
		}
	}
	return errors.New("could not fetch metadata from any peer")
}

func fetchMetadataFrom(peer Peer, infoHash, peerID string) ([]byte, error) {
	conn, err := net.DialTimeout("tcp", peer.String(), 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	hs, err := completeHandshake(conn, infoHash, peerID)
	if err != nil {
		return nil, err
	}
	if !hs.Extensions {
		return nil, errors.New("peer does not support the extension protocol")
	}
	conn.SetDeadline(time.Now().Add(20 * time.Second))
	err = sendExtended(conn, 0, map[string]interface{}{
		"m": map[string]interface{}{"ut_metadata": utMetadataID},
	})
	if err != nil {
		return nil, err
	}

	var metadata []byte
	numPieces, received := 0, 0
	for {
		msg, err := Read(conn)
		if err != nil {
			return nil, err
		}
		if msg == nil || msg.ID != MsgExtended || len(msg.Payload) == 0 {
			continue
		}
		dict, rest, err := decodeDict(msg.Payload[1:])
		if err != nil {
			return nil, err
		}

		switch msg.Payload[0] {
		case 0: // their extended handshake
			m, _ := dict["m"].(map[string]interface{})
			id, _ := m["ut_metadata"].(int64)
			size, _ := dict["metadata_size"].(int64)
			if id <= 0 || id > 255 || size <= 0 || size > maxMetadataSize {
				return nil, errors.New("peer cannot serve metadata")
			}
			metadata = make([]byte, size)
			numPieces = int((size + MaxBlockSize - 1) / MaxBlockSize)
			for i := 0; i < numPieces; i++ {
				err := sendExtended(conn, byte(id), map[string]interface{}{"msg_type": 0, "piece": i})
				if err != nil {
					return nil, err
				}
			}

		case utMetadataID:
			if metadata == nil {
				continue
			}
			msgType, _ := dict["msg_type"].(int64)
			piece, _ := dict["piece"].(int64)
			if msgType != 1 {
				return nil, errors.New("peer rejected metadata request")
			}
			off := int(piece) * MaxBlockSize
			if piece < 0 || int(piece) >= numPieces || off+len(rest) > len(metadata) {
				return nil, fmt.Errorf("bad metadata piece %d", piece)
			}
			copy(metadata[off:], rest)
			received++
			if received == numPieces {
				if h := sha1.Sum(metadata); string(h[:]) != infoHash {
					return nil, errors.New("metadata hash mismatch")
				}
				return metadata, nil
			}
		}
	}
}

func sendExtended(w io.Writer, id byte, v interface{}) error {
	var buf bytes.Buffer
	buf.WriteByte(id)
	if err := bencode.Marshal(&buf, v); err != nil {
		return err
	}
	_, err := w.Write((&Message{ID: MsgExtended, Payload: buf.Bytes()}).Serialize())
	return err
}

// decodeDict decodes a bencoded dict at the start of b and returns the bytes after it.
func decodeDict(b []byte) (map[string]interface{}, []byte, error) {
	r := bytes.NewReader(b)
	br := bufio.NewReader(r)
	v, err := bencode.Decode(br)
	if err != nil {
		return nil, nil, err
	}
	d, ok := v.(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("expected bencoded dict")
	}
	return d, b[len(b)-r.Len()-br.Buffered():], nil
}
