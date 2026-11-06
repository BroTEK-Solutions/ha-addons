package main

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestProbeInterruptedByCollectionBudgetIsNotAFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	result := checkEndpoint(ctx, func(attempt context.Context, _, _ string) (net.Conn, error) {
		<-attempt.Done()
		return nil, attempt.Err()
	}, "slow.example:443")
	if result.Reachable || !strings.Contains(result.Detail, "not tested") {
		t.Fatalf("a shortened probe cannot establish unreachability: %+v", result)
	}
}
