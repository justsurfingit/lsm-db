package wal

import (
	"os"
	"path/filepath"
	"sync"
)

// WAL struct is created
// WAL- Write ahead log basically it's used to make the database secure such that it can survice the system crash basically all the update as they are done will be stored in this file
type Wal struct {
	file *os.File
	mu   sync.Mutex
}

// constructor
func NewWal(path string) (*Wal, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	walPath := filepath.Join(path, "wal.log")
	file, err := os.OpenFile(walPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	_, err = file.Seek(0, 2)
	if err != nil {
		return nil, err
	}

	return &Wal{file: file}, nil
}

// function that will apend at the end of the
func (w *Wal) Append(key string, value []byte) error {
	// locking such that two different goroutine don't write to it at the same time
	w.mu.Lock()
	defer w.mu.Unlock()
	entry := key + ":" + string(value) + "\n"
	_, err := w.file.WriteString(entry)
	if err != nil {
		return err
	}
	// now we have to force the os to save this content directly to the operating system or else it can be lost
	err = w.file.Sync()
	if err != nil {
		return err
	}
	return nil
}

func (w *Wal) Clear() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.file.Truncate(0)
	if err != nil {
		return err
	}
	_, err = w.file.Seek(0, 0)
	if err != nil {
		return err
	}
	return w.file.Sync()
}
