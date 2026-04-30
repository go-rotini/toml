# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 0.1.0

### Added
- Full TOML 1.1.0 scanner, parser, encoder, and decoder
- `Marshal`/`Unmarshal` with struct tag support (`omitempty`, `inline`, `multiline`, `literal`, `commented`, `hex`, `octal`, `binary`, `required`, `default=<value>`)
- `UnmarshalTo[T]()` generic API
- `Encoder`/`Decoder` streaming types with context support
- `DecodeFile()` for reading TOML files from disk
- `Parse()` for AST access with `Walk()`, `Filter()`, `NodeToBytes()`
- `PathString()` path query system with `Read()`, `ReadString()`, `Replace()`, `Append()`, `Delete()`
- `ToJSON()`/`FromJSON()` conversion utilities
- `FormatError()` with source line and caret pointer
- `Valid()` quick validation
- `LocalDate`, `LocalTime`, `LocalDateTime` types
- `MapSlice`/`MapItem` for ordered map support
- `RawValue` for deferred decoding
- Custom marshaler/unmarshaler interfaces (`Marshaler`, `BytesMarshaler`, `MarshalerContext`, `Unmarshaler`, `BytesUnmarshaler`, `UnmarshalerContext`)
- `WithCustomMarshaler[T]()` and `WithCustomUnmarshaler[T]()` option-based custom codecs
- `StructValidator` interface for post-decode validation
- Functional options: `WithStrict()`, `WithDefaults()`, `WithOrderedMap()`, `WithMaxDepth()`, `WithMaxKeys()`, `WithMaxDocumentSize()`, `WithMaxNodes()`, `WithLocalTimeZone()`, and more
- Comment preservation through AST round-trips
- TOML 1.1.0 features: Unicode bare keys, `\e`/`\xHH` escape sequences, inline table newlines, trailing commas in inline tables and arrays
- DoS protection: depth limiting, key count limiting, document size limiting, node count limiting
- `comment:"text"` struct tag for attaching comments to encoded output
- Acceptance tests with real-world TOML files (Cargo.toml, pyproject.toml, Hugo config)
- Fuzz tests (FuzzUnmarshal, FuzzScanner, FuzzRoundTrip, FuzzValid)
- Example tests for all major public functions
- 663+ unit tests
