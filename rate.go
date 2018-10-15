package ratelimit

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mutex             *sync.Mutex
	tokens            int64
	tokensPerInterval int64
	burst             int64
	interval          time.Duration
	lastEvent         time.Time
}

func NewRateLimiter(rate time.Duration, burst int64) *RateLimiter {
	return NewRateLimiterPerInterval(
		rate,
		burst,
		1,
	)
}

func NewRateLimiterPerInterval(rate time.Duration, burst int64, tokensPerInterval int64) *RateLimiter {
	return NewRateLimiterPerIntervalInitialTokenValue(
		rate,
		burst,
		tokensPerInterval,
		0,
	)
}

// Can't set token > burst
func NewRateLimiterPerIntervalInitialTokenValue(rate time.Duration, burst int64, tokensPerInterval int64, token int64) *RateLimiter {
	if token > burst {
		return nil
	}
	return &RateLimiter{
		interval:          rate,
		burst:             burst,
		tokensPerInterval: tokensPerInterval,
		mutex:             &sync.Mutex{},
		tokens:            token,
		lastEvent:         time.Now(),
	}
}

func (r *RateLimiter) Consume() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.canProceed() {
		r.tokens += r.tokensPerInterval
	}

	r.advance(time.Now())
}

func (r *RateLimiter) canProceed() bool {
	return r.tokens < r.burst
}

func (r *RateLimiter) nextInterval(now time.Time) time.Duration {
	return r.lastEvent.Add(r.interval).Sub(now)
}

func (r *RateLimiter) advance(now time.Time) {
	for !r.lastEvent.Add(r.interval).After(now) {
		r.lastEvent = r.lastEvent.Add(r.interval)
		if r.tokens <= 0 {
			// Advance until current time.
			continue
		}
		r.tokens -= r.tokensPerInterval
	}
}

func (r *RateLimiter) Wait(ctx Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for !r.canProceed() {
		t := time.NewTimer(r.nextInterval(time.Now()))
		defer t.Stop()
		select {
		case <-t.C:
			r.advance(time.Now())
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

type Context interface {
	Done() <-chan struct{}
	Err() error
}
