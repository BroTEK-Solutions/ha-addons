package main

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// Execute the shipped rendering code with a minimal DOM, including refresh's
// response-to-render wiring. Node is part of the repository's test toolchain.
func TestDiagnosticsBrowserRendering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "node", "testdata/diagnostics.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("diagnostics rendering: %v\n%s", err, output)
	}
}
