# toml

A pure Go implementation of [TOML 1.1.0](https://toml.io/en/v1.1.0) with zero runtime dependencies.

Part of the [go-rotini](https://github.com/go-rotini) CLI framework.

## Installation

```
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
    Ports   []int  `toml:"ports"`
    Enabled bool   `toml:"enabled"`
    ConnMax int    `toml:"connection_max"`
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
    var cfg Config
    if err := toml.Unmarshal([]byte(input), &cfg); err != nil {
        panic(err)
    }
    fmt.Println(cfg.Title)  // Example
    fmt.Println(cfg.Owner.Name)  // Alice
    fmt.Println(cfg.Database.Ports)  // [8000 8001 8002]

    // Marshal back to TOML
    out, _ := toml.Marshal(cfg)
    fmt.Println(string(out))
}
```

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

## Struct Tags

```go
type Example struct {
    Name     string `toml:"name"`
    Count    int    `toml:"count,omitempty"`
    Ignored  string `toml:"-"`
    Debug    int    `toml:"debug,commented"`
    Port     int    `toml:"port,required"`
    Timeout  int    `toml:"timeout,default=30"`
    ID       int    `toml:"id,hex"`
    Perms    int    `toml:"perms,octal"`
    Flags    int    `toml:"flags,binary"`
    Desc     string `toml:"desc,multiline"`
    Path     string `toml:"path,literal"`
    Settings Inner  `toml:"settings,inline"`
}
```

Comments can be attached via a separate `comment` tag:

```go
type Config struct {
    Port int `toml:"port" comment:"Server listen port"`
}
```

## Decode Options

```go
toml.UnmarshalWithOptions(data, &cfg,
    toml.WithStrict(),            // reject unknown keys
    toml.WithDefaults(),          // apply default= tag values
    toml.WithOrderedMap(),        // preserve key order
    toml.WithMaxDepth(50),        // limit nesting depth
    toml.WithMaxKeys(10000),      // limit key count
    toml.WithMaxDocumentSize(1<<20), // limit input size
    toml.WithValidator(myValidator),
    toml.WithLocalTimeZone(time.UTC),
)
```

## Encode Options

```go
toml.MarshalWithOptions(cfg,
    toml.WithOmitEmpty(true),
    toml.WithArrayMultiline(true),
    toml.WithLiteralString(true),
    toml.WithTableSeparator(true),
    toml.WithComment("server.port", toml.Comment{
        Text:     "Listen port",
        Position: toml.HeadCommentPos,
    }),
)
```

## AST Access

```go
f, _ := toml.Parse(data)

// Walk all nodes
toml.Walk(f.Root, func(n *toml.Node) bool {
    fmt.Println(n.Kind, n.Key, n.Value)
    return true
})

// Path queries
p, _ := toml.PathString("$.database.ports[0]")
results, _ := p.Read(f.Root)

// Serialize back
out, _ := toml.NodeToBytes(f.Root)
```

## JSON Conversion

```go
jsonData, _ := toml.ToJSON(tomlData)
tomlData, _ := toml.FromJSON(jsonData)
```

## Error Formatting

```go
var cfg Config
err := toml.Unmarshal(data, &cfg)
if err != nil {
    fmt.Println(toml.FormatError(data, err))
    // Output:
    // toml: line 3, column 5: duplicate key "name"
    //   name = "second"
    //       ^
}
```

## License

MIT
