package main

import (
	"fmt"

	lsmdb "github.com/justsurfingit/lsm-db/db"
)

func main() {
	dbConnection, err := lsmdb.NewDb("mydb_data")
	if err != nil {
		fmt.Println("Unable to establish connection with the database:", err)
		return
	}

	fmt.Println("--- Testing Crash Recovery ---")

	// 1. First, let's see if we recovered previous unflushed data
	res, found, _ := dbConnection.Get("user_crash_test")
	if found {
		fmt.Printf("RECOVERED DATA: %s\n", string(res))
		return
	}

	// 2. If not found, let's insert it (but don't insert enough to trigger a flush)
	fmt.Println("Data not found. Inserting new data into Memtable & WAL...")
	key := "user_crash_test"
	value := []byte("This data should survive a database restart even if not flushed to SSTable!")
	
	err = dbConnection.Put(key, value)
	if err != nil {
		fmt.Println("Error putting data:", err)
		return
	}
	
	fmt.Println("Data inserted into Memtable and appended to WAL.")
	fmt.Println("Simulating a crash by exiting before flush...")
}
