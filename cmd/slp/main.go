package main

// Simple CLI that reads protobuf messages from STDIN or a file and prints them
// to STDOUT in JSON format.

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"

	"github.com/google/cel-go/cel"
	"github.com/picatz/slogproto"
	"github.com/spf13/cobra"
)

var (
	filterFlag   string
	logLevelFlag string
)

func init() {
	rootCmd.Flags().StringVarP(&filterFlag, "filter", "f", "", "filter expression")
	rootCmd.Flags().StringVarP(&logLevelFlag, "log-level", "l", "info", "log level")
}

var rootCmd = &cobra.Command{
	Use:   "slp [file]",
	Short: "Slogproto Log Parser",
	Long:  `SLP (Slogproto Log Parser) is a simple CLI that reads protobuf messages from STDIN or a file and prints them to STDOUT in JSON format.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logLevel, err := cmd.Flags().GetString("log-level")
		if err != nil {
			return fmt.Errorf("error getting log level flag: %w", err)
		}

		var level slog.Level

		err = level.UnmarshalText([]byte(logLevel))
		if err != nil {
			return fmt.Errorf("error parsing log leve %q: %w", logLevel, err)
		}

		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		}))

		expr, err := cmd.Flags().GetString("filter")
		if err != nil {
			return fmt.Errorf("error getting filter flag: %w", err)
		}

		filterProg, err := compileFilter(expr)
		if err != nil {
			return fmt.Errorf("error compiling filter expression: %w", err)
		}

		var input io.Reader = cmd.InOrStdin()

		// Check if STDIN is a pipe or not to determine if we should read from a file
		// or from STDIN.
		if len(args) > 0 {
			file := args[0]

			// Open the file for reading.
			f, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			input = f
		}

		// Read the protobuf messages from the reader and write them to
		// STDOUT in JSON format. Only include records that match the filter
		// expression, if one was provided.
		err = slogproto.Read(context.Background(), input, func(r *slog.Record) bool {
			include, err := slogproto.EvalFilter(filterProg, r)
			if err != nil {
				logger.Error("error evaluating filter expression", "error", err)
				return false
			}

			if include {
				logger.Handler().Handle(context.Background(), *r)
			}

			return true
		})

		return err
	},
}

func compileFilter(expr string) (cel.Program, error) {
	filterProg, err := slogproto.CompileFilter(expr)
	if err != nil {
		return nil, fmt.Errorf("error compiling filter expression: %w", err)
	}

	return filterProg, nil
}

func main() {
	// Create a new context that is canceled when the user sends an interrupt signal.
	//
	// Allows for easy CTRL+C termination of the program.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Execute the root command.
	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
