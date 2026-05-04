# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- TOML 1.1.0 scanner, parser, encoder, and decoder
- Marshal/Unmarshal with struct tag support
- Generic UnmarshalTo[T] API
- Multi-document streaming via Encoder/Decoder
- TOMLPath query engine
- ToJSON and FromJSON helpers
- AST access via Parse, Walk, Filter, and NodeToBytes
- Comment preservation through AST round-trips
- LocalDate, LocalTime, LocalDateTime types
- MapSlice/MapItem for ordered map support
- RawValue for deferred decoding
- DecodeFile for reading TOML files from disk
- Custom marshaler/unmarshaler interfaces and option-based registration
- Struct validation via StructValidator interface
- Fuzz testing corpus
- TOML test suite conformance tests
- DoS protection defaults; max nesting depth 100, max keys and document size configurable
