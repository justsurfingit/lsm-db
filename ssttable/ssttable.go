package sstable

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/justsurfingit/lsm-db/memtable"
	"github.com/justsurfingit/lsm-db/shared"
)

// SparseIndex indicate every which record will act as anchor for our sparse index
const SparseIndex = 10

// sst table will need skipList value and through them it will create a sst file


// we have to maintain indices also here
func WriteSSTable(filepath string, store []memtable.KVPair) ([]shared.IndexEntry, error) {
	file, err := os.Create(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var Index []shared.IndexEntry

	for idx, value := range store {
		if idx%SparseIndex == 0 {
			// we need to get offset
			offset, err := file.Seek(0, 1)
			if err != nil {
				return nil, err
			}
			Index = append(Index, shared.IndexEntry{
				Key:    value.Key,
				Offset: offset,
			})
		}

		curRecord := shared.EncodeRecords(value.Key, value.Value)
		_, err := file.Write(curRecord)
		if err != nil {
			return nil, err
		}
	}
	//capturing the index offset
	indexStartOffset, err := file.Seek(0, 1)
	if err != nil {
		return nil, err
	}
	// number of entries in index
	indexSize := uint32(len(Index))
	err = binary.Write(file, binary.LittleEndian, indexSize)
	if err != nil {
		return nil, err
	}
	// now write index to the file
	for _, entry := range Index {
		keySize := uint32(len(entry.Key))
		// offset size is not required as its integer fixed size of 8  byte
		err := binary.Write(file, binary.LittleEndian, uint32(keySize))
		if err != nil {
			return nil, err
		}

		_, err = file.Write([]byte(entry.Key))
		if err != nil {
			return nil, err
		}
		err = binary.Write(file, binary.LittleEndian, entry.Offset)
		if err != nil {
			return nil, err
		}
	}
	// writing the index offset at the end
	binary.Write(file, binary.LittleEndian, uint64(indexStartOffset))
	return Index, nil

}
func LoadIndexFromSSTFile(filepath string) ([]shared.IndexEntry, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Step 1: Read the footer — last 8 bytes contain the index start offset
	_, err = file.Seek(-8, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	var indexStartOffset uint64
	err = binary.Read(file, binary.LittleEndian, &indexStartOffset)
	if err != nil {
		return nil, err
	}

	// Step 2: Seek to the index start offset
	_, err = file.Seek(int64(indexStartOffset), io.SeekStart)
	if err != nil {
		return nil, err
	}

	// Step 3: Read the number of index entries
	var indexSize uint32
	err = binary.Read(file, binary.LittleEndian, &indexSize)
	if err != nil {
		return nil, err
	}

	// Step 4: Read each index entry
	var index []shared.IndexEntry
	for i := 0; i < int(indexSize); i++ {
		// keySize was written as uint64 in WriteSSTable
		var keySize uint32
		err = binary.Read(file, binary.LittleEndian, &keySize)
		if err != nil {
			return nil, err
		}

		// fetch the key
		keyBuf := make([]byte, keySize)
		_, err = io.ReadFull(file, keyBuf)
		if err != nil {
			return nil, err
		}

		// fetch the offset
		var offset int64
		err = binary.Read(file, binary.LittleEndian, &offset)
		if err != nil {
			return nil, err
		}

		index = append(index, shared.IndexEntry{
			Key:    string(keyBuf),
			Offset: offset,
		})
	}

	return index, nil
}
func SearchSSTFile(filepath string, key string) ([]byte, error) {
	index, err := LoadIndexFromSSTFile(filepath)
	if err != nil {
		return nil, err
	}

	var reqOffset int64 = -1

	for _, i := range index {
		if i.Key <= key {
			reqOffset = i.Offset
		} else {
			break
		}
	}

	// the required content is not in the file
	if reqOffset == -1 {
		return nil, nil
	}

	// open the file and search for the content
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// get the index start offset so we know where data ends
	_, err = file.Seek(-8, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	var indexStartOffset uint64
	err = binary.Read(file, binary.LittleEndian, &indexStartOffset)
	if err != nil {
		return nil, err
	}

	// seek to the offset found from the sparse index
	_, err = file.Seek(reqOffset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	for {
		// check if we've reached the index region — data is done
		curOffset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		if curOffset >= int64(indexStartOffset) {
			break
		}

		var keySize, valueSize uint32
		err = binary.Read(file, binary.LittleEndian, &keySize)
		if err != nil {
			return nil, err
		}
		err = binary.Read(file, binary.LittleEndian, &valueSize)
		if err != nil {
			return nil, err
		}

		keyBuf := make([]byte, keySize)
		_, err = io.ReadFull(file, keyBuf)
		if err != nil {
			return nil, err
		}

		if string(keyBuf) == key {
			valueBuf := make([]byte, valueSize)
			_, err = io.ReadFull(file, valueBuf)
			if err != nil {
				return nil, err
			}
			return valueBuf, nil
		}

		// keys are sorted — if we've passed it, stop early
		if string(keyBuf) > key {
			break
		}

		// skip the value bytes and move to the next record
		_, err = file.Seek(int64(valueSize), io.SeekCurrent)
		if err != nil {
			return nil, err
		}
	}

	// key not found
	return nil, nil
}

