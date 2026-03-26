package joblog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/logger"
)

func parseLogs(t *testing.T, raw string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid json line: %v line=%s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return logger.New(
		logger.Config{
			ServiceName: "svc",
			Environment: "production",
			Output:      buf,
		},
	)
}

func TestStartGeneratesRunID(t *testing.T) {
	var buf bytes.Buffer
	ctx, run := Start(context.Background(), newTestLogger(&buf), Meta{JobName: "daily-sync"})
	if run == nil {
		t.Fatalf("run must not be nil")
	}
	if JobRunID(ctx) == "" {
		t.Fatalf("job_run_id should be generated")
	}
	if JobName(ctx) != "daily-sync" {
		t.Fatalf("job_name should propagate in context")
	}
}

func TestEndCompleteAndFail(t *testing.T) {
	var buf1 bytes.Buffer
	_, run1 := Start(context.Background(), newTestLogger(&buf1), Meta{JobName: "ok-job"})
	run1.End(nil)
	logs1 := parseLogs(t, buf1.String())
	last1 := logs1[len(logs1)-1]
	if last1["event"] != "job.completed" {
		t.Fatalf("expected job.completed event")
	}
	
	var buf2 bytes.Buffer
	_, run2 := Start(context.Background(), newTestLogger(&buf2), Meta{JobName: "fail-job"})
	run2.End(errors.New("boom"))
	logs2 := parseLogs(t, buf2.String())
	last2 := logs2[len(logs2)-1]
	if last2["event"] != "job.failed" {
		t.Fatalf("expected job.failed event")
	}
}

func TestFailAndCompleteMethods(t *testing.T) {
	var buf bytes.Buffer
	_, run := Start(context.Background(), newTestLogger(&buf), Meta{JobName: "job-x"})
	run.Fail(errors.New("x"))
	run.Complete()
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	if last["event"] != "job.failed" {
		t.Fatalf("expected failed terminal event")
	}
}

func TestRetry(t *testing.T) {
	var buf bytes.Buffer
	_, run := Start(context.Background(), newTestLogger(&buf), Meta{JobName: "retry-job"})
	run.Retry(RetryMeta{Attempt: 2, MaxAttempts: 5, Reason: "timeout"})
	logs := parseLogs(t, buf.String())
	found := false
	for _, l := range logs {
		if l["event"] == "job.retry" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected retry event")
	}
}

func TestCountUpdates(t *testing.T) {
	var buf bytes.Buffer
	_, run := Start(context.Background(), newTestLogger(&buf), Meta{JobName: "count-job"})
	run.AddProcessed(10)
	run.AddSucceeded(8)
	run.AddFailed(1)
	run.AddSkipped(1)
	c := run.Counts()
	if c.Processed != 10 || c.Succeeded != 8 || c.Failed != 1 || c.Skipped != 1 {
		t.Fatalf("unexpected counts: %+v", c)
	}
}

func TestContextPropagationAndCorrelationPreserve(t *testing.T) {
	var buf bytes.Buffer
	baseCtx := correlation.WithID(context.Background(), "corr-job-1")
	ctx, run := Start(baseCtx, newTestLogger(&buf), Meta{JobName: "ctx-job"})
	run.Complete()
	
	if correlation.ID(ctx) != "corr-job-1" {
		t.Fatalf("correlation id should be preserved")
	}
	
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	if last["correlation_id"] != "corr-job-1" {
		t.Fatalf("expected correlation_id in logs")
	}
}

func TestCustomJobFieldsRemainGeneric(t *testing.T) {
	var buf bytes.Buffer
	_, run := Start(
		context.Background(), newTestLogger(&buf), Meta{
			JobName: "generic-job",
			Fields: map[string]any{
				"tenant_id": "t-1",
				"mode":      "dry-run",
			},
		},
	)
	run.Complete()
	
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	fields, ok := last["job.fields"].(map[string]any)
	if !ok {
		t.Fatalf("expected job.fields object")
	}
	if fields["tenant_id"] != "t-1" || fields["mode"] != "dry-run" {
		t.Fatalf("unexpected job.fields payload: %v", fields)
	}
}

func TestSlowThresholdFields(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.SlowThreshold = 1 * time.Millisecond
	ctx, run := Start(context.Background(), newTestLogger(&buf), Meta{JobName: "slow-job"}, opts)
	_ = ctx
	time.Sleep(3 * time.Millisecond)
	run.Complete()
	
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	if slow, ok := last["slow"].(bool); !ok || !slow {
		t.Fatalf("expected slow=true, got=%v", last["slow"])
	}
	if _, ok := last["slow_threshold_ms"]; !ok {
		t.Fatalf("expected slow_threshold_ms")
	}
	if _, ok := last["threshold_ms"]; !ok {
		t.Fatalf("expected threshold_ms")
	}
}

func TestSuccessSamplingForCompleted(t *testing.T) {
	jobCompleteSampleCounter.Store(0)
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.SuccessSampleEvery = 3
	
	for i := 0; i < 2; i++ {
		_, run := Start(context.Background(), newTestLogger(&buf), Meta{JobName: "sample-job"}, opts)
		run.Complete()
	}
	logs := parseLogs(t, buf.String())
	completed := 0
	for _, l := range logs {
		if l["event"] == "job.completed" {
			completed++
		}
	}
	if completed != 0 {
		t.Fatalf("first two completed events should be sampled out")
	}
	
	_, run := Start(context.Background(), newTestLogger(&buf), Meta{JobName: "sample-job"}, opts)
	run.Complete()
	logs = parseLogs(t, buf.String())
	completed = 0
	for _, l := range logs {
		if l["event"] == "job.completed" {
			completed++
		}
	}
	if completed != 1 {
		t.Fatalf("third completed event should pass sampling, got=%d", completed)
	}
}

func TestSlowCompletedNotSampledOut(t *testing.T) {
	jobCompleteSampleCounter.Store(0)
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.SuccessSampleEvery = 1000
	opts.SlowThreshold = 1 * time.Millisecond
	_, run := Start(context.Background(), newTestLogger(&buf), Meta{JobName: "slow-keep-job"}, opts)
	time.Sleep(3 * time.Millisecond)
	run.Complete()
	
	logs := parseLogs(t, buf.String())
	foundSlowComplete := false
	for _, l := range logs {
		if l["event"] == "job.completed" {
			if slow, ok := l["slow"].(bool); ok && slow {
				foundSlowComplete = true
			}
		}
	}
	if !foundSlowComplete {
		t.Fatalf("slow completed event must not be sampled out")
	}
}
