package elastic

import (
	"io"
	"testing"

	"github.com/budistwn15/go-obskit/logger"
)

func BenchmarkLoggerWithElasticDisabled(b *testing.B) {
	mw := NewMiddleware(DefaultConfig())
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      io.Discard,
		Middlewares: []logger.HandlerMiddleware{mw.LoggerMiddleware()},
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("bench", "i", i)
	}
}
