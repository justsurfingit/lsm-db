# LSM-DB: A Log-Structured Merge-Tree Database in Go

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)
![Data Structure](https://img.shields.io/badge/Architecture-LSM%20Tree-blue)
![Database](https://img.shields.io/badge/Type-Key%2FValue%20Store-orange)

## Overview
LSM-DB is a highly performant, embeddable Key-Value store built from scratch in Go. It implements the core mechanics of a **Log-Structured Merge-Tree (LSM)** storage engine, prioritizing high-throughput write performance and robust crash durability.

This project was built to explore low-level database systems engineering, disk I/O optimization, and memory management.

## Key Features & Architecture

### 1. In-Memory SkipList (Memtable)
All writes are initially buffered in a probabilistic SkipList. This allows for O(log N) average-case insertions and lookups while maintaining the keys in strictly sorted order.

### 2. Write-Ahead Log (WAL) & Crash Recovery
To ensure Data Durability (the 'D' in ACID), all operations are sequentially written to a binary-encoded `wal.log` file on disk before modifying the Memtable. 
* **Crash Recovery**: If the database crashes or loses power, the initialization sequence automatically replays the un-flushed WAL records, rebuilding the Memtable state seamlessly.

### 3. Sorted String Tables (SSTables)
When the Memtable exceeds its maximum capacity limit, it is flushed to disk as an immutable, binary-encoded SSTable file. The WAL is then truncated.

### 4. Sparse Indexing (O(1) Disk Lookups)
Instead of scanning an entire SSTable to find a key (O(N) read latency), the database implements **Sparse Indexing**. 
* During an SSTable flush, an index mapping equidistant keys to exact file byte-offsets is stored at the end of the file.
* On reads, the index is loaded into memory, binary-searched, and standard OS file-seeking (`io.SeekStart`) is utilized to jump directly to the relevant data block on disk.

### 5. Background Compaction
Over time, repeated flushes create multiple SSTables containing overlapping subsets of keys or tombstone markers (deleted records). 
The database runs a concurrent background goroutine that periodically merges old SSTables together in a K-way merge, discarding overridden keys, reclaiming disk space, and reducing read latencies.

## Getting Started

### Prerequisites
- Go 1.25 or higher

### Initialization & Basic Usage
```go
package main

import (
	"fmt"
	"github.com/justsurfingit/lsm-db/db"
)

func main() {
    // 1. Initialize DB (Auto-runs WAL Crash Recovery)
	dbConnection, err := lsmdb.NewDb("mydb_data")
	if err != nil {
		panic(err)
	}
    
    // 2. Insert Data
	err = dbConnection.Put("user_001", []byte("Alice"))
	if err != nil {
	    panic(err)
	}
	
	// 3. Query Data (Checks Memtable -> Sparse Indexed SSTables)
	res, found, err := dbConnection.Get("user_001")
	if err != nil {
		fmt.Println("Error:", err)
	} else if found {
		fmt.Printf("Found: %s\n", string(res))
	} else {
		fmt.Println("Not found")
	}
}
```

## Future Roadmap (Planned Enhancements)
- [ ] **Bloom Filters**: Injecting bitsets into SSTables to skip reading files completely if a key is guaranteed not to exist.
- [ ] **CLI / HTTP API**: Exposing a REDIS-like interaction layer (`SET`, `GET`, `DEL`) over a TCP socket.
- [ ] **Data Compression**: Adding Snappy or LZ4 compression to SSTable data blocks.
