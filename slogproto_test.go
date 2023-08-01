package slogproto_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/picatz/slogproto"
	"golang.org/x/exp/slog"
	"golang.org/x/exp/slog/slogtest"
)

func parseLogEntries(t *testing.T, data []byte) []map[string]any {
	records := []map[string]any{}

	slogproto.Read(context.Background(), bytes.NewReader(data), func(r *slog.Record) bool {
		record := map[string]any{
			"level":   r.Level,
			"message": r.Message,
			"time":    r.Time,
			"attrs":   r.Attrs,
		}
		records = append(records, record)
		return true
	})

	return records
}

func TestHandler(t *testing.T) {
	var buf bytes.Buffer

	// TODO: fix "got 2 results, want 14"
	err := slogtest.TestHandler(slogproto.NewHander(&buf), func() []map[string]any {
		return parseLogEntries(t, buf.Bytes())
	})
	if err != nil {
		t.Error(err)
	}
}

func ExampleHandler() {
	fh, err := os.OpenFile(filepath.Join(os.TempDir(), "test.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer fh.Close()

	logger := slog.New(slogproto.NewHander(fh))
	logger.Info("this is a test",
		slog.Group("test",
			slog.Int("test1", 1),
			slog.String("test2", "1"),
			slog.Float64("test3", 1.0),
		),
	)

	logger.Info("example", slog.Int("something", 1))
	// Output:
	//
}
