package httplog

import (
	"context"
	"log/slog"
	"testing"
)

func TestShouldLogRecoversHookPanic(t *testing.T) {
	keep := ShouldLog(NewSuccessSampler(1), func(DecisionMeta) bool {
		panic("hook failed")
	}, DecisionMeta{Event: "complete"}, true)
	if !keep {
		t.Fatal("logging hook panic must not drop application logs")
	}
}

func TestLogEventRecoversHandlerPanic(t *testing.T) {
	log := slog.New(panicHandler{})
	LogEvent(log, context.Background(), Event{Message: "test", Level: slog.LevelInfo}, true)
}

type panicHandler struct{}

func (panicHandler) Enabled(context.Context, slog.Level) bool  { return true }
func (panicHandler) Handle(context.Context, slog.Record) error { panic("handler failed") }
func (panicHandler) WithAttrs([]slog.Attr) slog.Handler        { return panicHandler{} }
func (panicHandler) WithGroup(string) slog.Handler             { return panicHandler{} }
