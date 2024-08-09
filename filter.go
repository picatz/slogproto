package slogproto

import (
	"fmt"
	"log/slog"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
)

// CompileFilter compiles a filter expression into a program that can be evaluated
// against a slog record. The expression must evaluate to a boolean value: if it's
// true, the record should be included. It may reference the following variables:
//
//   - msg: string
//   - level: int
//   - time: timestamp
//   - attrs: map[string]any
//
// The expression may also reference any of the functions provided by the
// CEL standard library, as well as the following functions provided by
// the CEL extension libraries:
//
//   - strings
//   - math
//   - encoders
//   - sets
//   - lists
//   - bindings
//
// If the expression is invalid, an error is returned.
func CompileFilter(expr string) (cel.Program, error) {
	// Create a CEL environment.
	env, err := cel.NewEnv(
		cel.StdLib(),
		ext.Strings(),
		ext.Math(),
		ext.Encoders(),
		ext.Sets(),
		ext.Lists(),
		ext.Bindings(),
		cel.OptionalTypes(cel.OptionalTypesVersion(1)),
		cel.Variable("msg", cel.StringType),
		cel.Variable("level", cel.StringType),
		cel.Variable("time", cel.TimestampType),
		cel.Variable("attrs", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating CEL environment: %s", err)
	}

	// Parse the expression.
	ast, iss := env.Compile(expr)
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

	// Return the program that can be evaluated against a slog record.
	return prog, nil
}

// EvalFilter evaluates a filter program against a slog record. The record
// must be a map[string]any, and the program must have been compiled with
// CompileFilter. If the program is invalid, an error is returned.
//
// The record must contain the following keys:
//
//   - msg: string
//   - level: int
//   - time: timestamp
//   - attrs: map[string]any
func EvalFilter(prog cel.Program, r *slog.Record) (bool, error) {
	if prog == nil {
		return false, nil
	}

	attrsMap := make(map[string]any, r.NumAttrs())

	r.Attrs(func(a slog.Attr) bool {
		attrsMap[a.Key] = a.Value.Any()
		return true
	})

	// Evaluate the program.
	result, _, err := prog.Eval(map[string]any{
		"msg":   r.Message,
		"level": r.Level.String(),
		"time":  r.Time,
		"attrs": attrsMap,
	})
	if err != nil {
		return false, fmt.Errorf("error evaluating program: %s", err)
	}

	val, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("invalid filter expression output type: %T", result.Value())
	}

	// Return the result.
	return val, nil
}
