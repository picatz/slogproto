package main

// Simple CLI that reads protobuf messages from STDIN or a file and prints them
// to STDOUT in JSON format.

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/picatz/slogproto"
	"golang.org/x/exp/slog"
)

func filterExprProgram(filterExpr string) (cel.Program, error) {
	// Create a CEL environment.
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("msg", decls.String),
			decls.NewVar("level", decls.Int),
			decls.NewVar("time", decls.Timestamp),
			decls.NewVar("attrs", decls.NewMapType(decls.String, decls.Any)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating CEL environment: %s", err)
	}

	// Parse the expression.
	ast, iss := env.Compile(filterExpr)
	if iss.Err() != nil {
		return nil, fmt.Errorf("parse error: %s", iss.Err())
	}

	// Check the type of the expression.
	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("type-check error: %s", issues.Err())
	}

	if checked.OutputType().DeclaredTypeName() != "bool" {
		return nil, fmt.Errorf("invalid filter expression output type: %s", checked.OutputType().DeclaredTypeName())
	}

	// Return the program.
	prog, err := env.Program(checked)
	if err != nil {
		return nil, fmt.Errorf("program construction error: %s", err)
	}

	return prog, nil
}

func main() {
	var filterProg cel.Program

	filterExpr := flag.String("filter", "", "filter expression")

	flag.Parse()

	// If the filter is not empty, then compile it.
	if filterExpr != nil && *filterExpr != "" {
		var err error
		filterProg, err = filterExprProgram(*filterExpr)
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
	err := slogproto.Read(context.Background(), r, func(r *slog.Record) bool {
		if filterProg != nil {
			attrsMap := make(map[string]any, r.NumAttrs())

			r.Attrs(func(a slog.Attr) bool {
				attrsMap[a.Key] = a.Value.Any()
				return true
			})

			out, _, err := filterProg.Eval(map[string]any{
				"msg":   r.Message,
				"level": r.Level,
				"time":  r.Time,
				"attrs": attrsMap,
			})
			if err != nil {
				panic(fmt.Errorf("error evaluating filter expression: %s", err))
			}
			v, ok := out.Value().(bool)
			if !ok {
				panic(fmt.Sprintf("invalid filter expr output value type: %T", out.Value()))
			}
			if !v {
				return true
			}
			// Print the record.
			logger.Handler().Handle(context.Background(), *r)
		} else {
			// Print the record.
			logger.Handler().Handle(context.Background(), *r)
		}
		return true
	})
	if err != nil {
		logger.Error("error reading file", "error", err)
		os.Exit(1)
	}
}
