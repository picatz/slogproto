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

// Handler implements the slog.Handler interface and writes the log record
// to the writer as a protocol buffer encoded struct containing the log
// record, including the levem, message and attributes.
type Handler struct {
	w     io.Writer
	group string
	attrs []slog.Attr
}

func NewHander(w io.Writer) *Handler {
	return &Handler{
		w: w,
	}
}

// Enabled returns true for all levels.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

var (
	recordPool = sync.Pool{
		New: func() interface{} {
			return new(Record)
		},
	}
)

// Handle writes the log record to the writer as a protocol buffer encoded
// struct containing the log record, including the level, message and attributes.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if r.PC == 0 || r.Time.IsZero() {
		return nil
	}

	// get a record from the pool
	pbr := recordPool.Get().(*Record)
	defer func() {
		// reset the record
		pbr.Reset()
		// return the record to the pool
		recordPool.Put(pbr)
	}()

	switch r.Level {
	case slog.LevelInfo:
		pbr.Level = Level_Info
	case slog.LevelWarn:
		pbr.Level = Level_Warn
	case slog.LevelError:
		pbr.Level = Level_Error
	case slog.LevelDebug:
		pbr.Level = Level_Debug
	default:
		return fmt.Errorf("slogproto: unsupported level: %v", r.Level)
	}

	pbr.Message = r.Message
	pbr.Time = timestamppb.New(r.Time)
	pbr.Attrs = make(map[string]*Value, r.NumAttrs())

	var err error
	r.Attrs(func(a slog.Attr) bool {
		var v *Value
		v, err = getValue(a.Key, a.Value)
		if err != nil {
			return false
		}
		pbr.Attrs[a.Key] = v
		return true
	})

	if err != nil {
		return fmt.Errorf("slogproto: error converting slog.Value to anypb.Any: %w", err)
	}

	// marshal the record
	b, err := proto.Marshal(pbr)
	if err != nil {
		return err
	}

	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(b)))
	if _, err := h.w.Write(buf); err != nil {
		return err
	}

	// write the struct to the writer
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
			// g.Attrs[i] = &Attr{
			// 	Key:   attrs[i].Key,
			// 	Value: v,
			// }
			g.Attrs[attrs[i].Key] = v
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
