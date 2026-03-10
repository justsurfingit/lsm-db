package main

import (
	"fmt"

	lsmdb "github.com/justsurfingit/lsm-db/db"
)

func main() {
	dbConnection, err := lsmdb.NewDb("mydb_data")
	if err != nil {
		fmt.Println("Unable to establish connection with the database, reasons: ", err)
		return
	}
	fmt.Println("--- Starting insertion to trigger flush ---")

	// We will insert 100 keys.
	// Each value is ~100 bytes. 100 * 100 = 10,000 bytes.
	// Since the limit is 4,096, this SHOULD trigger at least 2 flushes!
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("user_%03d", i)
		value := []byte(fmt.Sprintf("this is a fairly long string of data for user %d to help fill the memtable", i))

		err := dbConnection.Put(key, value)
		if err != nil {
			fmt.Printf("Error putting key %s: %v\n", key, err)
		}
	}

}
