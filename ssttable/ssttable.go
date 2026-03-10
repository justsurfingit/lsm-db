package ssttable

import (
	"encoding/binary"
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
