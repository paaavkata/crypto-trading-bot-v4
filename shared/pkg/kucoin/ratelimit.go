package kucoin

import (
	"sync"
	"time"
)

type RateLimiter struct {
	requests  chan struct{}
	mu        sync.Mutex
	lastReset time.Time
}

func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	rl := &RateLimiter{
		requests:  make(chan struct{}, requestsPerSecond),
		lastReset: time.Now(),
	}

	// Fill the initial bucket
	for i := 0; i < requestsPerSecond; i++ {
		rl.requests <- struct{}{}
	}

	// Start the refill goroutine
	go rl.refillBucket(requestsPerSecond)

	return rl
}

func (rl *RateLimiter) refillBucket(requestsPerSecond int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		// Refill the bucket
		for i := 0; i < requestsPerSecond; i++ {
			select {
			case rl.requests <- struct{}{}:
			default:
				// Bucket is full
			}
		}
		rl.lastReset = time.Now()
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Wait() {
	<-rl.requests
}
