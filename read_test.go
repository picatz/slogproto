package slogproto_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/picatz/slogproto"
	"golang.org/x/exp/slog"
)

func TestRead(t *testing.T) {
	tmpDir := t.TempDir()

	fh, err := os.OpenFile(filepath.Join(tmpDir, "test.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	logger := slog.New(slogproto.NewHander(fh))
	logger.Info("this is a test",
		slog.Group("test",
			slog.Int("test", 1),
			slog.String("test2", "test"),
			slog.Float64("test3", 1.0),
		),
	)

	fh.Close()

	fh, err = os.Open(filepath.Join(tmpDir, "test.log"))
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	err = slogproto.Read(context.Background(), fh, func(r *slog.Record) bool {
		if r.Message != "this is a test" {
			t.Fatalf("expected message to be 'this is a test', but got: %s", r.Message)
		}

		if r.NumAttrs() != 1 {
			t.Fatalf("expected 1 attribute, but got: %d", r.NumAttrs())
		}

		r.Attrs(func(a slog.Attr) bool {
			if a.Key != "test" {
				t.Fatalf("expected attribute key to be 'test', but got: %s", a.Key)
			}

			if len(a.Value.Group()) != 3 {
				t.Fatalf("expected 3 attributes in group, but got: %d", len(a.Value.Group()))
			}

			return true
		})

		return true
	})
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}
}
