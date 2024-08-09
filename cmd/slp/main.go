package main

// Simple CLI that reads protobuf messages from STDIN or a file and prints them
// to STDOUT in JSON format.

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"

	"github.com/google/cel-go/cel"
	"github.com/picatz/slogproto"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var filterProg cel.Program

	filterExpr := flag.String("filter", "", "filter expression")

	flag.Parse()

	// If the filter is not empty, then compile it.
	if filterExpr != nil && *filterExpr != "" {
		var err error
		filterProg, err = slogproto.CompileFilter(*filterExpr)
		if err != nil {
			panic(fmt.Errorf("error compiling filter expression: %s", err))
		}
	}

	// Create a new logger that writes to STDOUT in JSON format.
	//
	// This is also used to handle errors in this program.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	var r io.Reader

	// Check if STDIN is a pipe or not to determine if we should read from a file
	// or from STDIN.
	//
	// If STDIN is a pipe, read from it. Otherwise, read from the file specified
	// in the first argument.
	if stat, err := os.Stdin.Stat(); err != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		r = os.Stdin
	} else {
		if len(flag.Args()) < 1 {
			fmt.Println("missing file argument")
			os.Exit(1)
		}

		f, err := os.Open(flag.Args()[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer f.Close()

		r = f
	}

	// Read the protobuf messages from the reader and write them to S
	// TDOUT in JSON format.
	err := slogproto.Read(ctx, r, func(r *slog.Record) bool {
		include, err := slogproto.EvalFilter(filterProg, r)
		if err != nil {
			logger.Error("error evaluating filter expression", "expr", filterExpr, "error", err)
			return false
		}

		if include {
			logger.Handler().Handle(context.Background(), *r)
		}

		return true
	})
	if err != nil {
		logger.Error("error reading file", "error", err)
		os.Exit(1)
	}
}
