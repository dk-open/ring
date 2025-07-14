# ring
High-performance lock-free SPMC ring buffer with two-phase dequeue protocol and callback-based safety

![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/dk-open/ring)](https://goreportcard.com/report/github.com/dk-open/ring)
[![codecov](https://codecov.io/gh/dk-open/ring/graph/badge.svg?token=0UU4GMK24V)](https://codecov.io/gh/dk-open/ring)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dk-open/ring)](https://github.com/dk-open/ring)
[![GitHub release](https://img.shields.io/github/release/dk-open/ring.svg)](https://github.com/dk-open/ring/releases)
[![GitHub issues](https://img.shields.io/github/issues/dk-open/ring)](https://github.com/dk-open/ring/issues)

## Overview

`ringqueue` is a blazing-fast, lock-free ring buffer (queue) implementation for Go, inspired by Disruptor and optimized for low-latency, high-throughput scenarios. Designed for HFT, real-time systems, and any case where garbage-free, wait-free operations are required.

- No dependencies
- Generic (Go 1.18+)
- CAS-based head/tail, false sharing padding
- ABA protection
- Busy-spin and adaptive exponential backoff
- Supports multiple consumers (SPMC/MPMC)


## Features
- High-performance, lock-free queue with atomic operations
- Efficient memory usage (single buffer allocation)
- Dual-Counter System
- Two-phase dequeue protocol for safe concurrent access
- ABA protection
- Wait-free progress
- Configurable for concurrent readers
- Busy-spin + adaptive backoff to minimize latency spikes
- Padding to avoid false sharing on modern CPUs

##  When to Use
- High-frequency trading engines
- Real-time data pipelines
- Metrics/event aggregation
- Anywhere you want the speed of a ring buffer without GC churn or locks

## Example Usage

```go
package main

import (
	"context"
	"fmt"
	"github.com/dk-open/ring"
)

func main() {
	ctx := context.Background()
    r := ring.New[int](10) // Create a new ring buffer with a capacity of 10

	go func() {
		// Enqueue some items
		for i := 0; i < 10; i++ {
			if err := r.Enqueue(ctx, i); err != nil {
				fmt.Printf("Error enqueuing item %d: %v\n", i, err)
			} else {
				fmt.Printf("Enqueued item %d\n", i)
			}
		}
	}()

    // Dequeue items
    for i := 0; i < 10; i++ {
        item, err := r.Dequeue(ctx)
        if err != nil {
            fmt.Printf("Error dequeuing item: %v\n", err)
            continue
        }
        fmt.Printf("Dequeued item: %d\n", item)
    }
}

```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.

