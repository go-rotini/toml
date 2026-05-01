# toml

A Go TOML encoding and decoding package that implements the [TOML 1.1.0 specification](https://toml.io/en/v1.1.0) backed by the [TOML Test Suite]() conformance tests.

This package is used as the default TOML support package for [rotini](https://github.com/go-rotini/rotini).

## Features

- **TOML 1.1.0 compliant** — Unicode bare keys, `\e`/`\xHH` escapes, inline table newlines and trailing commas
- **Zero runtime dependencies** — only the Go standard library
- **`encoding/json`-style API** — `Marshal`/`Unmarshal`, `Encoder`/`Decoder`, struct tags
- **Generic API** — `UnmarshalTo[T]()` for type-safe decoding
- **AST access** — `Parse()` returns a mutable node tree; inspect, modify, and re-serialize
- **Path queries** — `PathString("$.server.ports[0]")` for reading and mutating the AST
- **Comment preservation** — head and line comments survive round-trips through the AST
- **JSON conversion** — `ToJSON()` and `FromJSON()` for interop
- **Rich struct tags** — `omitempty`, `inline`, `multiline`, `literal`, `commented`, `hex`/`octal`/`binary`, `required`, `default=<value>`
- **Custom marshalers** — `Marshaler`, `BytesMarshaler`, `MarshalerContext` (with context)
- **Struct validation** — `WithValidator()` for post-decode validation
- **Strict mode** — `WithStrict()` rejects unknown keys
- **Ordered maps** — `WithOrderedMap()` returns `MapSlice` preserving key order
- **DoS protection** — `WithMaxDepth()`, `WithMaxKeys()`, `WithMaxDocumentSize()`, `WithMaxNodes()`
- **Error formatting** — `FormatError()` with source line and caret pointer
- **Local date/time types** — `LocalDate`, `LocalTime`, `LocalDateTime`
- **Deferred decoding** — `RawValue` for lazy unmarshaling
- **File decoding** — `DecodeFile()` reads directly from disk

## Installation

```bash
go get github.com/go-rotini/toml
```

Requires Go 1.23 or later.

## Quick Start

```go
package main

import (
	"fmt"
	"github.com/go-rotini/toml"
)

type Config struct {
	Title    string   `toml:"title"`
	Owner    Owner    `toml:"owner"`
	Database Database `toml:"database"`
}

type Owner struct {
	Name string `toml:"name"`
}

type Database struct {
	Ports   []int `toml:"ports"`
	Enabled bool  `toml:"enabled"`
	ConnMax int   `toml:"connection_max"`
}

func main() {
    input := `
title = "Example"

[owner]
name = "Alice"

[database]
ports = [8000, 8001, 8002]
enabled = true
connection_max = 5000
`
	// Unmarshal
	var cfg Config
	if err := toml.Unmarshal([]byte(input), &cfg); err != nil {
		fmt.Fatal(err)
	}
	fmt.Println(cfg.Title)
	fmt.Println(cfg.Owner.Name)
	fmt.Println(cfg.Database.Ports)

	// Marshal
	out, _ := toml.Marshal(cfg)
	fmt.Println(string(out))
}
```

## Documentation

Full API reference is available on [pkg.go.dev](https://pkg.go.dev/github.com/go-rotini/toml).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute to this project.

## Code of Conduct

This project follows a code of conduct to ensure a welcoming community. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Security

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
