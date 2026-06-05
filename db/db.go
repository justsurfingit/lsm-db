package lsmdb

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/justsurfingit/lsm-db/memtable"
	"github.com/justsurfingit/lsm-db/shared"
	sstable "github.com/justsurfingit/lsm-db/ssttable"

	"github.com/justsurfingit/lsm-db/wal"
)

// index Entry

type Db struct {
	memtable        *memtable.SkipList
	wal             *wal.Wal
	memtableSize    int
	maxMemtableSize int
	nextFieldId     int
	sstDir          string
	indices         map[string]shared.IndexEntry
	mu              sync.RWMutex
	activeSSTables  []*shared.ActiveSSTableMeta
	compactionChan  chan struct{}
}
type DbStats struct {
	MemtableSize    int
	MaxMemtableSize int
	ActiveSSTables  int
}

// GetStats returns thread-safe metrics for the UI dashboard
func (d *Db) GetStats() DbStats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return DbStats{
		MemtableSize:    d.memtableSize,
		MaxMemtableSize: d.maxMemtableSize,
		ActiveSSTables:  len(d.activeSSTables),
	}
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
	// just checking for the latest nextID
	activeSSTables := make([]*shared.ActiveSSTableMeta, 0)
	if err == nil {
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "sst-") && strings.HasSuffix(file.Name(), ".sst") {
				// Trim "sst-" prefix and ".sst" suffix to get the number
				name := strings.TrimPrefix(file.Name(), "sst-")
				name = strings.TrimSuffix(name, ".sst")
				id, convErr := strconv.Atoi(name)
				if convErr == nil {
					if id >= nextID {
						nextID = id + 1
					}
					fullPath := filepath.Join(sstDir, file.Name())
					activeSSTables = append(activeSSTables, &shared.ActiveSSTableMeta{
						ID:       id,
						FilePath: fullPath,
						RefCount: 0,
					})
				}
			}
		}
	}

	// sort in decending order highest ID first
	sort.Slice(activeSSTables, func(i, j int) bool {
		return activeSSTables[i].ID > activeSSTables[j].ID
	})
	// d.activeSSTables = activeSSTables

	wal, err := wal.NewWal(path)
	if err != nil {
		return nil, err
	}
	// wAL replay
	WALcontent, err := wal.GetAll()
	if err != nil {
		return nil, err
	}
	loadedMemtable := memtable.NewSkipList()
	memSize := 0
	for _, kvpair := range WALcontent {
		prevVal, found := loadedMemtable.Get(kvpair.Key)
		cur := len(kvpair.Key) + len(kvpair.Value)
		if found {
			cur -= (len(kvpair.Key) + len(prevVal))
		}
		memSize += cur
		loadedMemtable.Put(kvpair.Key, kvpair.Value)
	}
	ch := make(chan struct{})
	// spwan a goroutine for automatic event driven compaction

	db := &Db{
		memtable:        loadedMemtable,
		wal:             wal,
		memtableSize:    memSize,
		maxMemtableSize: 4096,
		nextFieldId:     nextID,
		sstDir:          sstDir,
		indices:         make(map[string]shared.IndexEntry),
		activeSSTables:  activeSSTables,
		compactionChan:  ch,
	}
	go func() {
		for range db.compactionChan {
			db.mu.RLock()
			count := len(db.activeSSTables)
			db.mu.RUnlock()
			if count >= mergeSize {
				// The threshold is hit! Run it!
				db.CompactionManual()
			}
		}
	}()
	return db, nil

}
func (d *Db) Put(key string, value []byte) error {
	// appending to the wal logs
	// if after appending the current key value pair it's greater then memtable size then we have to flush them to ssttable
	//let's do that
	d.mu.Lock()
	defer d.mu.Unlock()
	curSize := len(key) + len(value)
	// check if previously present or not
	prevVal, f := d.memtable.Get(key)
	if f {
		curSize -= (len(prevVal) + len(key))
	}
	if curSize+d.memtableSize > d.maxMemtableSize {
		// fmt.Println(d.memtableSize)
		curSSTName := filepath.Join(d.sstDir, fmt.Sprintf("sst-%d.sst", d.nextFieldId))

		_, err := sstable.WriteSSTable(curSSTName, d.memtable.GetAll())
		if err != nil {
			return err
		}
		//add the channel triggering step
		select {
		case d.compactionChan <- struct{}{}:
			// Token sent successfully! Worker is waking up.
		default:
			// Pipe is full or worker is busy, skip sending.
		}
		// add entry to the active sstable
		newEntry := &shared.ActiveSSTableMeta{
			ID:       d.nextFieldId,
			FilePath: curSSTName,
			RefCount: 0,
		}
		d.activeSSTables = append([]*shared.ActiveSSTableMeta{newEntry}, d.activeSSTables...)
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
	d.mu.RLock()

	var files []*shared.ActiveSSTableMeta
	for _, sst := range d.activeSSTables {
		//as it is being read so reference count increases by 1
		atomic.AddInt32(&sst.RefCount, 1)
		files = append(files, sst)
	}
	d.mu.RUnlock()

	//decreasing the reference count when work is done
	defer func() {
		for _, file := range files {
			atomic.AddInt32(&file.RefCount, -1)
		}
	}()
	// parseSSTID := func(name string) int {
	// 	if !strings.HasPrefix(name, "sst-") || !strings.HasSuffix(name, ".sst") {
	// 		return -1
	// 	}
	// 	idStr := strings.TrimSuffix(strings.TrimPrefix(name, "sst-"), ".sst")
	// 	id, err := strconv.Atoi(idStr)
	// 	if err != nil {
	// 		return -1
	// 	}
	// 	return id
	// }
	// // Newest SST first (decreasing order)
	// sort.Slice(files, func(i, j int) bool {
	// 	return parseSSTID(files[i].FilePath) > parseSSTID(files[j].FilePath)
	// })
	for _, file := range files {
		data, err := sstable.SearchSSTFile(file.FilePath, key)
		if err != nil {
			return nil, false, err
		}
		if data != nil {
			return data, true, nil
		}
	}

	return nil, false, nil
}

func (d *Db) GetAll() {
	d.memtable.PrintAll()
}
