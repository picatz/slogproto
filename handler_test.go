package slogproto_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/picatz/slogproto"
	"golang.org/x/exp/slog"
	"golang.org/x/exp/slog/slogtest"
)

func parseLogEntries(t *testing.T, data []byte) []map[string]any {
	records := []map[string]any{}

	err := slogproto.Read(context.Background(), bytes.NewReader(data), func(r *slog.Record) bool {
		record := map[string]any{
			slog.LevelKey:   r.Level,
			slog.MessageKey: r.Message,
			slog.TimeKey:    r.Time,
		}
		r.Attrs(func(a slog.Attr) bool {
			record[a.Key] = a.Value.Any()
			return true
		})
		records = append(records, record)
		return true
	})

	if err != nil {
		t.Error(err)
	}

	return records
}

func TestHandler(t *testing.T) {
	var logBuffer bytes.Buffer

	// TODO: fix "got 13 results, want 14"
	err := slogtest.TestHandler(slogproto.NewHandler(&logBuffer), func() []map[string]any {
		return parseLogEntries(t, logBuffer.Bytes())
	})

	if err != nil {
		t.Error(err)
	}
}

func ExampleNewHandler() {
	var logBuffer bytes.Buffer

	logger := slog.New(slogproto.NewHandler(&logBuffer))

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

func TestGroupAttrs(t *testing.T) {
	var logBuffer bytes.Buffer

	l := slog.New(slogproto.NewHandler(&logBuffer))

	l.Info("msg", "a", "b", slog.Group("", slog.String("c", "d")), "e", "f")

	records := parseLogEntries(t, logBuffer.Bytes())

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0]["a"] != "b" {
		t.Errorf("expected a=b, got a=%v", records[0]["a"])
	}

	if records[0]["e"] != "f" {
		t.Errorf("expected e=f, got e=%v", records[0]["e"])
	}

	if records[0]["c"] != "d" {
		t.Errorf("expected c=d, got c=%v", records[0]["c"])
	}

	t.Logf("record: %v", records[0])
}

func TestWithGroup(t *testing.T) {
	var logBuffer bytes.Buffer

	l := slog.New(slogproto.NewHandler(&logBuffer))

	l.WithGroup("G").Info("msg", "a", "b")

	records := parseLogEntries(t, logBuffer.Bytes())

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	// Should not have "a" without a group
	if records[0]["a"] != nil {
		t.Errorf("expected a=nil, got a=%v", records[0]["a"])
	}

	// Should have "a" with a group
	if records[0]["G"] == nil {
		t.Errorf("expected G to be non-nil")
	}

	gAttrs := records[0]["G"].([]slog.Attr)

	if len(gAttrs) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(gAttrs))
	}

	if gAttrs[0].Key != "a" {
		t.Errorf("expected a, got %v", gAttrs[0].Key)
	}

	if gAttrs[0].Value.String() != "b" {
		t.Errorf("expected b, got %v", gAttrs[0].Value)
	}

	t.Logf("record: %v", records[0])
}

