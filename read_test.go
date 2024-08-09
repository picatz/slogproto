package slogproto_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/picatz/slogproto"
)

func setupTestLog(t *testing.T, recordsCount int) *os.File {
	t.Helper()

	tmpDir := t.TempDir()

	fh, err := os.Create(filepath.Join(tmpDir, "test.log"))
	if err != nil {
		t.Fatalf("failed to create test log file: %v", err)
	}

	logger := slog.New(slogproto.NewHandler(fh))

	for i := 0; i < recordsCount; i++ {
		logger.Info("this is a test", "test", i)
	}

	_, err = fh.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}
	t.Cleanup(func() {
		err := fh.Close()
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}
	})

	return fh
}

func TestRead(t *testing.T) {
	numberOfRecords := 100

	fh := setupTestLog(t, numberOfRecords)

	count := 0

	err := slogproto.Read(context.Background(), fh, func(r *slog.Record) bool {
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

	if count != numberOfRecords {
		t.Fatalf("expected 100 records, but got: %d", count)
	}
}
