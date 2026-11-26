package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"go.temporal.io/sdk/client"
)

func TestWorkerSmoke(t *testing.T) {
	address := os.Getenv("TEMPORAL_TEST_ADDRESS")
	if address == "" {
		t.Skip("TEMPORAL_TEST_ADDRESS is required for the Worker integration test")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	temporal, err := client.DialContext(ctx, client.Options{HostPort: address})
	if err != nil {
		t.Fatal(err)
	}
	defer temporal.Close()

	run, err := temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		TaskQueue:                "ai2workspace",
		WorkflowExecutionTimeout: 15 * time.Second,
	}, "workerSmoke")
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ Status string }
	if err := run.Get(ctx, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("got status %q, want ok", result.Status)
	}
}
