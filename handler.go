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
	attrs []slog.Attr
	level slog.Level

	parent    *Handler
	group     *Value_Group
	groupName string
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
	// If the r.PC is zero ignore the record.
	if r.PC == 0 {
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

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments.
//
// The Handler owns the slice: it may retain, modify or discard it.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// New handler
	newHandler := &Handler{
		w:      h.w,
		level:  h.level,
		attrs:  h.attrs,
		parent: h,
	}

	// If in a group, add the attributes to the group.
	if h.group != nil {
		for i := 0; i < len(attrs); i++ {
			v, err := getValue(attrs[i].Key, attrs[i].Value)
			if err != nil {
				panic(err)
			}
			h.group.Attrs[attrs[i].Key] = v
		}

		// Set the new handler's group to the existing group.
		newHandler.group = h.group
		newHandler.groupName = h.groupName
	} else {
		// Otherwise, add the attributes to the handler.
		newHandler.attrs = append(newHandler.attrs, attrs...)
	}

	return newHandler
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups.
//
// The keys of all subsequent attributes, whether added by With or in a
// Record, should be qualified by the sequence of group names.
//
// How this qualification happens is up to the Handler, so long as
// this Handler's attribute keys differ from those of another Handler
// with a different sequence of group names.
//
// A Handler should treat WithGroup as starting a Group of Attrs that ends
// at the end of the log event. That is,
//
//	logger.WithGroup("s").LogAttrs(level, msg, slog.Int("a", 1), slog.Int("b", 2))
//
// should behave like
//
//	logger.LogAttrs(level, msg, slog.Group("s", slog.Int("a", 1), slog.Int("b", 2)))
//
// If the name is empty, WithGroup returns the receiver.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	// Create a copy of the attributes.
	attrsCopy := make([]slog.Attr, len(h.attrs))
	copy(attrsCopy, h.attrs)

	// New handler
	newHandler := &Handler{
		w:         h.w,
		attrs:     attrsCopy,
		level:     h.level,
		parent:    h,
		groupName: name,
	}

	// New group
	newGroup := &Value_Group{
		Attrs: make(map[string]*Value),
	}

	// If there is already a group, embed the new group in the existing group.
	if h.parent != nil && h.parent.group != nil {
		h.parent.group.Attrs[name] = &Value{
			Kind: &Value_Group_{
				Group: newGroup,
			},
		}

		// Set the new handler's group to the existing group.
		newHandler.group = newGroup
	} else {
		// Otherwise, set the new group as the handler's group.
		newHandler.group = newGroup
	}

	return newHandler
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

		g := &Value_Group{
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
			Kind: &Value_Group_{
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
	pbr.Attrs = make(map[string]*Value, slr.NumAttrs()+len(h.attrs))

	timeIsZero := slr.Time.IsZero()

	if !timeIsZero {
		pbr.Time = timestamppb.New(slr.Time)
	}

	// Add the handler's attributes.
	for i := 0; i < len(h.attrs); i++ {
		// If the key is empty, skip it.
		if h.attrs[i].Key == "" {
			continue
		}

		v, err := getValue(h.attrs[i].Key, h.attrs[i].Value)
		if err != nil {
			return err
		}
		pbr.Attrs[h.attrs[i].Key] = v
	}

	// Add the record's attributes.
	var err error
	slr.Attrs(func(attr slog.Attr) bool {
		// If the key is empty, skip it, unless it is a group.
		// If it is a group, we want to add it to the record.
		if attr.Key == "" {
			if attr.Value.Kind() == slog.KindGroup {
				var v *Value
				v, err = getValue(attr.Key, attr.Value)
				if err != nil {
					return false
				}

				// Skip the empty group.
				if v == nil {
					return true
				}

				for k, v := range v.GetGroup().Attrs {
					pbr.Attrs[k] = v
				}
				return true
			}
			return true
		}

		var v *Value
		v, err = getValue(attr.Key, attr.Value)
		if err != nil {
			return false
		}

		// Skip the empty group.
		if v == nil {
			return true
		}

		if h.group != nil {
			h.group.Attrs[attr.Key] = v
		} else {
			pbr.Attrs[attr.Key] = v
		}

		return true
	})
	if err != nil {
		return err
	}

	// Add the group to the record.
	if h.group != nil {
		// If there is a parent, add the group to the parent.
		if h.parent != nil && h.parent.group != nil {
			h.parent.group.Attrs[h.groupName] = &Value{
				Kind: &Value_Group_{
					Group: h.group,
				},
			}
		} else {
			pbr.Attrs[h.groupName] = &Value{
				Kind: &Value_Group_{
					Group: h.group,
				},
			}
		}
	}

	// Add the parent's group to the record.
	if h.parent != nil && h.parent.group != nil {
		pbr.Attrs[h.parent.groupName] = &Value{
			Kind: &Value_Group_{
				Group: h.parent.group,
			},
		}
	}

	return nil
}
