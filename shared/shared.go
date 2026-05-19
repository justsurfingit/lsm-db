package shared

import (
	"encoding/binary"
)
var Tombstone=[]byte("TOMBSTONE")
type IndexEntry struct {
	Key    string
	Offset int64 // The byte position in the .sst file
}
//SSTable index used for storing stuff about active SSTable in memory
type ActiveSSTableMeta struct{
	ID       int
	FilePath string
	RefCount int32
}



func EncodeRecords(key string, value []byte) []byte {
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