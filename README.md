# Eco-Raft

A Go implementation of the Raft consensus algorithm and a fault-tolerant key-value store built on top of it.

## Overview

Eco-Raft is a distributed consensus system implementing the Raft algorithm, which ensures consistency across a cluster of servers even in the presence of failures. The project includes a key-value store (KVRaft) that leverages Raft for fault-tolerant, replicated state management.

## Features

### Raft Consensus Algorithm
- **Leader Election**: Automatic leader election with randomized election timeouts
- **Log Replication**: Reliable replication of log entries across the cluster
- **State Persistence**: Persistent storage of critical state (current term, voted for, logs)
- **Safety**: Ensures that committed entries are never lost
- **Availability**: Continues operation as long as a majority of servers are available

### Key-Value Store (KVRaft)
- **Fault-Tolerant Operations**: Get, Put, and Append operations with linearizable semantics
- **Client Request Handling**: Automatic redirection to the current leader
- **Idempotency**: Duplicate request detection using client and request IDs
- **Consensus-Based Replication**: All state changes go through Raft for consistency

## Project Structure

```
eco-raft/
├── src/
│   ├── raft/
│   │   ├── raft.go           # Core Raft implementation
│   │   ├── config.go         # Test configuration
│   │   ├── persister.go      # Persistence layer
│   │   ├── util.go           # Utility functions
│   │   └── test_test.go      # Raft tests
│   ├── kvraft/
│   │   ├── server.go         # KV server implementation
│   │   ├── client.go         # KV client implementation
│   │   ├── common.go         # Shared definitions
│   │   ├── config.go         # Test configuration
│   │   └── test_test.go      # KV store tests
│   └── labrpc/
│       ├── labrpc.go         # RPC simulation library
│       └── test_test.go      # RPC tests
```

## Prerequisites

- Go 1.13 or higher
- Git

## Building and Running

### Setting up Go environment

```bash
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

### Running Raft tests

```bash
cd src/raft
go test
```

### Running KVRaft tests

```bash
cd src/kvraft
go test
```

### Running specific tests

```bash
# Run a specific test
go test -run TestBasic

# Run with verbose output
go test -v

# Run with race detector
go test -race
```

## Implementation Details

### Raft Algorithm

The Raft implementation includes:

- **State Management**: Each server can be in one of three states: Follower, Candidate, or Leader
- **Term-based System**: Logical clock for detecting stale information
- **AppendEntries RPC**: Used for heartbeats and log replication
- **RequestVote RPC**: Used during leader election
- **Log Compaction**: Support for snapshots (Assignment 3)

### Key-Value Store

The KVRaft service provides:

- **Get(key)**: Retrieves the current value for a key
- **Put(key, value)**: Sets the value for a key
- **Append(key, value)**: Appends a value to an existing key

All operations are processed through the Raft log to ensure consistency and fault tolerance.

### RPC Simulation

The `labrpc` package provides a simulated network for testing that can:
- Introduce network delays
- Simulate network partitions
- Drop messages
- Reorder messages

## Testing

The project includes comprehensive tests that verify:

- Basic leader election
- Log replication
- Safety properties
- Persistence and recovery
- Network partition handling
- KV store operations under various failure scenarios

## Debugging

Set the `Debug` constant in `kvraft/server.go` to enable detailed logging:

```go
const Debug = 1  // Set to 1 for debugging, 0 to turn off
```

## Architecture Notes

- **Concurrency**: Uses Go's goroutines and channels for concurrent operations
- **Synchronization**: Protects shared state with mutexes
- **Persistence**: Encodes state using Go's `encoding/gob` package
- **RPC**: Uses the custom `labrpc` package for simulated network communication

## Known Limitations

- Designed for testing and educational purposes
- Uses simulated network rather than real network communication
- Snapshot functionality (Assignment 3) may not be fully implemented

## References

- [Raft Paper](https://raft.github.io/raft.pdf) - "In Search of an Understandable Consensus Algorithm"
- [Raft Visualization](http://thesecretlivesofdata.com/raft/)

## License

This project appears to be educational/assignment code. Please check with your institution regarding usage and distribution policies.
