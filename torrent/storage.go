package torrent

import (
	"os"
	"path/filepath"
)

// storage maps the torrent's byte stream onto its files on disk.
type storage struct {
	files []*os.File
	tf    *TorrentFile
}

// openStorage creates (or reopens) every file at its final size. existing
// reports whether any file already had data, i.e. there may be pieces to resume.
func openStorage(dir string, tf *TorrentFile) (s *storage, existing bool, err error) {
	s = &storage{tf: tf}
	for _, f := range tf.Files {
		p := filepath.Join(dir, f.Path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			s.Close()
			return nil, false, err
		}
		fh, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			s.Close()
			return nil, false, err
		}
		s.files = append(s.files, fh)
		fi, err := fh.Stat()
		if err == nil && fi.Size() > 0 {
			existing = true
		}
		if err == nil && fi.Size() != int64(f.Length) {
			err = fh.Truncate(int64(f.Length))
		}
		if err != nil {
			s.Close()
			return nil, false, err
		}
	}
	return s, existing, nil
}

// each runs fn on every file slice covered by the torrent range [off, off+len(p)).
func (s *storage) each(p []byte, off int, fn func(*os.File, []byte, int64) (int, error)) error {
	for i, f := range s.tf.Files {
		start, end := max(off, f.Offset), min(off+len(p), f.Offset+f.Length)
		if start >= end {
			continue
		}
		if _, err := fn(s.files[i], p[start-off:end-off], int64(start-f.Offset)); err != nil {
			return err
		}
	}
	return nil
}

func (s *storage) WriteAt(p []byte, off int) error { return s.each(p, off, (*os.File).WriteAt) }
func (s *storage) ReadAt(p []byte, off int) error  { return s.each(p, off, (*os.File).ReadAt) }

func (s *storage) Close() {
	for _, f := range s.files {
		f.Close()
	}
}
