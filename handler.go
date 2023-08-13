// Package slogproto provides a protocol buffer definition for the slog
// format (golang.org/x/exp/slog).
//
// It attempts to have minimial dependencies and minimize memory allocations.
package slogproto

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// recordPool is a pool of records used to reduce allocations when handling
// log records in the [github.com/picatz/slogproto.Handler.Handle] method.
var recordPool = sync.Pool{
	New: func() interface{} {
		return new(Record)
	},
}

// Handler implements the slog.Handler interface and writes the log record
// to the writer as a protocol buffer encoded struct containing the log
// record, including the levem, message and attributes.
type Handler struct {
	w     io.Writer
	group string
	attrs []slog.Attr
	level slog.Level
}

// NewHandler returns a new Handler that writes to the writer.
//
// # Example
//
//	h := slogproto.NewHandler(os.Stdout)
func NewHandler(w io.Writer) *Handler {
	return &Handler{
		w: w,
	}
}

// Enabled returns true if the level is enabled for the handler.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return level <= h.level
}

// Handle writes the log record to the writer as a protocol buffer encoded
// struct containing the log record, including the level, message and attributes.
//
// It will only be called when Enabled returns true.
// The Context argument is as for Enabled.
// It is present solely to provide Handlers access to the context's values.
// Canceling the context should not affect record processing.
// (Among other things, log messages may be necessary to debug a
// cancellation-related problem.)
//
// Handle methods that produce output should observe the following rules:
//   - If r.Time is the zero time, ignore the time.
//   - If r.PC is zero, ignore it.
//   - Attr's values should be resolved.
//   - If an Attr's key and value are both the zero value, ignore the Attr.
//     This can be tested with attr.Equal(Attr{}).
//   - If a group's key is empty, inline the group's Attrs.
//   - If a group has no Attrs (even if it has a non-empty key),
//     ignore it.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// If the r.PC is zero or the r.Time is the zero time, ignore the record.
	if r.PC == 0 || r.Time.IsZero() {
		return nil
	}

	// Get a protobuf record from the pool.
	pbr := recordPool.Get().(*Record)
	defer func() {
		// reset the record
		pbr.Reset()
		// return the record to the pool
		recordPool.Put(pbr)
	}()

	// Fill the protobuf record.
	if err := h.fillProtobufRecord(pbr, &r); err != nil {
		return err
	}

	// Marshal the protobuf record.
	b, err := proto.Marshal(pbr)
	if err != nil {
		return err
	}

	// fmt.Printf("proto record %d: %v\n\n", len(b), pbr)

	// Write the length of the struct to the writer
	// so that the reader knows how much to read.
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(b)))
	if _, err := h.w.Write(buf); err != nil {
		return err
	}

	// Write the struct to the writer.
	_, err = h.w.Write(b)
	return err
}

// WithAttrs returns the handler unchanged, as it does not support attributes.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(h.attrs, attrs...)
	return h
}

// WithGroup returns the handler unchanged, as it does not support groups.
func (h *Handler) WithGroup(name string) slog.Handler {
	h.group = name
	return h
}

// getValue converts a slog.Value to a slogproto Value.
func getValue(group string, value slog.Value) (*Value, error) {
	switch value.Kind() {
	case slog.KindAny:
		b, err := json.Marshal(value.Any())
		if err != nil {
			return nil, fmt.Errorf("slogproto: error marshaling slog.Value as JSON: %w", err)
		}
		return &Value{
			Kind: &Value_Any{
				Any: &anypb.Any{
					TypeUrl: fmt.Sprintf("go/slog/%T", value.Any()),
					Value:   b,
				},
			},
		}, nil
	case slog.KindBool:
		return &Value{
			Kind: &Value_Bool{
				Bool: value.Bool(),
			},
		}, nil
	case slog.KindDuration:
		return &Value{
			Kind: &Value_Duration{
				Duration: durationpb.New(value.Duration()),
			},
		}, nil
	case slog.KindFloat64:
		return &Value{
			Kind: &Value_Float{
				Float: value.Float64(),
			},
		}, nil
	case slog.KindInt64:
		return &Value{
			Kind: &Value_Int{
				Int: value.Int64(),
			},
		}, nil
	case slog.KindString:
		return &Value{
			Kind: &Value_String_{
				String_: value.String(),
			},
		}, nil
	case slog.KindTime:
		return &Value{
			Kind: &Value_Time{
				Time: timestamppb.New(value.Time()),
			},
		}, nil
	case slog.KindUint64:
		return &Value{
			Kind: &Value_Uint{
				Uint: value.Uint64(),
			},
		}, nil
	case slog.KindGroup:
		attrs := value.Group()

		g := &Group{
			Name:  group,
			Attrs: make(map[string]*Value, len(attrs)),
		}

		for i := 0; i < len(attrs); i++ {
			v, err := getValue(attrs[i].Key, attrs[i].Value)
			if err != nil {
				return nil, err
			}
			g.Attrs[attrs[i].Key] = v
		}

		// Return nil if there are no attributes.
		if len(g.Attrs) == 0 {
			return nil, nil
		}

		return &Value{
			Kind: &Value_Group{
				Group: g,
			},
		}, nil
	case slog.KindLogValuer:
		return getValue(group, value.LogValuer().LogValue())
	default:
		return nil, fmt.Errorf("unknown value kind: %v", value.Kind())
	}
}

// convertLevel converts a slog.Level to a slogproto Level.
func convertLevel(level slog.Level) Level {
	switch level {
	case slog.LevelInfo:
		return Level_Info
	case slog.LevelWarn:
		return Level_Warn
	case slog.LevelError:
		return Level_Error
	case slog.LevelDebug:
		return Level_Debug
	default:
		return Level_Info
	}
}

// fillProtobufRecord fills a slogproto Record with the values from a slog Record.
func (h *Handler) fillProtobufRecord(pbr *Record, slr *slog.Record) error {
	pbr.Level = convertLevel(slr.Level)
	pbr.Message = slr.Message
	pbr.Time = timestamppb.New(slr.Time)
	pbr.Attrs = make(map[string]*Value, slr.NumAttrs())

	currentAttrs := pbr.Attrs

	if h.group != "" {
		pbr.Attrs[h.group] = &Value{
			Kind: &Value_Group{
				Group: &Group{
					Name:  h.group,
					Attrs: map[string]*Value{},
				},
			},
		}
		currentAttrs = pbr.Attrs[h.group].GetGroup().Attrs
	}

	var err error
	slr.Attrs(func(a slog.Attr) bool {
		var v *Value

		v, err = getValue(a.Key, a.Value)
		if err != nil {
			return false
		}

		if v == nil {
			return true
		}

		// If value is a group, and group name is empty, inline the group's Attrs.
		if v.GetGroup() != nil && v.GetGroup().Name == "" {
			for k, v := range v.GetGroup().Attrs {
				currentAttrs[k] = v
			}
			return true
		}

		currentAttrs[a.Key] = v
		return true
	})

	// If there are any attrs that are in the handler, but not in the record,
	// then add them to the record.
	for _, attr := range h.attrs {
		if _, ok := currentAttrs[attr.Key]; !ok {
			v, err := getValue(attr.Key, attr.Value)
			if err != nil {
				return err
			}
			currentAttrs[attr.Key] = v
		}
	}

	if err != nil {
		return fmt.Errorf("slogproto: error converting slog.Value to anypb.Any: %w", err)
	}

	return nil
}
