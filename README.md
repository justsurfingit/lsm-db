# LSM-DB: A Log-Structured Merge-Tree Database in Go

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)
![Data Structure](https://img.shields.io/badge/Architecture-LSM%20Tree-blue)
![Database](https://img.shields.io/badge/Type-Key%2FValue%20Store-orange)

## Overview
LSM-DB is a highly performant, embeddable Key-Value store built from scratch in Go. It implements the core mechanics of a **Log-Structured Merge-Tree (LSM)** storage engine, prioritizing high-throughput write performance and robust crash durability.

This project was built to explore low-level database systems engineering, disk I/O optimization, and memory management.

## Key Features & Architecture

### 1. High-Throughput Concurrency (Reference Counting)
Designed for massive concurrent read/write workloads. The database utilizes an advanced **Reference Counting** architecture for its on-disk files. 
* **Lock-Free Reads**: `Get()` requests take a microsecond snapshot of the active file list and increment reference counters, allowing thousands of Goroutines to perform slow disk I/O concurrently without ever blocking writers.
* **Safe Deletions**: Compaction threads safely isolate old files and wait for active readers' reference counts to hit zero before permanently removing files from the OS.

### 2. Event-Driven Background Compaction (Min-Heap & Channels)
To prevent Write Amplification, the database runs a background compaction worker to merge redundant SSTables.
* **Zero-CPU Polling**: Instead of busy-waiting or relying on Cron jobs, the compaction engine is entirely event-driven. It listens on a blocked Go Channel (`chan struct{}`), consuming 0% CPU until a Memtable flush triggers it.
* **K-Way Merge via Min-Heap**: Compaction utilizes a generic Min-Heap (`container/heap`) to perform highly efficient $O(N \log K)$ merges across multiple SSTable streams, dropping Tombstoned records and reclaiming disk space.

### 3. In-Memory SkipList (Memtable)
All writes are initially buffered in a thread-safe, probabilistic SkipList, allowing for O(log N) average-case insertions and lookups while maintaining the keys in strictly sorted order.

### 4. Write-Ahead Log (WAL) & Crash Recovery
To ensure Data Durability (the 'D' in ACID), operations are sequentially written to a binary-encoded `wal.log` file before modifying the Memtable. 
* If the database crashes or loses power, the initialization sequence automatically replays un-flushed WAL records, rebuilding the Memtable state seamlessly.

### 5. Sparse Indexing (O(1) Disk Seeks)
Instead of scanning an entire SSTable to find a key (which incurs $O(N)$ disk reads), the database implements Sparse Indexing. During an SSTable flush, an index mapping equidistant keys to exact file byte-offsets is appended to the footer. On reads, the index is searched in memory, and standard OS file-seeking (`io.SeekStart`) is utilized to jump directly to the relevant data block on disk, guaranteeing $O(1)$ disk seeks per file.

### 6. Interactive TUI Dashboard (Bubble Tea)
The database features a stunning Terminal User Interface (TUI) built with Charmbracelet's **Bubble Tea** (Elm Architecture). It provides a real-time visual dashboard of Memtable capacities, active SSTable counts, and an interactive command prompt for executing live `PUT`, `GET`, `DEL`, and `COMPACT` queries.

## Getting Started

### Prerequisites
- Go 1.25 or higher

### Launching the Interactive Dashboard
```bash
go mod tidy
go run main.go
```
*Once launched, use the bottom input panel to execute commands like `PUT user01 alice` or `GET user01`.*

## Future Roadmap (Planned Enhancements)
- [ ] **Bloom Filters**: Injecting bitsets into SSTables to skip reading files completely if a key is guaranteed not to exist.
- [x] **Interactive CLI Dashboard**: Built using Bubble Tea.
- [ ] **Data Compression**: Adding Snappy or LZ4 compression to SSTable data blocks.
