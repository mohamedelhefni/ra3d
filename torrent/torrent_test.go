package torrent

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackpal/bencode-go"
)

func encodeInfo(t *testing.T, info map[string]interface{}) []byte {
	var buf bytes.Buffer
	if err := bencode.Marshal(&buf, info); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestMultiFileStorageAndResume(t *testing.T) {
	data := []byte("hello world, this spans files!") // 30 bytes, pieces of 8
	var pieces string
	for i := 0; i < len(data); i += 8 {
		h := sha1.Sum(data[i:min(i+8, len(data))])
		pieces += string(h[:])
	}
	raw := encodeInfo(t, map[string]interface{}{
		"name": "dir", "piece length": 8, "pieces": pieces,
		"files": []interface{}{
			map[string]interface{}{"length": 5, "path": []interface{}{"a.txt"}},
			map[string]interface{}{"length": 0, "path": []interface{}{"empty"}},
			map[string]interface{}{"length": 25, "path": []interface{}{"sub", "b.txt"}},
		},
	})
	var tf TorrentFile
	if err := tf.setInfo(raw); err != nil {
		t.Fatal(err)
	}
	if tf.Length != 30 || len(tf.Files) != 3 || tf.Files[2].Offset != 5 {
		t.Fatalf("bad layout: %+v", tf)
	}

	dir := t.TempDir()
	s, existing, err := openStorage(dir, &tf)
	if err != nil || existing {
		t.Fatal(err, existing)
	}
	if err := s.WriteAt(data[3:20], 3); err != nil { // crosses a.txt -> b.txt
		t.Fatal(err)
	}
	s.WriteAt(data[:3], 0)
	s.WriteAt(data[20:], 20)
	got := make([]byte, 30)
	if err := s.ReadAt(got, 0); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("read back %q, %v", got, err)
	}
	s.Close()
	if b, _ := os.ReadFile(filepath.Join(dir, "dir", "sub", "b.txt")); string(b) != string(data[5:]) {
		t.Fatalf("b.txt = %q", b)
	}

	// Reopening finds the data and every piece verifies, so nothing is left to download.
	tr := &Torrent{File: tf}
	if err := tr.Run(writeTorrent(t, raw), dir); err != nil {
		t.Fatal(err)
	}
	if st := tr.Stats(); !st.Done || st.DonePieces != 4 {
		t.Fatalf("resume stats: %+v", st)
	}
}

func writeTorrent(t *testing.T, info []byte) string {
	p := filepath.Join(t.TempDir(), "x.torrent")
	body := append([]byte("d8:announce14:http://x/annou4:info"), info...)
	os.WriteFile(p, append(body, 'e'), 0o644)
	return p
}

func TestUnsafePathsRejected(t *testing.T) {
	for _, name := range []string{"..", "a/b", ""} {
		raw := encodeInfo(t, map[string]interface{}{"name": name, "piece length": 1, "pieces": "", "length": 0})
		var tf TorrentFile
		if err := tf.setInfo(raw); err == nil {
			t.Errorf("name %q accepted", name)
		}
	}
}

func TestParseMagnet(t *testing.T) {
	tf, err := ParseMagnet("magnet:?xt=urn:btih:DPIIR3URM2QGFT2K6COPTFZA7JXBUMJT&dn=x.iso&tr=udp%3A%2F%2Fa%3A1&tr=http%3A%2F%2Fb%2Fannounce")
	if err != nil || len(tf.InfoHash) != 20 || tf.DisplayName != "x.iso" || len(tf.Trackers) != 2 {
		t.Fatalf("%+v %v", tf, err)
	}
	hex, _ := ParseMagnet("magnet:?xt=urn:btih:1bd088ee9166a062cf4af09cf99720fa6e1a3133&tr=udp%3A%2F%2Fa%3A1")
	if hex.InfoHash != tf.InfoHash {
		t.Fatal("hex and base32 hashes differ")
	}
}

func TestDecodeDictTrailingData(t *testing.T) {
	d, rest, err := decodeDict([]byte("d8:msg_typei1e5:piecei0eeRAWDATA"))
	if err != nil || d["msg_type"] != int64(1) || string(rest) != "RAWDATA" {
		t.Fatalf("%v %q %v", d, rest, err)
	}
}

func TestPauseResume(t *testing.T) {
	tr := &Torrent{File: TorrentFile{Length: 8, PieceLength: 8, PieceHashes: []string{"x"}}}
	go tr.download([]*pieceWork{{0, "x", 8}})
	waitStatus := func(want func(string) bool) {
		for range 100 {
			if want(tr.Stats().Status) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("status stuck at %q", tr.Stats().Status)
	}
	tr.TogglePause()
	waitStatus(func(s string) bool { return s == "Paused" })
	tr.TogglePause()
	waitStatus(func(s string) bool { return s != "Paused" })
	if tr.Stats().Paused {
		t.Fatal("still paused")
	}
}

func TestStopAndDelete(t *testing.T) {
	data := []byte("0123456789")
	h := sha1.Sum(data)
	info := map[string]interface{}{
		"name": "dir", "piece length": 16, "pieces": string(h[:]),
		"files": []interface{}{
			map[string]interface{}{"length": 10, "path": []interface{}{"sub", "a.txt"}},
		},
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "x.torrent")
	var buf bytes.Buffer
	bencode.Marshal(&buf, map[string]interface{}{"announce": "http://127.0.0.1:1/announce", "info": info})
	os.WriteFile(src, buf.Bytes(), 0o644)

	tr := &Torrent{}
	out := filepath.Join(dir, "out")
	errc := make(chan error)
	go func() { errc <- tr.Run(src, out) }()
	for tr.Stats().NumPieces == 0 && tr.Stats().Err == nil {
		time.Sleep(10 * time.Millisecond)
	}
	if err := tr.Delete(); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != ErrStopped {
		t.Fatalf("Run returned %v, want ErrStopped", err)
	}
	if _, err := os.Stat(filepath.Join(out, "dir")); !os.IsNotExist(err) {
		t.Fatalf("torrent folder still there: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output dir itself was removed: %v", err)
	}
}
