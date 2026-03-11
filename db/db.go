package lsmdb

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/justsurfingit/lsm-db/memtable"
	sstable "github.com/justsurfingit/lsm-db/ssttable"

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
	// check if previously present or not
	prevVal, f := d.memtable.Get(key)
	if f {
		curSize -= (len(prevVal) + len(key))
	}
	if curSize+d.memtableSize > d.maxMemtableSize {
		// fmt.Println(d.memtableSize)
		curSSTName := filepath.Join(d.sstDir, fmt.Sprintf("sst-%d.sst", d.nextFieldId))

		err := sstable.WriteSSTtable(curSSTName, d.memtable.GetAll())
		if err != nil {
			return err
		}
		// clear wal
		err = d.wal.Clear()
		if err != nil {
			return err
		}
		// clear the memtable
		d.nextFieldId += 1
		d.memtable = memtable.NewSkipList()
		d.memtableSize = 0
		// as in case of flush entire new entry will be stored even if it's present already.
		curSize = len(key) + len(value)
	}

	err := d.wal.Append(key, value)
	if err != nil {
		return err
	}
	//now memtable
	d.memtable.Put(key, value)
	d.memtableSize += curSize

	return nil
}
func (d *Db) Get(key string) ([]byte, bool, error) {
	data, found := d.memtable.Get(key)
	if found {
		return data, true, nil
	}

	files, err := os.ReadDir(d.sstDir)
	if err != nil {
		return nil, false, err
	}

	parseSSTID := func(name string) int {
		if !strings.HasPrefix(name, "sst-") || !strings.HasSuffix(name, ".sst") {
			return -1
		}
		idStr := strings.TrimSuffix(strings.TrimPrefix(name, "sst-"), ".sst")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return -1
		}
		return id
	}

	// Newest SST first (decreasing order)
	sort.Slice(files, func(i, j int) bool {
		return parseSSTID(files[i].Name()) > parseSSTID(files[j].Name())
	})

	for _, file := range files {
		if file.IsDir() || parseSSTID(file.Name()) < 0 {
			continue
		}
		fullPath := filepath.Join(d.sstDir, file.Name())
		data, found, err := sstable.SearchSSTable(fullPath, key)
		if err != nil {
			return nil, false, err
		}
		if found {
			return data, true, nil
		}
	}

	return nil, false, nil
}

func (d *Db) GetAll() {
	d.memtable.PrintAll()
}
