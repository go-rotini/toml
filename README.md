# go-rotini/toml

A Go TOML encoding and decoding package that implements the [TOML 1.1.0 specification](https://toml.io/en/v1.1.0) backed by the [TOML Test Suite](https://github.com/toml-lang/toml-test) conformance tests.

This package is used as the default TOML support package for [rotini](https://github.com/go-rotini/rotini).

## Features

- Full [TOML 1.1.0](https://toml.io/en/v1.1.0) specification support including Unicode bare keys, `\e`/`\xHH` escapes, inline table newlines, and trailing commas
- Tested against the official [TOML Test Suite](https://github.com/toml-lang/toml-test) for conformance
- Generic `UnmarshalTo[T]` API and type-safe custom marshaler/unmarshaler registration
- Multi-document streaming with `Encoder`/`Decoder`
- Struct field tags: `omitempty`, `inline`, `multiline`, `literal`, `commented`, `hex`/`octal`/`binary`, `required`, `default=<value>`
- Encode options: indent, indent tables, inline tables, multiline arrays, literal strings, table separators, comments
- Decode options: strict mode, ordered maps, struct validation, local time zone, JSON unmarshaler fallback
- AST access via `Parse`, `Walk`, `Filter`, and `Node` tree manipulation
- JSONPath-like query engine (`PathString`) with read, replace, append, and delete operations
- Bidirectional JSON conversion (`ToJSON`/`FromJSON`)
- `Valid` function for quick syntax validation without full decoding
- `FormatError` for human-readable error output with source line and column pointer
- Context-aware encoding/decoding via `EncodeContext`/`DecodeContext`
- Local date/time types: `LocalDate`, `LocalTime`, `LocalDateTime`
- Deferred decoding with `RawValue` and file decoding with `DecodeFile`
- DoS protection: depth limiting, key count limiting, document size limiting, node count limiting

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
	"log"

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
	// Marshal
	cfg := Config{
		Title: "Example",
		Owner: Owner{Name: "Alice"},
		Database: Database{
			Ports:   []int{8000, 8001, 8002},
			Enabled: true,
			ConnMax: 5000,
		},
	}
	b, err := toml.Marshal(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))

	// Unmarshal
	var cfg2 Config
	if err := toml.Unmarshal(b, &cfg2); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", cfg2)

	// Generic unmarshal (no pointer required)
	cfg3, err := toml.UnmarshalTo[Config](b)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", cfg3)
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
