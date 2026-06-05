package main

import (
	"fmt"
	"os"

	lsmdb "github.com/justsurfingit/lsm-db/db"
	"github.com/justsurfingit/lsm-db/ui"
)

func main() {
	// 1. Boot up the LSM-Tree Database Engine
	database, err := lsmdb.NewDb("data")
	if err != nil {
		fmt.Println("❌ Fatal Error starting Database Engine:", err)
		os.Exit(1)
	}

	// 2. Launch the Interactive Dashboard
	err = ui.StartDashboard(database)
	if err != nil {
		fmt.Printf("❌ Fatal UI Error: %v\n", err)
		os.Exit(1)
	}
}
