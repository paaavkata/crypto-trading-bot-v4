
package kucoin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	publicLimiter  *rate.Limiter
	privateLimiter *rate.Limiter
	mutex          sync.RWMutex
}

// KuCoin rate limits:
// Public endpoints: 100 requests per 10 seconds
// Private endpoints: 45 requests per 10 seconds
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		publicLimiter:  rate.NewLimiter(rate.Every(100*time.Millisecond), 10), // 10 requests per second
		privateLimiter: rate.NewLimiter(rate.Every(222*time.Millisecond), 4),  // 4.5 requests per second
	}
}

func (rl *RateLimiter) WaitForPublic() error {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	return rl.publicLimiter.Wait(ctx)
}

func (rl *RateLimiter) WaitForPrivate() error {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	return rl.privateLimiter.Wait(ctx)
}

func (rl *RateLimiter) CanMakePublicRequest() bool {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	
	return rl.publicLimiter.Allow()
}

func (rl *RateLimiter) CanMakePrivateRequest() bool {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	
	return rl.privateLimiter.Allow()
}

// UpdateLimits allows dynamic adjustment of rate limits based on API responses
func (rl *RateLimiter) UpdateLimits(publicRate, privateRate float64) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	if publicRate > 0 {
		rl.publicLimiter.SetLimit(rate.Limit(publicRate))
	}
	if privateRate > 0 {
		rl.privateLimiter.SetLimit(rate.Limit(privateRate))
	}
}
