# slogproto

> [!WARNING] 
> This is an experimental module and is subject to change.

Go [`log/slog.Handler`](https://pkg.go.dev/log/slog#Handler) using [Protocol Buffers](https://protobuf.dev/). This can reduce the size of log messages when saving them to disk or sending them over the network, and can reduce the amount of time spent marshaling and unmarshaling log messages, at the cost of human readability. 

To enable interopability with other tools, the `slp` CLI can read protobuf encoded [`slog.Record`](https://pkg.go.dev/log/slog#Record)s from STDIN (or a file) and output them as JSON to STDOUT. Logs can be filtered using [CEL](https://github.com/google/cel-spec/blob/master/doc/langdef.md) expressions.

## Installation

For use with Go programs:

```console
$ go get -u -v github.com/picatz/slogproto@latest
```

To use the `slp` CLI:

```console
$ go install -v github.com/picatz/slogproto/cmd/slp@latest
```

## Usage

```go
package main

import (
	"log/slog"
	"os"

	"github.com/picatz/slogproto"
)

func main() {
	logger := slog.New(slogproto.NewHander(os.Stdout))

	logger.Info("example", slog.Int("something", 1))
}
```

Read from a program that produces slogproto formatted logs to STDOUT (like the example above): 

```console
$ go run main.go | slp
{"time":"2023-08-01T03:12:11.272826Z","level":"INFO","msg":"example","something":1}
```

Read from a file in slogproto format:

```console
$ slp output.log
{"time":"..","level":"...","msg":"...", ... }
{"time":"..","level":"...","msg":"...", ... }
{"time":"..","level":"...","msg":"...", ... }
...
```

> [!NOTE]
> Input to `slp` can be from STDIN or a file.

#### Filtering

The filter flag can be used to filter logs using a given expression. The expression is evaluated against the [`slog.Record`](https://pkg.go.dev/golang.org/x/exp/slog#Record) and must return a boolean value. For each log record that the expression evaluates as `true` will be output to STDOUT as JSON.

* `msg` is the message in the log record.
* `level` is the level in the log record.
* `time` is the timestamp in the log record.
* `attrs` is a map of all the attributes in the log record, not including the message, level, or time.

	```javascript
	attrs.something == 1
	```
	```javascript
	has(attrs.something) && attrs.something == 1
	```
	```javascript
	attrs.?something.orValue(0) == 1
	```

> [!IMPORTANT]
> Invalid access to an attribute will cause the filter to fail at evaluation time. Invalid expressions (which do not evaluate to a boolean) will be checked before reading the log records, and will cause the program to exit with an error message.

```console
$ slp --filter='has(attrs.something)' output.log
{"time":"2023-08-11T00:06:00.474782Z","level":"INFO","msg":"example","something":1}
```

```console
$ slp --filter='msg == "this is a test"' test.log
{"time":"2023-08-11T00:06:00.474033Z","level":"INFO","msg":"this is a test","test":{"test2":"1","test3":1,"test1":1}}
```

## File Format

The file format is a series of [delimited](https://developers.google.com/protocol-buffers/docs/techniques#streaming) [Protocol Buffer](https://developers.google.com/protocol-buffers) messages. Each message is prefixed with a 32-bit unsigned integer representing the size of the message. The message itself is a protobuf encoded [`slog.Record`](https://pkg.go.dev/golang.org/x/exp/slog#Record).

```console
╭────────────────────────────────────────────────────────────╮
│  Message Size  │  Protocol Buffer Message  │  ...  │  EOF  │
╰────────────────────────────────────────────────────────────╯
```

## Comparisons to Other Formats

Using the following record written 1024 times:

```json
{
	"time": $timestamp,
	"level": "INFO",
	"msg": "hello world",
	"i": $n
}
```

| Format   | GZIP (KB)  | Snappy (KB) | Zstandard (KB) |  Uncompressed (KB) |
|----------|------------|-------------|----------------|--------------------|
| Protobuf | 5.48       | 11.17       | 3.58           | 41.88              |
| JSON     | 5.79       | 9.59        | 5.04           | 86.81              |
| Text     | 2.93       | 7.66        | 1.31           | 69.92              |
