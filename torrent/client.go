package torrent

import (
	"fmt"
	"net"
	"time"
)

type Client struct {
	Conn     net.Conn
	Choked   bool
	Bitfield Bitfield
	peer     Peer
	infoHash string
	peerID   string
}

func completeHandshake(conn net.Conn, infohash, peerID string) (*Handshake, error) {
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	defer conn.SetDeadline(time.Time{}) // Disable the deadline

	req := NewHandshake(infohash, peerID)
	_, err := conn.Write(req.Serialize())
	if err != nil {
		return nil, err
	}

	res, err := ReadHandshake(conn)
	if err != nil {
		return nil, err
	}
	if res.InfoHash != infohash {
		return nil, fmt.Errorf("Expected infohash %x but got %x", res.InfoHash, infohash)
	}
	return res, nil
}

// NewClient connects with a peer, completes a handshake, and receives a handshake
// returns an err if any of those fail.
func NewClient(peer Peer, peerID, infoHash string, numPieces int) (*Client, error) {
	conn, err := net.DialTimeout("tcp", peer.String(), 3*time.Second)
	if err != nil {
		return nil, err
	}

	_, err = completeHandshake(conn, infoHash, peerID)
	if err != nil {
		conn.Close()
		return nil, err
	}

	c := &Client{
		Conn:     conn,
		Choked:   true,
		Bitfield: make(Bitfield, (numPieces+7)/8),
		peer:     peer,
		infoHash: infoHash,
		peerID:   peerID,
	}

	// Wait until we know what the peer has. Most send a bitfield, possibly
	// after an extended handshake; some send only HAVE messages.
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetDeadline(time.Time{})
	for {
		msg, err := c.Read()
		if err == nil {
			err = c.handle(msg)
		}
		if err != nil {
			conn.Close()
			return nil, err
		}
		if msg != nil && (msg.ID == MsgBitfield || msg.ID == MsgHave) {
			return c, nil
		}
	}
}

// handle applies a state-changing message (choke, have, bitfield) to the client.
func (c *Client) handle(msg *Message) error {
	if msg == nil { // keep-alive
		return nil
	}
	switch msg.ID {
	case MsgUnchoke:
		c.Choked = false
	case MsgChoke:
		c.Choked = true
	case MsgHave:
		index, err := ParseHave(msg)
		if err != nil {
			return err
		}
		c.Bitfield.SetPiece(index)
	case MsgBitfield:
		copy(c.Bitfield, msg.Payload)
	}
	return nil
}

// Read reads and consumes a message from the connection
func (c *Client) Read() (*Message, error) {
	msg, err := Read(c.Conn)
	return msg, err
}

// SendRequest sends a Request message to the peer
func (c *Client) SendRequest(index, begin, length int) error {
	req := FormatRequest(index, begin, length)
	_, err := c.Conn.Write(req.Serialize())
	return err
}

// SendInterested sends an Interested message to the peer
func (c *Client) SendInterested() error {
	msg := Message{ID: MsgInterested}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// SendNotInterested sends a NotInterested message to the peer
func (c *Client) SendNotInterested() error {
	msg := Message{ID: MsgNotInterested}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// SendUnchoke sends an Unchoke message to the peer
func (c *Client) SendUnchoke() error {
	msg := Message{ID: MsgUnchoke}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// SendHave sends a Have message to the peer
func (c *Client) SendHave(index int) error {
	msg := FormatHave(index)
	_, err := c.Conn.Write(msg.Serialize())
	return err
}
