package wal

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/justsurfingit/lsm-db/memtable"
	"github.com/justsurfingit/lsm-db/shared"
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
	record := shared.EncodeRecords(key, value)
	_, err := w.file.Write(record)
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
func (w *Wal) GetAll() ([]memtable.KVPair, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Ensure we read from the beginning of the file
	_, err := w.file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	var KVpairs []memtable.KVPair
	for {
		var KeySize, ValueSize uint32
		err := binary.Read(w.file, binary.LittleEndian, &KeySize)
		if err == io.EOF {
			break // Successfully reached the end of the WAL
		}
		if err != nil {
			return nil, err
		}
		err = binary.Read(w.file, binary.LittleEndian, &ValueSize)
		if err != nil {
			return nil, err
		}
		key := make([]byte, KeySize)
		value := make([]byte, ValueSize)
		_, err = io.ReadFull(w.file, key)
		if err != nil {
			return nil, err
		}
		_, err = io.ReadFull(w.file, value)
		if err != nil {
			return nil, err
		}
		KVpairs = append(KVpairs, memtable.KVPair{
			Key:   string(key),
			Value: value,
		})
	}
	// Seek back to the end of the file so future writes append correctly
	_, err = w.file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	return KVpairs, nil
}