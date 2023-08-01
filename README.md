# slogproto

> **Warning**: this is an experimental module and is subject to change.

Go [`slog.Handler`](https://pkg.go.dev/golang.org/x/exp/slog#Handler) using [Protocol Buffers](https://protobuf.dev/). This can reduce the size of log messages when saving them to disk or sending them over the network, and can reduce the amount of time spent marshaling and unmarshaling log messages at the cost of human readability. To enable interopability with other tools, the `slp` CLI can read protobuf encoded [`slog.Record`](https://pkg.go.dev/golang.org/x/exp/slog#Record)s from STDIN (or a file) and output them as JSON to STDOUT.

## Installation

For use with Go programs:

```console
$ go get -u -v github.com/picatz/slogproto
```

For use with the `slp` CLI:

```console
$ go install -u -v github.com/picatz/slogproto/cmd/slp@latest
```

## Usage

```go
package main

import (
	"os"

	"github.com/picatz/slogproto"
	"golang.org/x/exp/slog"
)

func main() {
	logger := slog.New(slogproto.NewHander(os.Stdout))

	logger.Info("example", slog.Int("something", 1))
}
```

```console
$ go run main.go | slp
{"time":"2023-08-01T03:12:11.272826Z","level":"INFO","msg":"example","something":1}
```

```console
$ slp output.log
{"time":"..","level":"...","msg":"...", ... }
{"time":"..","level":"...","msg":"...", ... }
{"time":"..","level":"...","msg":"...", ... }
...
```

> **Note**: input to `slp` can be from STDIN or a file.

## File Format

The file format is a series of [delimited](https://developers.google.com/protocol-buffers/docs/techniques#streaming) [Protocol Buffer](https://developers.google.com/protocol-buffers) messages. Each message is prefixed with a [varint](https://developers.google.com/protocol-buffers/docs/encoding#varints) that indicates the length of the message using a [base-128](https://en.wikipedia.org/wiki/Variable-length_quantity) encoding (4 bytes max).

```console
╭────────────────────────────────────────────────────────────╮
│  Message Size  │  Protocol Buffer Message  │  ...  │  EOF  │
╰────────────────────────────────────────────────────────────╯
```
