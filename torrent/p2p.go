package torrent

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// MaxBlockSize is the largest number of bytes a request can ask for
const MaxBlockSize = 16384

// MaxBacklog is the number of unfulfilled requests a client can have in its pipeline
const MaxBacklog = 5

// reannounceEvery is how often trackers are asked for fresh peers.
const reannounceEvery = time.Minute

// Stats is a snapshot of a download's progress.
type Stats struct {
	Name       string
	Status     string
	Length     int
	Downloaded int // bytes in verified pieces
	Fetched    int // bytes downloaded from peers this run
	Pieces     Bitfield
	NumPieces  int
	DonePieces int
	Peers      int // connected peers
	KnownPeers int
	Done       bool
	Err        error
}

// Torrent downloads one torrent. Call Run, and poll Stats from elsewhere.
type Torrent struct {
	PeerID string
	File   TorrentFile

	mu     sync.Mutex
	stats  Stats
	store  *storage
	active map[string]bool // peers with a running worker
}

type pieceWork struct {
	index  int
	hash   string
	length int
}

type pieceResult struct {
	index int
	err   error
}

type pieceProgress struct {
	index      int
	buf        []byte
	downloaded int
	requested  int
	backlog    int
	client     *Client
}

func (state *pieceProgress) readMessage() error {
	msg, err := state.client.Read() // this call blocks
	if err != nil {
		return err
	}
	if msg != nil && msg.ID == MsgPiece {
		n, err := ParsePiece(state.index, state.buf, msg)
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
		return nil
	}
	return state.client.handle(msg)
}

func attemptDownloadPiece(c *Client, pw *pieceWork) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}

	// Setting a deadline helps get unresponsive peers unstuck.
	// 30 seconds is more than enough time to download a 262 KB piece
	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{}) // Disable the deadline

	for state.downloaded < pw.length {
		// If unchoked, send requests until we have enough unfulfilled requests
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := MaxBlockSize
				// Last block might be shorter than the typical block
				if pw.length-state.requested < blockSize {
					blockSize = pw.length - state.requested
				}

				err := c.SendRequest(pw.index, state.requested, blockSize)
				if err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}

		err := state.readMessage()
		if err != nil {
			return nil, err
		}
	}
	return state.buf, nil
}

func checkIntegrity(pw *pieceWork, buf []byte) error {
	hash := sha1.Sum(buf)
	if string(hash[:]) != pw.hash {
		return fmt.Errorf("Index %d failed integrity check", pw.index)
	}
	return nil
}

func (t *Torrent) startDownloadWorker(peer Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	c, err := NewClient(peer, t.PeerID, t.File.InfoHash, len(t.File.PieceHashes))
	if err != nil {
		return
	}
	defer c.Conn.Close()
	t.update(func(s *Stats) { s.Peers++ })
	defer t.update(func(s *Stats) { s.Peers-- })

	c.SendUnchoke()
	c.SendInterested()

	skipped := 0
	for pw := range workQueue {
		if !c.Bitfield.HasPiece(pw.index) {
			workQueue <- pw // Put piece back on the queue
			// Only pieces this peer lacks are left; don't spin on them.
			if skipped++; skipped > len(workQueue) {
				time.Sleep(time.Second)
				skipped = 0
			}
			continue
		}
		skipped = 0

		buf, err := attemptDownloadPiece(c, pw)
		if err != nil {
			workQueue <- pw // Put piece back on the queue
			return
		}
		if checkIntegrity(pw, buf) != nil {
			workQueue <- pw // Put piece back on the queue
			continue
		}

		begin, _ := t.calculateBoundsForPiece(pw.index)
		err = t.store.WriteAt(buf, begin)
		c.SendHave(pw.index)
		results <- &pieceResult{pw.index, err}
	}
}

func (t *Torrent) calculateBoundsForPiece(index int) (begin int, end int) {
	begin = index * t.File.PieceLength
	end = begin + t.File.PieceLength
	if end > t.File.Length {
		end = t.File.Length
	}
	return begin, end
}

func (t *Torrent) calculatePieceSize(index int) int {
	begin, end := t.calculateBoundsForPiece(index)
	return end - begin
}

