package slogproto_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/picatz/slogproto"
)

func TestFilter(t *testing.T) {
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "this is a test", 1)
	record.AddAttrs(slog.Bool("test", true))
	record.AddAttrs(slog.Int("number", 42))
	record.AddAttrs(slog.String("name", "picatz"))
	record.AddAttrs(slog.Float64("pi", 3.14159))

	t.Run("match all", func(t *testing.T) {
		prog, err := slogproto.CompileFilter(`level == "INFO" && msg == "this is a test" && attrs.test == true && attrs.number == 42 && attrs.name == "picatz" && attrs.pi == 3.14159`)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}

		matched, err := slogproto.EvalFilter(prog, &record)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}

		if !matched {
			t.Fatalf("expected matched to be true")
		}
	})

	t.Run("match none", func(t *testing.T) {
		prog, err := slogproto.CompileFilter(`level == "ERROR"`)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}

		matched, err := slogproto.EvalFilter(prog, &record)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}

		if matched {
			t.Fatalf("expected matched to be false")
		}
	})

	t.Run("match some", func(t *testing.T) {
		prog, err := slogproto.CompileFilter(`cel.bind(value, attrs.?missing.orValue("other"), value != "thing")`)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}

		matched, err := slogproto.EvalFilter(prog, &record)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}

		if !matched {
			t.Fatalf("expected matched to be true")
		}
	})
}
