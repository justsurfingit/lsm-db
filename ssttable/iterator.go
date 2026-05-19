package sstable

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/justsurfingit/lsm-db/memtable"
)

// SSTableIterator struct for iterating the SSTable
// fp - pointer to the cur file
// curOffset - current position
// lastOffset - index starting offset (where data ends)
type SSTableIterator struct {
	fp         *os.File
	curOffset  int64
	lastOffset int64
}

// NewSSTableIterator creates a new iterator for an SSTable file
func NewSSTableIterator(filepath string) (*SSTableIterator, error) {
	fp, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	// get last offset
	_, err = fp.Seek(-8, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	var lastOffset int64
	err = binary.Read(fp, binary.LittleEndian, &lastOffset)
	if err != nil {
		return nil, err
	}

	// seek back to start
	_, err = fp.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	return &SSTableIterator{
		fp:         fp,
		curOffset:  0,
		lastOffset: lastOffset,
	}, nil
}

func (s *SSTableIterator) HasNext() bool {
	return s.curOffset < s.lastOffset
}

func (s *SSTableIterator) Next() (*memtable.KVPair, error) {
	if !s.HasNext() {
		return nil, nil
	}
	//reading the content
	var keySize uint32
	err := binary.Read(s.fp, binary.LittleEndian, &keySize)
	if err != nil {
		return nil, err
	}
	var valueSize uint32
	err = binary.Read(s.fp, binary.LittleEndian, &valueSize)
	if err != nil {
		return nil, err
	}
	keyBuf := make([]byte, keySize)
	_, err = io.ReadFull(s.fp, keyBuf)
	if err != nil {
		return nil, err
	}
	valueBuf := make([]byte, valueSize)
	_, err = io.ReadFull(s.fp, valueBuf)
	if err != nil {
		return nil, err
	}
	
	// moving the curpointer to the next entry 
	// 8 bytes for keySize (4) + valueSize (4)
	s.curOffset += 8 + int64(keySize+valueSize)
	
	return &memtable.KVPair{
		Key:   string(keyBuf),
		Value: valueBuf,
	}, nil
}

func (s *SSTableIterator) Close() error {
	return s.fp.Close()
}