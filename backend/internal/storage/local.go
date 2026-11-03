package storage

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Originals are private and never registered as a static HTTP directory.
type Store interface {
	SaveOriginal(string, []byte) error
	ReadOriginal(string) ([]byte, error)
	RemoveOriginal(string) error
	SaveResult(string, []byte) error
	ResultPath(string) (string, error)
	ExpiredOriginals(time.Time) ([]string, error)
}
type Local struct{ root string }

func NewLocal(root string) (*Local, error) {
	for _, dir := range []string{"originals", "results"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			return nil, errors.New("图片存储目录无法创建")
		}
	}
	return &Local{root}, nil
}
func (l *Local) path(kind, id string) (string, error) {
	if !validID.MatchString(id) {
		return "", errors.New("invalid image id")
	}
	return filepath.Join(l.root, kind, id+".jpg"), nil
}
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".writing-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
func (l *Local) SaveOriginal(id string, data []byte) error {
	path, err := l.path("originals", id)
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}
func (l *Local) ReadOriginal(id string) ([]byte, error) {
	path, err := l.path("originals", id)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
func (l *Local) RemoveOriginal(id string) error {
	path, err := l.path("originals", id)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func (l *Local) SaveResult(id string, data []byte) error {
	path, err := l.path("results", id)
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}
func (l *Local) ResultPath(id string) (string, error) {
	path, err := l.path("results", id)
	if err != nil {
		return "", err
	}
	if _, err = os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func (l *Local) ExpiredOriginals(cutoff time.Time) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(l.root, "originals"))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".jpg")
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jpg") || !validID.MatchString(id) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.ModTime().Before(cutoff) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
