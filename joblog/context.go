package joblog

import "context"

type contextKey string

const (
	jobRunIDKey contextKey = "go-obskit/job_run_id"
	jobNameKey  contextKey = "go-obskit/job_name"
)

func WithJobRunID(ctx context.Context, runID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, jobRunIDKey, runID)
}

func JobRunID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(jobRunIDKey).(string)
	return v
}

func WithJobName(ctx context.Context, jobName string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, jobNameKey, jobName)
}

func JobName(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(jobNameKey).(string)
	return v
}
