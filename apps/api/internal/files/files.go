// Package files stores conversation attachments on local disk. Files are
// named by their generated id, so a stored file never carries the original
// name on disk (that lives in the database) and cannot collide.
package files

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ErrInvalidName = errors.New("files: invalid file name")

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type Store struct {
	dir string
}

// New prepares (and creates when missing) the attachment directory.
func New(dir string) (*Store, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "data/uploads"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("files: create dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// ExtensionOf extracts a safe lowercase extension from a client-provided
// filename. Unknown or dangerous names yield an empty extension.
func ExtensionOf(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	base = strings.ReplaceAll(base, "\\", "")
	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" || len(ext) > 12 {
		return ""
	}
	if !safeExtension(ext[1:]) {
		return ""
	}
	return ext
}

func safeExtension(ext string) bool {
	for _, char := range ext {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

// Path returns the on-disk location of a stored attachment.
func (s *Store) Path(id, extension string) string {
	return filepath.Join(s.dir, id+extension)
}

// Save writes the content under the attachment id and returns the stored name
// (id plus extension) so callers can find it again.
func (s *Store) Save(id, extension string, content io.Reader) (string, error) {
	name := id + extension
	if strings.ContainsAny(id, `/\`) || len(id) > 64 {
		return "", ErrInvalidName
	}

	file, err := os.OpenFile(s.Path(id, extension), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("files: create %s: %w", name, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, content); err != nil {
		return "", fmt.Errorf("files: write %s: %w", name, err)
	}

	return name, nil
}

// Open returns a seekable reader for a stored attachment.
func (s *Store) Open(id, extension string) (io.ReadSeekCloser, error) {
	file, err := os.Open(s.Path(id, extension))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrInvalidName
		}
		return nil, fmt.Errorf("files: open %s: %w", id, err)
	}
	return file, nil
}
