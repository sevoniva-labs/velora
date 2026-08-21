package ratelimit

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestLocalLimiterBoundsKeyCardinality(t *testing.T) {
	l := New(nil)
	now := time.Now()
	for i := 0; i < maxLocalEntries+250; i++ {
		if ok, err := l.Allow(context.Background(), "ip:"+strconv.Itoa(i), 1, time.Hour, now); err != nil || !ok {
			t.Fatalf("Allow() = (%v, %v), want first attempt allowed", ok, err)
		}
	}
	if got := len(l.local); got > maxLocalEntries {
		t.Fatalf("local limiter retained %d entries, want <= %d", got, maxLocalEntries)
	}
}

func TestLocalLimiterExpiresKeysDuringAdmission(t *testing.T) {
	l := New(nil)
	start := time.Unix(100, 0)
	if ok, err := l.Allow(context.Background(), "expired", 1, time.Second, start); err != nil || !ok {
		t.Fatalf("initial Allow() = (%v, %v)", ok, err)
	}
	if ok, err := l.Allow(context.Background(), "expired", 1, time.Second, start.Add(2*time.Second)); err != nil || !ok {
		t.Fatalf("expired key was not admitted: (%v, %v)", ok, err)
	}
}
