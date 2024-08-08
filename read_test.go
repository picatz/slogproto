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

	logger := slog.New(slogproto.NewHandler(fh))

	for i := 0; i < 100; i++ {
		logger.Info("this is a test", "test", i)
	}

	err = fh.Close()
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	fh, err = os.Open(filepath.Join(tmpDir, "test.log"))
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	count := 0

	err = slogproto.Read(context.Background(), fh, func(r *slog.Record) bool {
		count++

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

			return true
		})

		return true
	})
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}

	if count != 100 {
		t.Fatalf("expected 100 records, but got: %d", count)
	}
}
