package torrent

import (
	"bytes"
	"crypto/sha1"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jackpal/bencode-go"
)

// File is one file inside a torrent, laid out back to back with the others.
type File struct {
	Path   string // relative to the download directory
	Length int
	Offset int // where the file starts in the torrent's byte stream
}

type TorrentFile struct {
	Trackers    []string
	DisplayName string
	Length      int
	InfoHash    string
	PieceHashes []string
	PieceLength int
	Files       []File
}

type bencodeFile struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

type bencodeInfo struct {
	Pieces      string        `bencode:"pieces"`
	PieceLength int           `bencode:"piece length"`
	Length      int           `bencode:"length"`
	Name        string        `bencode:"name"`
	Files       []bencodeFile `bencode:"files"`
}

// Open parses a .torrent file.
func (tf *TorrentFile) Open(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := bencode.Decode(file)
	if err != nil {
		return err
	}
	dict, ok := data.(map[string]interface{})
	if !ok {
		return errors.New("not a torrent file")
	}
	info, ok := dict["info"].(map[string]interface{})
	if !ok {
		return errors.New("torrent file has no info dict")
	}
	// Re-encode the generic dict (keys sorted, unknown keys kept) so the
	// info hash matches the original bytes.
	var raw bytes.Buffer
	if err := bencode.Marshal(&raw, info); err != nil {
		return err
	}
	if err := tf.setInfo(raw.Bytes()); err != nil {
		return err
	}

	if a, ok := dict["announce"].(string); ok {
		tf.Trackers = append(tf.Trackers, a)
	}
	tiers, _ := dict["announce-list"].([]interface{})
	for _, tier := range tiers {
		urls, _ := tier.([]interface{})
		for _, u := range urls {
			if s, ok := u.(string); ok && !slices.Contains(tf.Trackers, s) {
				tf.Trackers = append(tf.Trackers, s)
			}
		}
	}
	return nil
}

// setInfo fills the torrent from a raw bencoded info dict.
func (tf *TorrentFile) setInfo(raw []byte) error {
	var info bencodeInfo
	if err := bencode.Unmarshal(bytes.NewReader(raw), &info); err != nil {
		return err
	}
	if len(info.Pieces)%20 != 0 {
		return fmt.Errorf("received malformed pieces of length %d", len(info.Pieces))
	}
	if info.PieceLength <= 0 {
		return fmt.Errorf("invalid piece length %d", info.PieceLength)
	}

	h := sha1.Sum(raw)
	tf.InfoHash = string(h[:])
	tf.DisplayName = info.Name
	tf.PieceLength = info.PieceLength
	tf.PieceHashes = nil
	for i := 0; i < len(info.Pieces); i += 20 {
		tf.PieceHashes = append(tf.PieceHashes, info.Pieces[i:i+20])
	}

	tf.Files, tf.Length = nil, 0
	if len(info.Files) == 0 {
		info.Files = []bencodeFile{{Length: info.Length}}
	}
	for _, f := range info.Files {
		p, err := safePath(append([]string{info.Name}, f.Path...))
		if err != nil {
			return err
		}
		if f.Length < 0 {
			return fmt.Errorf("invalid length for %s", p)
		}
		tf.Files = append(tf.Files, File{Path: p, Length: f.Length, Offset: tf.Length})
		tf.Length += f.Length
	}

	if want := (tf.Length + tf.PieceLength - 1) / tf.PieceLength; len(tf.PieceHashes) != want {
		return fmt.Errorf("torrent has %d piece hashes, expected %d", len(tf.PieceHashes), want)
	}
	return nil
}

// safePath joins path components from an untrusted torrent, refusing
// anything that could escape the download directory.
func safePath(parts []string) (string, error) {
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, "/\\\x00") {
			return "", fmt.Errorf("unsafe path component %q in torrent", p)
		}
	}
	return filepath.Join(parts...), nil
}

// ParseMagnet reads the info hash, name and trackers from a magnet link.
// The rest of the torrent comes from peers, see fetchMetadata.
func ParseMagnet(uri string) (TorrentFile, error) {
	var tf TorrentFile
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "magnet" {
		return tf, fmt.Errorf("invalid magnet link")
	}
	q := u.Query()
	for _, xt := range q["xt"] {
		h, ok := strings.CutPrefix(xt, "urn:btih:")
		if !ok {
			continue
		}
		var b []byte
		switch len(h) {
		case 40:
			b, err = hex.DecodeString(h)
		case 32:
			b, err = base32.StdEncoding.DecodeString(strings.ToUpper(h))
		default:
			err = fmt.Errorf("bad info hash length %d", len(h))
		}
		if err != nil {
			return tf, fmt.Errorf("invalid magnet info hash: %w", err)
		}
		tf.InfoHash = string(b)
	}
	if tf.InfoHash == "" {
		return tf, errors.New("magnet link has no btih info hash")
	}
	tf.DisplayName = q.Get("dn")
	tf.Trackers = q["tr"]
	if len(tf.Trackers) == 0 {
		return tf, errors.New("magnet link has no trackers (DHT is not supported)")
	}
	return tf, nil
}
