package main

// Simple CLI that reads protobuf messages from STDIN or a file and prints them
// to STDOUT in JSON format.

import (
	"context"
	"io"
	"os"

	"github.com/picatz/slogproto"
	"golang.org/x/exp/slog"
)

func main() {
	// Create a new logger that writes to STDOUT in JSON format.
	//
	// This is also used to handle errors in this program.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Check if STDIN is a pipe or not to determine if we should read from a file
	// or from STDIN.
	stat, err := os.Stdin.Stat()
	if err != nil {
		logger.Error("error getting STDIN stat", "error", err)
	}

	// If STDIN is a pipe, read from it. Otherwise, read from the file specified
	// in the first argument.
	var r io.Reader
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		r = os.Stdin
	} else {
		if len(os.Args) < 2 {
			logger.Error("missing file argument")
			os.Exit(1)
		}

		f, err := os.Open(os.Args[1])
		if err != nil {
			logger.Error("error opening file", "error", err)
			os.Exit(1)
		}
		defer f.Close()

		r = f
	}

	// Read the protobuf messages from the reader and write them to S
	// TDOUT in JSON format.
	err = slogproto.Read(context.Background(), r, func(r *slog.Record) bool {
		logger.Handler().Handle(context.Background(), *r)
		return true
	})
	if err != nil {
		logger.Error("error reading file", "error", err)
		os.Exit(1)
	}
}
