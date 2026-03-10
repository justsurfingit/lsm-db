package lsmdb

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/justsurfingit/lsm-db/memtable"
	"github.com/justsurfingit/lsm-db/ssttable"
	"github.com/justsurfingit/lsm-db/wal"
)

type Db struct {
	memtable        *memtable.SkipList
	wal             *wal.Wal
	memtableSize    int
	maxMemtableSize int
	nextFieldId     int
	sstDir          string
}

func NewDb(path string) (*Db, error) {
	nextID := 1 // Default if the folder is completely empty
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	sstDir := filepath.Join(path, "sst")
	if err := os.MkdirAll(sstDir, 0755); err != nil {
		return nil, err
	}

	files, err := os.ReadDir(sstDir)
	if err == nil {
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "sst-") && strings.HasSuffix(file.Name(), ".sst") {
				// Trim "sst-" prefix and ".sst" suffix to get the number
				name := strings.TrimPrefix(file.Name(), "sst-")
				name = strings.TrimSuffix(name, ".sst")
				id, convErr := strconv.Atoi(name)
				if convErr == nil && id >= nextID {
					nextID = id + 1
				}
			}
		}
	}
	wal, err := wal.NewWal(path)
	if err != nil {
		return nil, err
	}
	return &Db{
		memtable:        memtable.NewSkipList(),
		wal:             wal,
		memtableSize:    0,
		maxMemtableSize: 4096,
		nextFieldId:     nextID,
		sstDir:          sstDir,
	}, nil

}
func (d *Db) Put(key string, value []byte) error {
	// appending to the wal logs
	// if after appending the current key value pair it's greater then memtable size then we have to flush them to ssttable
	//let's do that
	curSize := len(key) + len(value)
	if curSize+d.memtableSize > d.maxMemtableSize {
		// fmt.Println(d.memtableSize)
		curSSTName := filepath.Join(d.sstDir, fmt.Sprintf("sst-%d.sst", d.nextFieldId))

		err := ssttable.WriteSSTtable(curSSTName, d.memtable.GetAll())
		if err != nil {
			return err
		}
		// clear wal
		d.wal.Clear()
		// clear the memtable
		d.nextFieldId += 1
		d.memtable = memtable.NewSkipList()
		d.memtableSize = 0
	}
	d.memtableSize += curSize
	err := d.wal.Append(key, value)
	if err != nil {
		return err
	}
	//now memtable
	d.memtable.Put(key, value)
	return nil
}
func (d *Db) Get(key string) ([]byte, bool) {
	return d.memtable.Get(key)
}
func (d *Db) GetAll() {
	d.memtable.PrintAll()
}