func TestHandler_verbose_test_suite(t *testing.T) {
	t.Run("this test expects slog.TimeKey, slog.LevelKey and slog.MessageKey", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.Info("message")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0][slog.TimeKey] == nil {
			t.Errorf("expected %s to be non-nil", slog.TimeKey)
		}

		if records[0][slog.LevelKey] == nil {
			t.Errorf("expected %s to be non-nil", slog.LevelKey)
		}

		if records[0][slog.MessageKey] == nil {
			t.Errorf("expected %s to be non-nil", slog.MessageKey)
		}

		t.Logf("record: %v", records[0])
	})

	t.Run("a Handler should output attributes passed to the logging function", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.Info("message", "k", "v")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0]["k"] == nil {
			t.Errorf("expected k to be non-nil")
		}

		if records[0]["k"] != "v" {
			t.Errorf("expected k=v, got k=%v", records[0]["k"])
		}
	})

	t.Run("a Handler should ignore an empty Attr", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.Info("msg", "a", "b", "", nil, "c", "d")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0]["a"] != "b" {
			t.Errorf("expected a=b, got a=%v", records[0]["a"])
		}

		if records[0]["c"] != "d" {
			t.Errorf("expected c=d, got c=%v", records[0]["c"])
		}

		t.Logf("record: %v", records[0])
	})

	t.Run("a Handler should ignore a zero Record.Time", func(t *testing.T) {
		var logBuffer bytes.Buffer

		h := slogproto.NewHandler(&logBuffer)

		h.Handle(context.Background(), slog.Record{
			Time:    time.Time{},
			Level:   slog.LevelInfo,
			Message: "msg",
			PC:      1,
		})

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 0 {
			t.Fatalf("expected 0 records, got %d", len(records))
		}
	})

	t.Run("a Handler should include the attributes from the WithAttrs method", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.With("a", "b").Info("msg", "k", "v")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0]["a"] == nil {
			t.Errorf("expected a to be non-nil")
		}

		if records[0]["a"] != "b" {
			t.Errorf("expected a=b, got a=%v", records[0]["a"])
		}

		if records[0]["k"] == nil {
			t.Errorf("expected k to be non-nil")
		}

		if records[0]["k"] != "v" {
			t.Errorf("expected k=v, got k=%v", records[0]["k"])
		}

		if records[0][slog.MessageKey] != "msg" {
			t.Errorf("expected msg, got %v", records[0][slog.MessageKey])
		}

		t.Logf("record: %v", records[0])
	})

	t.Run("a Handler should handle Group attributes", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.Info("msg", "a", "b", slog.Group("G", slog.String("c", "d")), "e", "f")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0]["a"] != "b" {
			t.Errorf("expected a=b, got a=%v", records[0]["a"])
		}

		if records[0]["e"] != "f" {
			t.Errorf("expected e=f, got e=%v", records[0]["e"])
		}

		if records[0]["G"] == nil {
			t.Errorf("expected G to be non-nil")
		}

		gAttrs := records[0]["G"].([]slog.Attr)

		if len(gAttrs) != 1 {
			t.Fatalf("expected 1 attribute, got %d", len(gAttrs))
		}

		if gAttrs[0].Key != "c" {
			t.Errorf("expected c, got %v", gAttrs[0].Key)
		}

		if gAttrs[0].Value.String() != "d" {
			t.Errorf("expected d, got %v", gAttrs[0].Value)
		}
	})

	t.Run("a Handler should ignore an empty group", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.Info("msg", "a", "b", slog.Group("G"), "e", "f")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0]["a"] != "b" {
			t.Errorf("expected a=b, got a=%v", records[0]["a"])
		}

		if records[0]["e"] != "f" {
			t.Errorf("expected e=f, got e=%v", records[0]["e"])
		}

		if records[0]["G"] != nil {
			t.Errorf("expected G to be nil")
		}
	})

	t.Run("a Handler should handle the WithGroup method", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.WithGroup("G").Info("msg", "a", "b")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		// Should not have "a" without a group
		if records[0]["a"] != nil {
			t.Errorf("expected a=nil, got a=%v", records[0]["a"])
		}

		// Should have "a" with a group
		if records[0]["G"] == nil {
			t.Errorf("expected G to be non-nil")
		}

		gAttrs := records[0]["G"].([]slog.Attr)

		if len(gAttrs) != 1 {
			t.Fatalf("expected 1 attribute, got %d", len(gAttrs))
		}

		if gAttrs[0].Key != "a" {
			t.Errorf("expected a, got %v", gAttrs[0].Key)
		}

		if gAttrs[0].Value.String() != "b" {
			t.Errorf("expected b, got %v", gAttrs[0].Value)
		}
	})

	// TODO: Work in progress.
	t.Run("a Handler should handle multiple WithGroup and WithAttr calls", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.With("a", "b").WithGroup("G").With("c", "d").WithGroup("H").Info("msg", "e", "f")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		// Should have "a" attribute without a group
		if records[0]["a"] == nil {
			t.Errorf("expected a to be non-nil")
		}

		if records[0]["a"] != "b" {
			t.Errorf("expected a=b, got a=%v", records[0]["a"])
		}

		// Should have "c" attribute in "G" group
		if records[0]["G"] == nil {
			t.Errorf("expected G to be non-nil")
		}

		gAttrs := records[0]["G"].([]slog.Attr)

		if len(gAttrs) != 2 {
			t.Fatalf("expected 2 attribute, got %d", len(gAttrs))
		}

		for _, a := range gAttrs {
			if a.Key == "c" {
				if a.Value.String() != "d" {
					t.Errorf("expected d, got %v", a.Value)
				}
			}

			switch a.Key {
			case "c":
				if a.Value.String() != "d" {
					t.Errorf("expected d, got %v", a.Value)
				}
			case "H":
				hAttrs := a.Value.Group()
				if len(hAttrs) != 1 {
					t.Fatalf("expected 1 attribute, got %d", len(hAttrs))
				}

				if hAttrs[0].Key != "e" {
					t.Errorf("expected e, got %v", hAttrs[0].Key)
				}

				if hAttrs[0].Value.String() != "f" {
					t.Errorf("expected f, got %v", hAttrs[0].Value)
				}
			default:
				t.Errorf("unexpected attribute: %v", a)
			}
		}
	})

	t.Run("a Handler should call Resolve on attribute values", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.Info("msg", "k", &replace{"replaced"})

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		// should have k with value "replaced"
		if records[0]["k"] != "replaced" {
			t.Errorf("expected k=replaced, got k=%v", records[0]["k"])
		}
	})

	t.Run("a Handler should call Resolve on attribute values in groups", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l.Info("msg",
			slog.Group("G",
				slog.String("a", "v1"),
				slog.Any("b", &replace{"v2"})))

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		// G should have a with value "v1", and b with value "v2"
		if records[0]["G"] == nil {
			t.Errorf("expected G to be non-nil")
		}

		gAttrs := records[0]["G"].([]slog.Attr)

		if len(gAttrs) != 2 {
			t.Fatalf("expected 2 attribute, got %d", len(gAttrs))
		}

		for _, a := range gAttrs {
			switch a.Key {
			case "a":
				if a.Value.String() != "v1" {
					t.Errorf("expected v1, got %v", a.Value)
				}
			case "b":
				if a.Value.String() != "v2" {
					t.Errorf("expected v2, got %v", a.Value)
				}
			default:
				t.Errorf("unexpected attribute: %v", a)
			}
		}
	})

	t.Run("a Handler should call Resolve on attribute values from WithAttrs", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l = l.With("k", &replace{"replaced"})
		l.Info("msg")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		// should have k with value "replaced"
		if records[0]["k"] != "replaced" {
			t.Errorf("expected k=replaced, got k=%v", records[0]["k"])
		}
	})

	t.Run("a Handler should call Resolve on attribute values in groups from WithAttrs", func(t *testing.T) {
		var logBuffer bytes.Buffer

		l := slog.New(slogproto.NewHandler(&logBuffer))

		l = l.With(slog.Group("G",
			slog.String("a", "v1"),
			slog.Any("b", &replace{"v2"})))
		l.Info("msg")

		records := parseLogEntries(t, logBuffer.Bytes())

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		// G should have a with value "v1", and b with value "v2"
		if records[0]["G"] == nil {
			t.Errorf("expected G to be non-nil")
		}

		gAttrs := records[0]["G"].([]slog.Attr)

		if len(gAttrs) != 2 {
			t.Fatalf("expected 2 attribute, got %d", len(gAttrs))
		}

		for _, a := range gAttrs {
			switch a.Key {
			case "a":
				if a.Value.String() != "v1" {
					t.Errorf("expected v1, got %v", a.Value)
				}
			case "b":
				if a.Value.String() != "v2" {
					t.Errorf("expected v2, got %v", a.Value)
				}
			default:
				t.Errorf("unexpected attribute: %v", a)
			}
		}
	})
}

type replace struct {
	v any
}

func (r *replace) LogValue() slog.Value { return slog.AnyValue(r.v) }

func (r *replace) String() string {
	return fmt.Sprintf("<replace(%v)>", r.v)
}
