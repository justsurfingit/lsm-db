package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	lsmdb "github.com/justsurfingit/lsm-db/db" 
)

const (
	numWorkers   = 100  // 100 concurrent threads
	opsPerWorker = 1000 // Each thread does 1000 inserts
)

func main() {
	fmt.Println("Starting High-Concurrency Stress Test...")

	// 1. Initialize a fresh Database
	os.RemoveAll("stress_data") // Clean up old test data
	database, err := lsmdb.NewDb("stress_data")
	if err != nil {
		panic(err)
	}

	// 2. Setup the WaitGroup
	var wg sync.WaitGroup
	fmt.Printf("Spawning %d Goroutines (Total Operations: %d)...\n", numWorkers, numWorkers*opsPerWorker)
	// Start the timer!
	start := time.Now()

	// 3. Spawn the Workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1) // Tell the WaitGroup we are adding 1 worker

		go func(workerID int) {
			defer wg.Done() // Tell the WaitGroup we are finished when this function exits

			for j := 0; j < opsPerWorker; j++ {
				// Generate a random key between 0 and 50,000 to force collisions/overwrites
				key := fmt.Sprintf("key-%d", rand.Intn(50000))
				value := fmt.Sprintf("value-from-worker-%d", workerID)

				// Fire the Put request!
				err := database.Put(key, []byte(value))
				if err != nil {
					fmt.Println("DB Write Error:", err)
				}
			}
		}(i)
	}

	// 4. Wait for all 100 Goroutines to finish
	wg.Wait()

	// Stop the timer!
	duration := time.Since(start)
	totalOps := numWorkers * opsPerWorker
	opsPerSec := float64(totalOps) / duration.Seconds()

	// 5. Print the Recruiter-Friendly Report
	fmt.Println("\n✅ Stress Test Completed Successfully!")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Total Time taken : %v\n", duration)
	fmt.Printf("Total Writes     : %d\n", totalOps)
	fmt.Printf("Write Throughput : %.2f Ops/Sec\n", opsPerSec)
	fmt.Println("--------------------------------------------------")
}
