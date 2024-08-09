package slogproto

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"

	"google.golang.org/protobuf/proto"
)

// Read reads protobuf encoded slog records from the reader and calls the
// provided function for each record. If the function returns false, the
// iteration is stopped.
//
// If the context is canceled, the iteration is stopped and the error is
// returned. If the reader returns an error, the error is returned.
func Read(ctx context.Context, r io.Reader, fn func(r *slog.Record) bool) error {
	// Create a new scanner to read from the reader.
	scanner := bufio.NewScanner(r)

	// Iterate over content from the scanner, which contains
	// protobuf encoded messages in binary format, which cannot be split
	// by line.
	//
	//
	// The file format is a series of [delimited](https://developers.google.com/protocol-buffers/docs/techniques#streaming)
	// [Protocol Buffer](https://developers.google.com/protocol-buffers) messages. Each message is prefixed
	// with a 32-bit unsigned integer representing the size of the message. The message
	// itself is a protobuf encoded [`slog.Record`](https://pkg.go.dev/golang.org/x/exp/slog#Record).
	//
	// ╭────────────────────────────────────────────────────────────╮
	// │  Message Size  │  Protocol Buffer Message  │  ...  │  EOF  │
	// ╰────────────────────────────────────────────────────────────╯
	//
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Check context.
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}

		// If we're at the end of the file, return 0, nil, nil.
		if atEOF {
			return 0, nil, nil
		}

		// Check if we have enough data to read the message length.
		if len(data) < 4 {
			return 0, nil, nil
		}

		// Get the length of the message (first 4 bytes).
		size := binary.LittleEndian.Uint32(data[:4])

		// Check if we have enough data to read the message.
		if len(data) < int(size)+4 {
			return 0, nil, nil
		}

		// Return the length of the message and the message itself.
		return int(size) + 4, data[4 : int(size)+4], nil
	})

	for scanner.Scan() && ctx.Err() == nil {
		// Create a new pbRecord.
		pbRecord := &Record{}

		// Unmarshal the line into the record.
		err := proto.Unmarshal(scanner.Bytes(), pbRecord)
		if err != nil {
			return fmt.Errorf("error unmarshaling record: %w", err)
		}

		attrs := make([]slog.Attr, 0, len(pbRecord.Attrs))
		for k, v := range pbRecord.Attrs {
			// Skip empty keys.
			if k == "" {
				continue
			}

			v, err := fromPBValue(v)
			if err != nil {
				return fmt.Errorf("error converting value: %w", err)
			}

			attr := slog.Attr{
				Key:   k,
				Value: v,
			}

			attrs = append(attrs, attr)
		}

		record := slog.NewRecord(pbRecord.Time.AsTime(), fromPBLevel(pbRecord.Level), pbRecord.Message, 1)
		record.AddAttrs(attrs...)

		ok := fn(&record)
		if !ok {
			break
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning input: %w", err)
	}

	return nil
}

func fromPBLevel(l Level) slog.Level {
	switch l {
	case Level_Info:
		return slog.LevelInfo
	case Level_Warn:
		return slog.LevelWarn
	case Level_Error:
		return slog.LevelError
	case Level_Debug:
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

func fromPBValue(v *Value) (slog.Value, error) {
	switch v.Kind.(type) {
	case *Value_Bool:
		return slog.BoolValue(v.GetBool()), nil
	case *Value_Float:
		return slog.Float64Value(v.GetFloat()), nil
	case *Value_Int:
		return slog.IntValue(int(v.GetInt())), nil
	case *Value_String_:
		return slog.StringValue(v.GetString_()), nil
	case *Value_Time:
		return slog.TimeValue(v.GetTime().AsTime()), nil
	case *Value_Duration:
		return slog.DurationValue(v.GetDuration().AsDuration()), nil
	case *Value_Uint:
		return slog.Uint64Value(uint64(v.GetUint())), nil
	case *Value_Any:
		return slog.AnyValue(v.GetAny()), nil
	case *Value_Group_:
		attrs := make([]slog.Attr, 0, len(v.GetGroup().GetAttrs()))

		for k, v := range v.GetGroup().GetAttrs() {
			v, err := fromPBValue(v)
			if err != nil {
				return slog.Value{}, fmt.Errorf("error converting nested value: %w", err)
			}

			attr := slog.Attr{
				Key:   k,
				Value: v,
			}

			attrs = append(attrs, attr)
		}

		return slog.GroupValue(attrs...), nil
	case nil:
		return slog.Value{}, nil
	default:
		return slog.Value{}, fmt.Errorf("unsupported value type: %T", v.Kind)
	}
}
