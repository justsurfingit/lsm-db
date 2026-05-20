package lsmdb

import (
	"container/heap"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/justsurfingit/lsm-db/memtable"
	"github.com/justsurfingit/lsm-db/shared"
	sstable "github.com/justsurfingit/lsm-db/ssttable"
)

const mergeSize = 4

type HeapItem struct {
	ScannerID int
	KVPair    *memtable.KVPair
	fp        *sstable.SSTableIterator
}
type MergeHeap []*HeapItem

func (h MergeHeap) Len() int { return len(h) }
func (h MergeHeap) Less(i, j int) bool {
	if h[i].KVPair.Key == h[j].KVPair.Key {
		// as greater scanerID means latest sst table with new entries
		return h[i].ScannerID > h[j].ScannerID
	}
	return h[i].KVPair.Key < h[j].KVPair.Key
}
func (h MergeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *MergeHeap) Push(x interface{}) {
	*h = append(*h, x.(*HeapItem))
}
func (h *MergeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

func integerIDParser(id string) int {
	temp := strings.TrimPrefix(id, "sst-")
	temp = strings.TrimSuffix(temp, ".sst")
	val, err := strconv.Atoi(temp)
	if err != nil {
		return -1
	}
	return val
}

func (d *Db) CompactionManual() error {
	//as we are maintaing it so why don't just use it
	fileList := d.activeSSTables
	// enough file check if number of files are less than required then there is no need for compaction
	if len(fileList) <= mergeSize {
		return nil
	}
	// sorting the files based on the ID
	// sort.Slice(fileList, func(i, j int) bool {
	// 	return integerIDParser(fileList[i].Name()) < integerIDParser(fileList[j].Name())
	// })
	// taking the last mergeSize files
	filesToMerge := fileList[len(fileList)-mergeSize:]

	// initialize min heap
	h := &MergeHeap{}
	heap.Init(h)
	// seeding first entries of all files into the heap
	for i := 0; i < len(filesToMerge); i++ {
		fullPath := filesToMerge[i].FilePath
		fileId := filesToMerge[i].ID
		iterator, err := sstable.NewSSTableIterator(fullPath)
		if err != nil {
			return err
		}
		kvp, err := iterator.Next()
		if err != nil {
			return err
		}
		if kvp != nil {
			heap.Push(h, &HeapItem{
				ScannerID: fileId,
				KVPair:    kvp,
				fp:        iterator,
			})
		}
	}
	// creating temporary file for storing merged values
	// name should be sst-<minid>-temp.sst
	tempfileId := filesToMerge[0].ID
	tempFilePath := filepath.Join(d.sstDir, "sst-"+strconv.Itoa(tempfileId)+"-temp.sst")

	var compactedData []memtable.KVPair
	var lastKey string
	var isFirst = true

	// actual compaction
	for h.Len() > 0 {
		// extract min from heap
		minItem := heap.Pop(h).(*HeapItem)

		// Process minItem
		if isFirst || minItem.KVPair.Key != lastKey {
			lastKey = minItem.KVPair.Key
			isFirst = false
			if string(minItem.KVPair.Value) != string(shared.Tombstone) {
				compactedData = append(compactedData, *minItem.KVPair)
			}
		}

		// fetch the next key value pair from the file
		nextkvp, err := minItem.fp.Next()
		if err != nil {
			return err
		}
		// if nextkvp is nill that means we have reached the end of the file
		if nextkvp == nil {
			minItem.fp.Close()
		} else {
			// pushing the next key value pair into the heap
			heap.Push(h, &HeapItem{
				ScannerID: minItem.ScannerID,
				KVPair:    nextkvp,
				fp:        minItem.fp,
			})
		}
	}
	// eventually everything will be done
	// write the fully merged slice to a temporary SSTable
	_, err := sstable.WriteSSTable(tempFilePath, compactedData)
	if err != nil {
		return err
	}
	tempSSTName := filepath.Join(d.sstDir, fmt.Sprintf("sst-%d.sst", tempfileId))
	err = os.Rename(tempFilePath, tempSSTName)
	if err != nil {
		return err
	}

	// storing new sst file into activeSSTable
	d.mu.Lock()
	var neoActiveSST []*shared.ActiveSSTableMeta
	for i := 0; i < len(d.activeSSTables)-mergeSize; i++ {
		neoActiveSST = append(neoActiveSST, d.activeSSTables[i])
	}
	neoActiveSST = append(neoActiveSST, &shared.ActiveSSTableMeta{
		ID:       tempfileId,
		FilePath: tempSSTName,
		RefCount: 0,
	})
	d.activeSSTables = neoActiveSST
	d.mu.Unlock()

	// Wait for readers to finish with the old files
	for {
		readersActive := false
		for _, oldFile := range filesToMerge {
			if atomic.LoadInt32(&oldFile.RefCount) > 0 {
				readersActive = true
				break
			}
		}
		if !readersActive {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// delete those files
	for _, file := range filesToMerge {
		os.Remove(file.FilePath)
	}

	return nil
}
