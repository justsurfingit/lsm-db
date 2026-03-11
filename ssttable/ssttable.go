package sstable

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/justsurfingit/lsm-db/memtable"
)

// sst table will need skipList value and through them it will create a sst file
func encodeRecords(key string, value []byte) []byte {
	keySize := uint32(len(key))
	valueSize := uint32(len(value))
	// uint has fixed size of 4 byte so it will make this system agnostic no matter which architecture we are using it will store number in uint format using 4 byte only.
	totSize := 8 + len(key) + len(value)
	record := make([]byte, totSize)
	binary.LittleEndian.PutUint32(record[0:4], keySize)
	binary.LittleEndian.PutUint32(record[4:8], valueSize)
	copy(record[8:8+keySize], key)
	copy(record[8+keySize:], value)
	return record
}
func WriteSSTtable(filepath string, store []memtable.KVPair) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, value := range store {
		curRecord := encodeRecords(value.Key, value.Value)
		_, err := file.Write(curRecord)
		if err != nil {
			return err
		}
	}
	return nil

}
func SearchSSTable(filepath string, key string) ([]byte, bool, error) {
	file, err := os.Open(filepath)
	if err != nil {
		fmt.Println("failed to open file")
		return nil, false, err
	}
	defer file.Close()
	// now we are searching the file and we are good to go then
	for {
		var keySize uint32
		var valueSize uint32
		err := binary.Read(file, binary.LittleEndian, &keySize)
		// reached end of file meaning data not found
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error occured while reading sst file", err)
			return nil, false, err
		}
		err = binary.Read(file, binary.LittleEndian, &valueSize)
		if err != nil {
			return nil, false, err
		}
		// reading content
		keyBuf := make([]byte, keySize)

		_, err = io.ReadFull(file, keyBuf)
		if err != nil {
			return nil, false, err
		}
		if string(keyBuf) == key {
			valueBuf := make([]byte, valueSize)
			_, err = io.ReadFull(file, valueBuf)
			if err != nil {
				return nil, false, err
			}
			return valueBuf, true, nil

		}
		_, err = file.Seek(int64(valueSize), 1)
		if err != nil {
			return nil, false, err
		}

	}
	return nil, false, nil
}