// Stats returns a snapshot of the download's progress. Safe to call concurrently with Run.
func (t *Torrent) Stats() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.stats
	s.Pieces = slices.Clone(s.Pieces)
	return s
}

func (t *Torrent) update(fn func(*Stats)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fn(&t.stats)
}

func (t *Torrent) setStatus(status string) {
	t.update(func(s *Stats) { s.Status = status })
}

func (t *Torrent) pieceDone(index int) {
	size := t.calculatePieceSize(index)
	t.update(func(s *Stats) {
		s.Pieces.SetPiece(index)
		s.DonePieces++
		s.Downloaded += size
	})
}

// Run downloads source (a .torrent path or magnet link) into outDir.
// Data already on disk is verified and kept, so an interrupted download resumes.
func (t *Torrent) Run(source, outDir string) error {
	err := t.run(source, outDir)
	t.update(func(s *Stats) {
		s.Err = err
		s.Done = err == nil
		if s.Done {
			s.Status = "Done"
		}
	})
	return err
}

func (t *Torrent) run(source, outDir string) error {
	id := make([]byte, 12)
	rand.Read(id)
	t.PeerID = "-RA0001-" + string(id)

	if strings.HasPrefix(source, "magnet:") {
		tf, err := ParseMagnet(source)
		if err != nil {
			return err
		}
		t.File = tf
		t.update(func(s *Stats) { s.Name = tf.DisplayName })
		if err := t.fetchMetadata(); err != nil {
			return err
		}
	} else if err := t.File.Open(source); err != nil {
		return err
	}

	n := len(t.File.PieceHashes)
	t.update(func(s *Stats) {
		s.Name = t.File.DisplayName
		s.Length = t.File.Length
		s.NumPieces = n
		s.Pieces = make(Bitfield, (n+7)/8)
	})

	store, existing, err := openStorage(outDir, &t.File)
	if err != nil {
		return err
	}
	defer store.Close()
	t.store = store

	var work []*pieceWork
	buf := make([]byte, t.File.PieceLength)
	for i, hash := range t.File.PieceHashes {
		pw := &pieceWork{i, hash, t.calculatePieceSize(i)}
		if existing {
			if i%64 == 0 {
				t.setStatus(fmt.Sprintf("Checking existing data %d%%", i*100/n))
			}
			begin, _ := t.calculateBoundsForPiece(i)
			if err := store.ReadAt(buf[:pw.length], begin); err != nil {
				return err
			}
			if checkIntegrity(pw, buf[:pw.length]) == nil {
				t.pieceDone(i)
				continue
			}
		}
		work = append(work, pw)
	}
	if len(work) == 0 {
		return nil
	}
	return t.download(work)
}

func (t *Torrent) download(work []*pieceWork) error {
	workQueue := make(chan *pieceWork, len(work))
	for _, pw := range work {
		workQueue <- pw
	}
	results := make(chan *pieceResult)
	peersCh := make(chan []Peer)
	t.active = map[string]bool{}

	announce := time.NewTimer(0)
	for remaining := len(work); remaining > 0; {
		select {
		case <-announce.C:
			t.setStatus("Asking trackers for peers")
			left := t.File.Length - t.Stats().Downloaded
			go func() { peersCh <- t.announce(left) }()

		case peers := <-peersCh:
			t.update(func(s *Stats) { s.KnownPeers = len(peers) })
			t.setStatus("Downloading")
			if len(peers) == 0 {
				t.setStatus("No peers found, retrying")
			}
			for _, peer := range peers {
				t.startWorker(peer, workQueue, results)
			}
			announce.Reset(reannounceEvery)

		case res := <-results:
			if res.err != nil {
				return fmt.Errorf("writing piece %d: %w", res.index, res.err)
			}
			t.pieceDone(res.index)
			t.update(func(s *Stats) { s.Fetched += t.calculatePieceSize(res.index) })
			remaining--
		}
	}
	close(workQueue)
	return nil
}

// startWorker starts a download worker for peer unless one is already running.
func (t *Torrent) startWorker(peer Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	key := peer.String()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active[key] {
		return
	}
	t.active[key] = true
	go func() {
		t.startDownloadWorker(peer, workQueue, results)
		t.mu.Lock()
		delete(t.active, key) // allow a reconnect on the next announce
		t.mu.Unlock()
	}()
}
