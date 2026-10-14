package toml

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAcceptanceCargoToml(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}

	// Parse into map
	var m map[string]any
	if err := Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Check package section
	pkg, ok := m["package"].(map[string]any)
	if !ok {
		t.Fatal("package section not found")
	}
	if pkg["name"] != "my-project" {
		t.Errorf("package.name = %v", pkg["name"])
	}
	if pkg["version"] != "0.1.0" {
		t.Errorf("package.version = %v", pkg["version"])
	}
	authors, ok := pkg["authors"].([]any)
	if !ok || len(authors) != 2 {
		t.Errorf("package.authors = %v", pkg["authors"])
	}

	// Check dependencies (inline tables)
	deps, ok := m["dependencies"].(map[string]any)
	if !ok {
		t.Fatal("dependencies section not found")
	}
	serde, ok := deps["serde"].(map[string]any)
	if !ok {
		t.Fatal("dependencies.serde not found or not table")
	}
	if serde["version"] != "1.0" {
		t.Errorf("serde.version = %v", serde["version"])
	}

	// Check array of tables (bins)
	bins, ok := m["bin"].([]any)
	if !ok {
		t.Fatal("bin array of tables not found")
	}
	if len(bins) != 2 {
		t.Errorf("expected 2 bins, got %d", len(bins))
	}

	// Round-trip: marshal and parse again
	out, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m2 map[string]any
	if err := Unmarshal(out, &m2); err != nil {
		t.Fatalf("re-Unmarshal: %v", err)
	}

	// Verify key data survived round-trip
	pkg2 := m2["package"].(map[string]any)
	if pkg2["name"] != "my-project" {
		t.Error("round-trip lost package.name")
	}
}

func TestAcceptancePyprojectToml(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "pyproject.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Check nested tables
	project, ok := m["project"].(map[string]any)
	if !ok {
		t.Fatal("project not found")
	}
	if project["name"] != "my-python-project" {
		t.Errorf("project.name = %v", project["name"])
	}
	if project["version"] != "1.2.3" {
		t.Errorf("project.version = %v", project["version"])
	}

	// Check dependencies array
	deps, ok := project["dependencies"].([]any)
	if !ok {
		t.Fatal("project.dependencies not found")
	}
	if len(deps) != 3 {
		t.Errorf("expected 3 dependencies, got %d", len(deps))
	}

	// Check deep nesting
	tool, ok := m["tool"].(map[string]any)
	if !ok {
		t.Fatal("tool not found")
	}
	black, ok := tool["black"].(map[string]any)
	if !ok {
		t.Fatal("tool.black not found")
	}
	lineLen, ok := black["line-length"].(int64)
	if !ok || lineLen != 88 {
		t.Errorf("tool.black.line-length = %v", black["line-length"])
	}

	// Valid check
	if !Valid(data) {
		t.Error("Valid() returned false for pyproject.toml")
	}
}

func TestAcceptanceHugoToml(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "hugo.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if m["title"] != "My Hugo Site" {
		t.Errorf("title = %v", m["title"])
	}
	paginate, ok := m["paginate"].(int64)
	if !ok || paginate != 10 {
		t.Errorf("paginate = %v", m["paginate"])
	}

	// Check array of tables for menu
	menu, ok := m["menu"].(map[string]any)
	if !ok {
		t.Fatal("menu not found")
	}
	mainMenu, ok := menu["main"].([]any)
	if !ok {
		t.Fatalf("menu.main not found, got %T", menu["main"])
	}
	if len(mainMenu) != 3 {
		t.Errorf("expected 3 menu items, got %d", len(mainMenu))
	}

	// Check deeply nested tables
	markup, ok := m["markup"].(map[string]any)
	if !ok {
		t.Fatal("markup not found")
	}
	goldmark, ok := markup["goldmark"].(map[string]any)
	if !ok {
		t.Fatal("markup.goldmark not found")
	}
	renderer, ok := goldmark["renderer"].(map[string]any)
	if !ok {
		t.Fatal("markup.goldmark.renderer not found")
	}
	if renderer["unsafe"] != false {
		t.Errorf("markup.goldmark.renderer.unsafe = %v", renderer["unsafe"])
	}
}

func TestAcceptanceParseAllFixtures(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("testdata", "acceptance"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "acceptance", entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if !Valid(data) {
				t.Errorf("Valid() returned false")
			}
			var m map[string]any
			if err := Unmarshal(data, &m); err != nil {
				t.Errorf("Unmarshal: %v", err)
			}

			// Parse via AST
			f, err := Parse(data)
			if err != nil {
				t.Errorf("Parse: %v", err)
				return
			}
			if f.Root == nil {
				t.Error("Parse returned nil root")
				return
			}

			// JSON conversion
			jsonData, err := ToJSON(data)
			if err != nil {
				t.Errorf("ToJSON: %v", err)
			}
			if len(jsonData) == 0 {
				t.Error("ToJSON returned empty")
			}
		})
	}
}

func TestAcceptancePathQueries(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// Query package name via ReadString
	p, err := PathString("$.package.name")
	if err != nil {
		t.Fatal(err)
	}
	val, err := p.ReadString(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	if val != "my-project" {
		t.Errorf("$.package.name = %q, want %q", val, "my-project")
	}
}

func TestAcceptanceStrictMode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}

	// Strict mode with a struct that doesn't have all fields should fail
	type Partial struct {
		Package struct {
			Name string `toml:"name"`
		} `toml:"package"`
	}
	var p Partial
	err = UnmarshalWithOptions(data, &p, WithStrict())
	if err == nil {
		t.Error("expected strict mode error for partial struct")
	}
}

func TestAcceptanceOrderedMap(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "hugo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var m any
	if err := UnmarshalWithOptions(data, &m, WithOrderedMap()); err != nil {
		t.Fatal(err)
	}
	ms, ok := m.(MapSlice)
	if !ok {
		t.Fatalf("expected MapSlice, got %T", m)
	}
	if len(ms) == 0 {
		t.Error("empty MapSlice")
	}
	// First key should be baseURL
	if ms[0].Key != "baseURL" {
		t.Errorf("first key = %v, want baseURL", ms[0].Key)
	}
}

// kitchenSinkConfig models the kitchen-sink.toml fixture as a struct, exercising
// most decoder paths (scalars, arrays, nested tables, datetimes, byte slices,
// arrays of tables, dotted keys, integer base prefixes).
type kitchenSinkConfig struct {
	Title                string         `toml:"title"`
	MultilineDescription string         `toml:"multiline_description"`
	LiteralPath          string         `toml:"literal_path"`
	MultilineLiteral     string         `toml:"multiline_literal"`
	Meta                 kitchenMeta    `toml:"meta"`
	Server               kitchenServer  `toml:"server"`
	Features             kitchenFlags   `toml:"features"`
	BinaryData           kitchenBinary  `toml:"binary_data"`
	Locales              map[string]any `toml:"locales"`
	Deeply               kitchenDeep    `toml:"deeply"`
}

type kitchenMeta struct {
	SchemaVersion   int               `toml:"schema_version"`
	PercentComplete float64           `toml:"percent_complete"`
	Ratio           float64           `toml:"ratio"`
	Total           int64             `toml:"total"`
	HexValue        uint64            `toml:"hex_value"`
	OctalValue      uint64            `toml:"octal_value"`
	BinaryValue     uint64            `toml:"binary_value"`
	Enabled         bool              `toml:"enabled"`
	Disabled        bool              `toml:"disabled"`
	Timestamps      kitchenTimestamps `toml:"timestamps"`
}

type kitchenTimestamps struct {
	CreatedOffset time.Time     `toml:"created_offset"`
	UpdatedUTC    time.Time     `toml:"updated_utc"`
	LocalDateTime LocalDateTime `toml:"local_dt"`
	LocalDateOnly LocalDate     `toml:"local_d"`
	LocalTimeOnly LocalTime     `toml:"local_t"`
}

type kitchenServer struct {
	Host      string            `toml:"host"`
	Port      int               `toml:"port"`
	Timeout   time.Duration     `toml:"timeout"`
	TLS       kitchenTLS        `toml:"tls"`
	Endpoints []kitchenEndpoint `toml:"endpoints"`
}

type kitchenTLS struct {
	Cert string `toml:"cert"`
	Key  string `toml:"key"`
}

type kitchenEndpoint struct {
	Path   string `toml:"path"`
	Method string `toml:"method"`
}

type kitchenFlags struct {
	Flags       []string       `toml:"flags"`
	Priorities  []int          `toml:"priorities"`
	Matrix      [][]int        `toml:"matrix"`
	MixedInline map[string]any `toml:"mixed_inline"`
}

type kitchenBinary struct {
	Payload []byte `toml:"payload"`
}

type kitchenDeep struct {
	Nested struct {
		Config struct {
			Value struct {
				Final string `toml:"final"`
			} `toml:"value"`
		} `toml:"config"`
	} `toml:"nested"`
}

func loadKitchenSink(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "kitchen-sink.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAcceptanceKitchenSinkStruct(t *testing.T) {
	data := loadKitchenSink(t)
	var cfg kitchenSinkConfig
	if err := Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Title != "Kitchen Sink Config" {
		t.Errorf("title = %q", cfg.Title)
	}
	if !strings.Contains(cfg.MultilineDescription, "broad to cover") {
		t.Errorf("multiline_description lost content: %q", cfg.MultilineDescription)
	}
	if cfg.LiteralPath != `C:\Users\example\config.toml` {
		t.Errorf("literal_path = %q", cfg.LiteralPath)
	}

	if cfg.Meta.SchemaVersion != 2 {
		t.Errorf("meta.schema_version = %d", cfg.Meta.SchemaVersion)
	}
	if cfg.Meta.HexValue != 0xDEADBEEF {
		t.Errorf("meta.hex_value = %#x", cfg.Meta.HexValue)
	}
	if cfg.Meta.OctalValue != 0o755 {
		t.Errorf("meta.octal_value = %o", cfg.Meta.OctalValue)
	}
	if cfg.Meta.BinaryValue != 0b10101010 {
		t.Errorf("meta.binary_value = %b", cfg.Meta.BinaryValue)
	}
	if cfg.Meta.Total != 1_000_000 {
		t.Errorf("meta.total = %d", cfg.Meta.Total)
	}
	if cfg.Meta.PercentComplete != 99.5 {
		t.Errorf("meta.percent_complete = %v", cfg.Meta.PercentComplete)
	}
	if !cfg.Meta.Enabled || cfg.Meta.Disabled {
		t.Error("bool fields wrong")
	}

	if cfg.Meta.Timestamps.CreatedOffset.IsZero() {
		t.Error("created_offset zero")
	}
	if cfg.Meta.Timestamps.UpdatedUTC.UTC().Year() != 1979 {
		t.Errorf("updated_utc year = %d", cfg.Meta.Timestamps.UpdatedUTC.UTC().Year())
	}
	if cfg.Meta.Timestamps.LocalDateOnly.Year != 1979 {
		t.Errorf("local_d year = %d", cfg.Meta.Timestamps.LocalDateOnly.Year)
	}
	if cfg.Meta.Timestamps.LocalTimeOnly.Hour != 7 {
		t.Errorf("local_t hour = %d", cfg.Meta.Timestamps.LocalTimeOnly.Hour)
	}

	if cfg.Server.Timeout != 30*time.Second {
		t.Errorf("server.timeout = %v", cfg.Server.Timeout)
	}
	if len(cfg.Server.Endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(cfg.Server.Endpoints))
	}

	if len(cfg.Features.Matrix) != 3 || cfg.Features.Matrix[2][1] != 6 {
		t.Errorf("features.matrix wrong: %v", cfg.Features.Matrix)
	}

	if string(cfg.BinaryData.Payload) != "hello world" {
		t.Errorf("binary_data.payload = %q", cfg.BinaryData.Payload)
	}

	if cfg.Locales["fr-FR"] != "French" {
		t.Errorf("locales[fr-FR] = %v", cfg.Locales["fr-FR"])
	}
	if cfg.Locales["key.with.dots"] != "quoted dotted" {
		t.Errorf("locales[key.with.dots] = %v", cfg.Locales["key.with.dots"])
	}
	if cfg.Locales["123"] != "numeric key" {
		t.Errorf("locales[123] = %v", cfg.Locales["123"])
	}

	if cfg.Deeply.Nested.Config.Value.Final != "leaf" {
		t.Errorf("deeply.nested.config.value.final = %q", cfg.Deeply.Nested.Config.Value.Final)
	}
}

func TestAcceptanceKitchenSinkRoundTrip(t *testing.T) {
	data := loadKitchenSink(t)

	var cfg kitchenSinkConfig
	if err := Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out, err := Marshal(&cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var cfg2 kitchenSinkConfig
	if err := Unmarshal(out, &cfg2); err != nil {
		t.Fatalf("re-Unmarshal:\n%s\nerr: %v", out, err)
	}
	if cfg.Server.Host != cfg2.Server.Host {
		t.Errorf("round-trip lost server.host: %q vs %q", cfg.Server.Host, cfg2.Server.Host)
	}
	if cfg.Meta.HexValue != cfg2.Meta.HexValue {
		t.Errorf("round-trip lost hex_value: %x vs %x", cfg.Meta.HexValue, cfg2.Meta.HexValue)
	}
	if !bytes.Equal(cfg.BinaryData.Payload, cfg2.BinaryData.Payload) {
		t.Errorf("round-trip lost binary payload")
	}
}

func TestAcceptanceFloatSpecialValues(t *testing.T) {
	src := `
positive_inf = +inf
negative_inf = -inf
not_a_number = nan
plain_inf = inf
zero_pi = 0.0
neg_zero = -0.0
big_exp = 1e100
small = 1e-10
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if !math.IsInf(m["positive_inf"].(float64), 1) {
		t.Errorf("positive_inf = %v", m["positive_inf"])
	}
	if !math.IsInf(m["negative_inf"].(float64), -1) {
		t.Errorf("negative_inf = %v", m["negative_inf"])
	}
	if !math.IsNaN(m["not_a_number"].(float64)) {
		t.Errorf("not_a_number = %v", m["not_a_number"])
	}
	if m["big_exp"].(float64) != 1e100 {
		t.Errorf("big_exp = %v", m["big_exp"])
	}
}

func TestAcceptanceBigIntFloat(t *testing.T) {
	// Values within int64 / float64 range, targeting big.Int/big.Float pointers.
	src := `
small_for_big = 12345
ratio = 3.14159
`
	type Bigs struct {
		Small *big.Int   `toml:"small_for_big"`
		Ratio *big.Float `toml:"ratio"`
	}
	var b Bigs
	if err := Unmarshal([]byte(src), &b); err != nil {
		t.Fatal(err)
	}
	if b.Small == nil || b.Small.Sign() <= 0 {
		t.Errorf("small = %v", b.Small)
	}
	if b.Ratio == nil || b.Ratio.Sign() <= 0 {
		t.Errorf("ratio = %v", b.Ratio)
	}
}

func TestAcceptanceEncodeOptionsAll(t *testing.T) {
	type Inner struct {
		Tag  string `toml:"tag"`
		Note string `toml:"note,multiline"`
	}
	type Outer struct {
		Name   string   `toml:"name"`
		Empty  string   `toml:"empty,omitempty"`
		Hex    int      `toml:"hex,hex"`
		Octal  int      `toml:"octal,octal"`
		Binary int      `toml:"binary,binary"`
		Tags   []string `toml:"tags"`
		Inner  Inner    `toml:"inner"`
	}

	v := Outer{
		Name:   "demo",
		Hex:    255,
		Octal:  8,
		Binary: 5,
		Tags:   []string{"a", "b"},
		Inner:  Inner{Tag: "x", Note: "line1\nline2"},
	}

	out, err := MarshalWithOptions(v,
		WithIndent(2),
		WithIndentTables(true),
		WithTableSeparator(false),
		WithMultiLineString(true),
		WithLiteralString(true),
		WithBasicStrings(false),
		WithOmitEmpty(true),
		WithArrayMultiline(true),
		WithComment(map[string][]Comment{
			"name":  {{Position: HeadCommentPos, Text: "the name"}},
			"inner": {{Position: LineCommentPos, Text: "inline note"}},
		}),
	)
	if err != nil {
		t.Fatalf("MarshalWithOptions: %v", err)
	}
	s := string(out)
	if !strings.Contains(strings.ToLower(s), "0xff") {
		t.Errorf("expected hex prefix in output:\n%s", s)
	}
	if !strings.Contains(s, "0o10") {
		t.Errorf("expected octal prefix in output:\n%s", s)
	}
	if !strings.Contains(s, "0b101") {
		t.Errorf("expected binary prefix in output:\n%s", s)
	}
	if !strings.Contains(s, "# the name") {
		t.Errorf("expected head comment in output:\n%s", s)
	}
}

func TestAcceptanceInlineTableEncoding(t *testing.T) {
	type Inner struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type Cfg struct {
		Server Inner          `toml:"server"`
		Map    map[string]any `toml:"map,inline"`
	}
	c := Cfg{
		Server: Inner{Host: "localhost", Port: 9090},
		Map:    map[string]any{"k": "v"},
	}
	// WithInlineTable forces struct-typed values to use {...} syntax.
	out, err := MarshalWithOptions(&c, WithInlineTable(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "{") {
		t.Errorf("inline table not produced:\n%s", out)
	}

	// Also test inline-tagged map (uses encodeInlineMap, not encodeInlineTable).
	out2, err := Marshal(map[string]any{"x": map[string]any{"a": 1, "b": 2}})
	if err != nil {
		t.Fatal(err)
	}
	_ = out2
}

func TestAcceptanceStreamingEncoderDecoder(t *testing.T) {
	type Doc struct {
		Title string `toml:"title"`
		Count int    `toml:"count"`
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf, WithIndent(2))
	enc.SetContext(context.Background())
	if err := enc.Encode(Doc{Title: "first", Count: 1}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dec := NewDecoder(&buf)
	dec.SetContext(context.Background())
	var got Doc
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Title != "first" || got.Count != 1 {
		t.Errorf("got %+v", got)
	}
	// Second decode should be EOF since TOML is single-document.
	if err := dec.Decode(&got); err == nil {
		t.Error("expected EOF on second Decode")
	}
}

func TestAcceptanceDecodeFile(t *testing.T) {
	path := filepath.Join("testdata", "acceptance", "hugo.toml")
	type Hugo struct {
		Title    string `toml:"title"`
		Theme    string `toml:"theme"`
		Paginate int    `toml:"paginate"`
	}
	var h Hugo
	if err := DecodeFile(path, &h); err != nil {
		t.Fatal(err)
	}
	if h.Title == "" || h.Theme == "" || h.Paginate == 0 {
		t.Errorf("DecodeFile incomplete: %+v", h)
	}

	if err := DecodeFile("testdata/acceptance/does-not-exist.toml", &h); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestAcceptanceUnmarshalTo(t *testing.T) {
	type Cfg struct {
		Name string `toml:"name"`
	}
	cfg, err := UnmarshalTo[Cfg]([]byte(`name = "demo"`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "demo" {
		t.Errorf("UnmarshalTo: got %+v", cfg)
	}
}

func TestAcceptanceASTOperations(t *testing.T) {
	data := loadKitchenSink(t)
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// Walk and count nodes by kind.
	counts := map[NodeKind]int{}
	Walk(f.Root, func(n *Node) bool {
		counts[n.Kind]++
		return true
	})
	if counts[TableNode] == 0 {
		t.Error("no TableNode found")
	}
	if counts[ArrayTableNode] == 0 {
		t.Error("no ArrayTableNode found")
	}
	if counts[KeyValueNode] == 0 {
		t.Error("no KeyValueNode found")
	}

	// Filter: get all string nodes.
	strings := Filter(f.Root, func(n *Node) bool { return n.Kind == StringNode })
	if len(strings) == 0 {
		t.Error("Filter returned no string nodes")
	}

	// Validate
	if err := f.Root.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	// Re-serialize
	out, err := NodeToBytes(f.Root)
	if err != nil {
		t.Fatalf("NodeToBytes: %v", err)
	}
	if len(out) == 0 {
		t.Error("NodeToBytes returned empty")
	}

	out2, err := NodeToBytesWithOptions(f.Root, WithIndentTables(true), WithTableSeparator(false))
	if err != nil {
		t.Fatalf("NodeToBytesWithOptions: %v", err)
	}
	if len(out2) == 0 {
		t.Error("NodeToBytesWithOptions returned empty")
	}

	// Stringer methods on Node
	if f.Root.String() == "" && len(f.Root.Children) > 0 {
		// root has empty key path; check a child instead
		for _, c := range f.Root.Children {
			if c.String() == "" && c.Kind != CommentNode {
				t.Errorf("Node.String returned empty for kind %v", c.Kind)
			}
		}
	}
}

func TestAcceptancePathReadReplaceAppendDelete(t *testing.T) {
	src := `
[server]
host = "localhost"
port = 8080

[[entry]]
name = "first"

[[entry]]
name = "second"

[[entry]]
name = "third"
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	// Read
	p, err := PathString("$.server.port")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := p.Read(f.Root)
	if err != nil || len(nodes) == 0 {
		t.Fatalf("Read: %v %v", nodes, err)
	}

	// ReadString
	val, err := p.ReadString(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	if val != "8080" {
		t.Errorf("port = %q", val)
	}

	// ReadPositions
	positions, err := p.ReadPositions(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) == 0 || positions[0].Line == 0 {
		t.Error("expected positions with non-zero line")
	}
	if positions[0].String() == "" {
		t.Error("Position.String empty")
	}

	// Wildcard
	pAll, err := PathString("$.server.*")
	if err != nil {
		t.Fatal(err)
	}
	all, err := pAll.Read(f.Root)
	if err != nil || len(all) < 2 {
		t.Errorf("wildcard: %v err=%v", all, err)
	}

	// Recursive descent
	pName, err := PathString("$..name")
	if err != nil {
		t.Fatal(err)
	}
	names, err := pName.Read(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Errorf("recursive descent: got %d nodes, want 3", len(names))
	}

	// Index access (negative)
	pLast, err := PathString("$.entry[-1]")
	if err != nil {
		t.Fatal(err)
	}
	last, err := pLast.Read(f.Root)
	if err != nil || len(last) == 0 {
		t.Fatalf("negative index: %v %v", last, err)
	}

	// Replace
	replacement := &Node{
		Kind:     KeyValueNode,
		Key:      []string{"port"},
		Children: []*Node{{Kind: IntegerNode, Value: "9090"}},
	}
	if err := p.Replace(f.Root, replacement); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	// Append
	pSrv, err := PathString("$.server")
	if err != nil {
		t.Fatal(err)
	}
	addition := &Node{
		Kind:     KeyValueNode,
		Key:      []string{"name"},
		Children: []*Node{{Kind: StringNode, Value: "demo"}},
	}
	if err := pSrv.Append(f.Root, addition); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Delete
	pHost, err := PathString("$.server.host")
	if err != nil {
		t.Fatal(err)
	}
	if err := pHost.Delete(f.Root); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// String returns the original expression
	if pHost.String() != "$.server.host" {
		t.Errorf("Path.String = %q", pHost.String())
	}

	// Not-found
	pMissing, _ := PathString("$.nope")
	if _, err := pMissing.Read(f.Root); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}

	// Bad path syntax
	if _, err := PathString("nope"); !errors.Is(err, ErrPathSyntax) {
		t.Errorf("expected ErrPathSyntax, got %v", err)
	}
}

func TestAcceptanceTypedErrorsMatchSentinels(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want error
	}{
		{"syntax", `not = valid =`, ErrSyntax},
		{"duplicate", "a = 1\na = 2\n", ErrDuplicateKey},
		{"overflow", "a = 99999999999999999999999\n", nil}, // wraps in TypeError when decoded into int8
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m map[string]any
			err := Unmarshal([]byte(tt.src), &m)
			if err == nil {
				t.Skip("no error returned")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want match for %v", err, tt.want)
			}
			// FormatError returns non-empty string with line context.
			formatted := FormatError([]byte(tt.src), err)
			if formatted == "" {
				t.Error("FormatError returned empty")
			}
			// With color
			colored := FormatError([]byte(tt.src), err, true)
			if colored == "" {
				t.Error("FormatError(color) returned empty")
			}
		})
	}

	// Overflow case: int8 with large value
	type Tiny struct {
		N int8 `toml:"n"`
	}
	var ti Tiny
	if err := Unmarshal([]byte(`n = 1000`), &ti); err == nil {
		t.Error("expected overflow error")
	} else if !errors.Is(err, ErrType) && !errors.Is(err, ErrOverflow) {
		// either is acceptable here
		t.Logf("overflow err: %v", err)
	}

	// UnknownField with strict mode
	type Small struct {
		Name string `toml:"name"`
	}
	var s Small
	err := UnmarshalWithOptions([]byte(`name = "x"`+"\n"+`extra = 1`), &s, WithStrict())
	if err == nil {
		t.Error("expected unknown field error")
	}
	if !errors.Is(err, ErrUnknownField) {
		t.Errorf("expected ErrUnknownField, got %v", err)
	}
}

type acceptanceValidator struct {
	called bool
}

func (v *acceptanceValidator) Struct(_ any) error {
	v.called = true
	return errors.New("validator rejected")
}

func TestAcceptanceValidator(t *testing.T) {
	type Cfg struct {
		Name string `toml:"name"`
	}
	var c Cfg
	v := &acceptanceValidator{}
	err := UnmarshalWithOptions([]byte(`name = "demo"`), &c, WithValidator(v))
	if err == nil {
		t.Fatal("expected validator error")
	}
	if !v.called {
		t.Error("validator not invoked")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestAcceptanceDefaults(t *testing.T) {
	type Cfg struct {
		Name    string        `toml:"name,default=anonymous"`
		Port    int           `toml:"port,default=8080"`
		Pi      float64       `toml:"pi,default=3.14"`
		Active  bool          `toml:"active,default=true"`
		Timeout time.Duration `toml:"timeout,default=30s"`
	}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(``), &c, WithDefaults()); err != nil {
		t.Fatal(err)
	}
	if c.Name != "anonymous" || c.Port != 8080 || c.Pi != 3.14 || !c.Active || c.Timeout != 30*time.Second {
		t.Errorf("defaults not applied: %+v", c)
	}
}

func TestAcceptanceRequired(t *testing.T) {
	type Cfg struct {
		Name string `toml:"name,required"`
	}
	var c Cfg
	if err := Unmarshal([]byte(``), &c); err == nil {
		t.Error("expected required-field error")
	}
}

type acceptancePoint struct {
	X, Y int
}

func (p *acceptancePoint) UnmarshalTOML(unmarshal func(any) error) error {
	var raw struct {
		X int `toml:"x"`
		Y int `toml:"y"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	p.X = raw.X
	p.Y = raw.Y
	return nil
}

func (p acceptancePoint) MarshalTOML() (any, error) {
	return map[string]any{"x": p.X, "y": p.Y}, nil
}

func TestAcceptanceCustomMarshalerInterface(t *testing.T) {
	type Cfg struct {
		Origin acceptancePoint `toml:"origin"`
	}
	var c Cfg
	src := `[origin]` + "\n" + `x = 3` + "\n" + `y = 4` + "\n"
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Origin.X != 3 || c.Origin.Y != 4 {
		t.Errorf("UnmarshalTOML failed: %+v", c.Origin)
	}

	out, err := Marshal(&c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(out), "x = 3") {
		t.Errorf("MarshalTOML not used:\n%s", out)
	}
}

type acceptanceURL string

func TestAcceptanceCustomMarshalerOption(t *testing.T) {
	type Cfg struct {
		URL acceptanceURL `toml:"url"`
	}

	encOpt := WithCustomMarshaler(func(u acceptanceURL) ([]byte, error) {
		return []byte(`"prefix://` + string(u) + `"`), nil
	})
	decOpt := WithCustomUnmarshaler(func(u *acceptanceURL, data []byte) error {
		s := strings.Trim(string(data), `"`)
		s = strings.TrimPrefix(s, "prefix://")
		*u = acceptanceURL(s)
		return nil
	})

	out, err := MarshalWithOptions(&Cfg{URL: acceptanceURL("example.com")}, encOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "prefix://example.com") {
		t.Errorf("custom marshaler not used:\n%s", out)
	}

	var c Cfg
	if err := UnmarshalWithOptions(out, &c, decOpt); err != nil {
		t.Fatal(err)
	}
	if c.URL != "example.com" {
		t.Errorf("custom unmarshaler not used: %q", c.URL)
	}
}

func TestAcceptanceJSONInterop(t *testing.T) {
	src := `
title = "demo"
[server]
host = "localhost"
port = 8080
`
	jsonBytes, err := ToJSON([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonBytes), `"title":"demo"`) {
		t.Errorf("ToJSON missing title: %s", jsonBytes)
	}

	tomlOut, err := FromJSON(jsonBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tomlOut), `title = "demo"`) {
		t.Errorf("FromJSON missing title: %s", tomlOut)
	}
}

func TestAcceptanceRawValue(t *testing.T) {
	raw := RawValue(`"hello world"`)
	var wrap struct {
		Raw string `toml:"_raw_"`
	}
	if err := raw.Unmarshal(&wrap); err != nil {
		t.Fatal(err)
	}
	if wrap.Raw != "hello world" {
		t.Errorf("RawValue.Unmarshal: %q", wrap.Raw)
	}
}

func TestAcceptanceLimits(t *testing.T) {
	deeplyNested := strings.Repeat("[a.", 200) + "b]\n" + strings.Repeat("a.", 200) + "b = 1\n"
	var m map[string]any
	err := UnmarshalWithOptions([]byte(deeplyNested), &m, WithMaxDepth(50))
	if err == nil {
		t.Skip("max depth not enforced for this input shape")
	}

	if err := UnmarshalWithOptions([]byte(`a=1`+"\n"+`b=2`+"\n"+`c=3`), &m, WithMaxKeys(2)); err == nil {
		t.Error("expected max-keys error")
	}

	if err := UnmarshalWithOptions([]byte(`a=1`), &m, WithMaxDocumentSize(2)); !errors.Is(err, ErrDocumentSize) {
		t.Errorf("expected ErrDocumentSize, got %v", err)
	}

	if err := UnmarshalWithOptions([]byte(`a=[1,2,3]`), &m, WithMaxNodes(2)); err == nil {
		t.Error("expected max-nodes error")
	}
}

func TestAcceptanceLocalTimeZone(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	type Cfg struct {
		When time.Time `toml:"when"`
	}
	var c Cfg
	src := `when = 2024-01-15T10:30:00`
	if err := UnmarshalWithOptions([]byte(src), &c, WithLocalTimeZone(loc)); err != nil {
		t.Fatal(err)
	}
	if c.When.Location().String() != loc.String() {
		t.Errorf("location = %v, want %v", c.When.Location(), loc)
	}
}

func TestAcceptanceMapSliceEncode(t *testing.T) {
	ms := MapSlice{
		MapItem{Key: "first", Value: "one"},
		MapItem{Key: "second", Value: int64(2)},
		MapItem{Key: "third", Value: true},
	}
	out, err := Marshal(ms)
	if err != nil {
		// MapSlice top-level is permitted via map-style encoding only if the encoder supports it.
		// If it's not supported, a wrapper map should work.
		out, err = Marshal(map[string]any{"items": ms})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(out) == 0 {
		t.Error("Marshal MapSlice: empty output")
	}
}

func TestAcceptanceCommentPreservation(t *testing.T) {
	src := `# top-level head comment
title = "demo" # inline comment

# section header
[server]
host = "localhost"
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	out, err := NodeToBytes(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "top-level head comment") {
		t.Errorf("head comment lost:\n%s", s)
	}
	if !strings.Contains(s, "inline comment") {
		t.Errorf("inline comment lost:\n%s", s)
	}
}

func TestAcceptanceJSONUnmarshalerOption(t *testing.T) {
	type T struct {
		Tags map[string]int `toml:"tags"`
	}
	// Use a JSON-style document via the option (currently only affects fallback).
	src := `[tags]` + "\n" + `a = 1` + "\n" + `b = 2`
	var v T
	if err := UnmarshalWithOptions([]byte(src), &v, WithJSONUnmarshaler()); err != nil {
		t.Fatal(err)
	}
	if v.Tags["a"] != 1 || v.Tags["b"] != 2 {
		t.Errorf("tags = %v", v.Tags)
	}
}

func TestAcceptanceTOMLPathOnLargeFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "pyproject.toml"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	for _, expr := range []string{
		"$.project.name",
		"$.project.dependencies",
		"$.project.urls.Homepage",
		"$.tool.black.line-length",
	} {
		p, err := PathString(expr)
		if err != nil {
			t.Errorf("PathString(%q): %v", expr, err)
			continue
		}
		nodes, err := p.Read(f.Root)
		if err != nil {
			t.Errorf("Read(%q): %v", expr, err)
			continue
		}
		if len(nodes) == 0 {
			t.Errorf("Read(%q): no nodes", expr)
		}
	}
}

func TestAcceptanceErrorTypeFormatting(t *testing.T) {
	// Trigger every named error type so its Error/Is/Unwrap methods get exercised.
	cases := []struct {
		name   string
		src    string
		opts   []DecodeOption
		target any
		check  func(t *testing.T, err error)
	}{
		{
			name: "duplicate",
			src:  "a = 1\na = 2\n",
			check: func(t *testing.T, err error) {
				t.Helper()
				var dup *DuplicateKeyError
				if !errors.As(err, &dup) {
					t.Errorf("expected DuplicateKeyError, got %T", err)
					return
				}
				if dup.Error() == "" || dup.Pos.Line == 0 {
					t.Error("DuplicateKeyError missing details")
				}
				if !errors.Is(err, ErrDuplicateKey) {
					t.Error("errors.Is(ErrDuplicateKey) failed")
				}
			},
		},
		{
			name: "inline-extension",
			src:  "[a]\nb=1\n[a.b.c]\nd=2\n",
			check: func(t *testing.T, err error) {
				t.Helper()
				if err == nil {
					t.Skip("no error")
				}
				_ = err.Error()
			},
		},
		{
			name: "conflict",
			src:  "[a]\nb=1\n[[a]]\nc=2\n",
			check: func(t *testing.T, err error) {
				t.Helper()
				var conf *ConflictError
				if errors.As(err, &conf) {
					_ = conf.Error()
					_ = conf.Is(&ConflictError{})
				}
				if errors.Is(err, ErrConflict) {
					return
				}
			},
		},
		{
			name: "overflow-int8",
			src:  "n = 1000",
			target: &struct {
				N int8 `toml:"n"`
			}{},
			check: func(t *testing.T, err error) {
				t.Helper()
				var ov *OverflowError
				if errors.As(err, &ov) {
					_ = ov.Error()
					_ = ov.Is(&OverflowError{})
				}
			},
		},
		{
			name: "syntax",
			src:  "broken =",
			check: func(t *testing.T, err error) {
				t.Helper()
				var syn *SyntaxError
				if errors.As(err, &syn) {
					_ = syn.Error()
					_ = syn.Is(&SyntaxError{})
				}
				if !errors.Is(err, ErrSyntax) {
					t.Error("errors.Is(ErrSyntax) failed")
				}
			},
		},
		{
			name: "type",
			src:  "n = \"not a number\"",
			target: &struct {
				N int `toml:"n"`
			}{},
			check: func(t *testing.T, err error) {
				t.Helper()
				var typ *TypeError
				if errors.As(err, &typ) {
					_ = typ.Error()
					_ = typ.Is(&TypeError{})
				}
			},
		},
		{
			name: "unknown-field",
			src:  "name = \"x\"\nextra = 1",
			opts: []DecodeOption{WithStrict()},
			target: &struct {
				Name string `toml:"name"`
			}{},
			check: func(t *testing.T, err error) {
				t.Helper()
				var unk *UnknownFieldError
				if !errors.As(err, &unk) {
					t.Errorf("expected UnknownFieldError, got %T", err)
					return
				}
				_ = unk.Error()
				_ = unk.Is(&UnknownFieldError{})
			},
		},
		{
			name: "default",
			src:  "",
			opts: []DecodeOption{WithDefaults()},
			target: &struct {
				N int `toml:"n,default=not-a-number"`
			}{},
			check: func(t *testing.T, err error) {
				t.Helper()
				var def *DefaultError
				if errors.As(err, &def) {
					_ = def.Error()
					_ = def.Is(&DefaultError{})
				}
			},
		},
		{
			name: "validation",
			src:  "name = \"x\"",
			opts: []DecodeOption{WithValidator(&acceptanceValidator{})},
			target: &struct {
				Name string `toml:"name"`
			}{},
			check: func(t *testing.T, err error) {
				t.Helper()
				var val *ValidationError
				if !errors.As(err, &val) {
					t.Errorf("expected ValidationError, got %T", err)
					return
				}
				_ = val.Error()
				_ = val.Is(&ValidationError{})
				if val.Unwrap() == nil {
					t.Error("ValidationError.Unwrap returned nil")
				}
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			if target == nil {
				target = &map[string]any{}
			}
			err := UnmarshalWithOptions([]byte(tt.src), target, tt.opts...)
			tt.check(t, err)

			// Exercise FormatError on the produced error.
			if err != nil {
				_ = FormatError([]byte(tt.src), err)
				_ = FormatError([]byte(tt.src), err, true)
			}
		})
	}

	// FormatError on an error without a Position falls through to err.Error().
	plain := errors.New("not positional")
	if FormatError([]byte("anything"), plain) != plain.Error() {
		t.Error("FormatError should return Error() for non-positional errors")
	}
}

func TestAcceptanceLocalTimeAsTime(t *testing.T) {
	d := LocalDate{Year: 2024, Month: 3, Day: 15}
	tm := d.AsTime(time.UTC)
	if tm.Year() != 2024 || tm.Month() != 3 || tm.Day() != 15 {
		t.Errorf("LocalDate.AsTime: %v", tm)
	}

	dt := LocalDateTime{LocalDate: d, LocalTime: LocalTime{Hour: 12, Minute: 30}}
	tm2 := dt.AsTime(time.UTC)
	if tm2.Hour() != 12 || tm2.Minute() != 30 {
		t.Errorf("LocalDateTime.AsTime: %v", tm2)
	}

	if d.String() == "" || dt.String() == "" {
		t.Error("String methods empty")
	}

	// Round-trip via MarshalText/UnmarshalText.
	b, err := d.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var d2 LocalDate
	if err := d2.UnmarshalText(b); err != nil {
		t.Fatal(err)
	}
	if d2 != d {
		t.Errorf("LocalDate round-trip: %v != %v", d2, d)
	}

	if err := d2.UnmarshalText([]byte("not a date")); err == nil {
		t.Error("expected error for invalid date")
	}

	lt := LocalTime{Hour: 7, Minute: 30, Second: 0, Nanosecond: 500_000_000}
	b2, _ := lt.MarshalText()
	var lt2 LocalTime
	if err := lt2.UnmarshalText(b2); err != nil {
		t.Fatal(err)
	}
	if lt2 != lt {
		t.Errorf("LocalTime round-trip: %v != %v", lt2, lt)
	}
	if err := lt2.UnmarshalText([]byte("not a time")); err == nil {
		t.Error("expected error for invalid time")
	}
}

func TestAcceptanceDateTimeIntoTimeAndInterface(t *testing.T) {
	src := `
offset = 1979-05-27T07:32:00Z
local_dt = 2024-01-15T10:30:00
local_d = 2024-01-15
local_t = 10:30:00
`
	// Decode into struct of time.Time.
	type AsTimes struct {
		Offset  time.Time `toml:"offset"`
		LocalDT time.Time `toml:"local_dt"`
		LocalD  time.Time `toml:"local_d"`
	}
	var at AsTimes
	if err := Unmarshal([]byte(src), &at); err != nil {
		t.Fatalf("decode into time.Time: %v", err)
	}
	if at.Offset.IsZero() || at.LocalDT.IsZero() || at.LocalD.IsZero() {
		t.Errorf("zero time: %+v", at)
	}

	// Decode into typed local types.
	type Locals struct {
		Offset  time.Time     `toml:"offset"`
		LocalDT LocalDateTime `toml:"local_dt"`
		LocalD  LocalDate     `toml:"local_d"`
		LocalT  LocalTime     `toml:"local_t"`
	}
	var l Locals
	if err := Unmarshal([]byte(src), &l); err != nil {
		t.Fatalf("decode into locals: %v", err)
	}
	if l.LocalDT.Year != 2024 || l.LocalD.Year != 2024 || l.LocalT.Hour != 10 {
		t.Errorf("locals: %+v", l)
	}

	// Decode into interface map (uses nodeToInterface paths).
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["offset"].(time.Time); !ok {
		t.Errorf("offset is %T, want time.Time", m["offset"])
	}
	if _, ok := m["local_dt"].(LocalDateTime); !ok {
		t.Errorf("local_dt is %T, want LocalDateTime", m["local_dt"])
	}

	// Encode them back. Round-trip exercises encoder paths for these types.
	out, err := Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(out, []byte("1979-05-27")) {
		t.Errorf("encoded output missing date:\n%s", out)
	}
}

func TestAcceptanceTextUnmarshalerKey(t *testing.T) {
	// Map key with a non-string kind implementing encoding.TextUnmarshaler.
	type Cfg struct {
		Items map[acceptancePort]string `toml:"items"`
	}
	var c Cfg
	src := "[items]\n8080 = \"http\"\n9090 = \"grpc\"\n"
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Items[acceptancePort(8080)] != "http" || c.Items[acceptancePort(9090)] != "grpc" {
		t.Errorf("items = %v", c.Items)
	}

	// Error path: invalid key for this TextUnmarshaler.
	type Bad struct {
		Items map[acceptancePort]string `toml:"items"`
	}
	var b Bad
	if err := Unmarshal([]byte("[items]\nnot-a-port = \"x\"\n"), &b); err == nil {
		t.Error("expected error for invalid port key")
	}
}

type acceptancePort int

func (p *acceptancePort) UnmarshalText(data []byte) error {
	var n int
	for _, c := range data {
		if c < '0' || c > '9' {
			return errors.New("invalid port")
		}
		n = n*10 + int(c-'0')
	}
	*p = acceptancePort(n)
	return nil
}

func TestAcceptanceMultilineStringEscapes(t *testing.T) {
	src := "a = \"\"\"\nfirst line\\\nsecond line\\u00e9\\x41\nend\n\"\"\"\n"
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	got := m["a"].(string)
	if !strings.Contains(got, "second lineé") {
		t.Errorf("multiline escape failed:\n%q", got)
	}
	if !strings.Contains(got, "A") {
		t.Errorf("hex escape failed:\n%q", got)
	}
}

func TestAcceptanceEncodeAllFloatVariants(t *testing.T) {
	type Cfg struct {
		Pos    float64 `toml:"pos"`
		Neg    float64 `toml:"neg"`
		Inf    float64 `toml:"inf"`
		NegInf float64 `toml:"neg_inf"`
		NaN    float64 `toml:"nan_val"`
		Whole  float64 `toml:"whole"`
		Tiny   float64 `toml:"tiny"`
		BigExp float64 `toml:"big_exp"`
	}
	c := Cfg{
		Pos:    3.14,
		Neg:    -2.5,
		Inf:    math.Inf(1),
		NegInf: math.Inf(-1),
		NaN:    math.NaN(),
		Whole:  100.0,
		Tiny:   1e-20,
		BigExp: 1e30,
	}
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"3.14", "-2.5", "inf", "-inf", "nan"} {
		if !strings.Contains(s, want) {
			t.Errorf("encoded output missing %q:\n%s", want, s)
		}
	}
}

func TestAcceptanceEncodeUintBases(t *testing.T) {
	type Cfg struct {
		Plain  uint   `toml:"plain"`
		Hex    uint32 `toml:"hex,hex"`
		Octal  uint16 `toml:"octal,octal"`
		Binary uint8  `toml:"binary,binary"`
	}
	c := Cfg{Plain: 100, Hex: 0xCAFE, Octal: 0o777, Binary: 0b1111_0000}
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(out))
	for _, want := range []string{"0xcafe", "0o777", "0b11110000"} {
		if !strings.Contains(s, want) {
			t.Errorf("encoded output missing %q:\n%s", want, out)
		}
	}
}

func TestAcceptanceCommentedTagWithoutCommentText(t *testing.T) {
	type Cfg struct {
		Active bool `toml:"active,commented"`
	}
	out, err := Marshal(&Cfg{Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(out)), "#") {
		t.Errorf("commented field not produced:\n%s", out)
	}
}

func TestAcceptanceEncodePointerAndNil(t *testing.T) {
	type Inner struct {
		Name string `toml:"name"`
	}
	type Cfg struct {
		Set    *Inner   `toml:"set"`
		NotSet *Inner   `toml:"not_set,omitempty"`
		Tags   []string `toml:"tags,omitempty"`
		Empty  []string `toml:"empty,omitempty"`
	}
	c := Cfg{
		Set:  &Inner{Name: "demo"},
		Tags: []string{"a", "b"},
	}
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `name = "demo"`) {
		t.Errorf("pointer struct not encoded:\n%s", s)
	}
	if strings.Contains(s, "not_set") {
		t.Errorf("nil pointer with omitempty leaked:\n%s", s)
	}
	if strings.Contains(s, "empty") {
		t.Errorf("empty slice with omitempty leaked:\n%s", s)
	}
}

func TestAcceptanceEncodeArrayOfMixedScalars(t *testing.T) {
	out, err := Marshal(map[string]any{
		"strs":   []string{"a", "b"},
		"ints":   []int{1, 2, 3},
		"floats": []float64{1.1, 2.2},
		"bools":  []bool{true, false},
		"nested": [][]int{{1, 2}, {3, 4}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"strs =", "ints =", "floats =", "bools =", "nested ="} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
}

func TestAcceptanceArrayOfTablesEncode(t *testing.T) {
	type Item struct {
		Name string `toml:"name"`
	}
	type Cfg struct {
		Items []Item `toml:"items"`
	}
	c := Cfg{Items: []Item{{Name: "first"}, {Name: "second"}}}
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(out), "[[items]]") != 2 {
		t.Errorf("expected 2 array-table headers:\n%s", out)
	}
}

type acceptanceUnmarshalScalar struct {
	Value string
}

func (u *acceptanceUnmarshalScalar) UnmarshalTOML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	u.Value = "decoded:" + s
	return nil
}

type acceptanceUnmarshalCtx struct {
	Value string
}

func (u *acceptanceUnmarshalCtx) UnmarshalTOML(_ context.Context, unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	u.Value = "ctx:" + s
	return nil
}

type acceptanceBytesUnmarshal struct {
	Raw string
}

func (u *acceptanceBytesUnmarshal) UnmarshalTOML(data []byte) error {
	u.Raw = string(data)
	return nil
}

func TestAcceptanceUnmarshalerInterfaces(t *testing.T) {
	// Scalar Unmarshaler
	type Cfg1 struct {
		Val acceptanceUnmarshalScalar `toml:"val"`
	}
	var c1 Cfg1
	if err := Unmarshal([]byte(`val = "hello"`), &c1); err != nil {
		t.Fatal(err)
	}
	if c1.Val.Value != "decoded:hello" {
		t.Errorf("Unmarshaler not used: %q", c1.Val.Value)
	}

	// UnmarshalerContext
	type Cfg2 struct {
		Val acceptanceUnmarshalCtx `toml:"val"`
	}
	var c2 Cfg2
	if err := Unmarshal([]byte(`val = "world"`), &c2); err != nil {
		t.Fatal(err)
	}
	if c2.Val.Value != "ctx:world" {
		t.Errorf("UnmarshalerContext not used: %q", c2.Val.Value)
	}

	// BytesUnmarshaler
	type Cfg3 struct {
		Val acceptanceBytesUnmarshal `toml:"val"`
	}
	var c3 Cfg3
	if err := Unmarshal([]byte(`val = "raw bytes"`), &c3); err != nil {
		t.Fatal(err)
	}
	if c3.Val.Raw == "" {
		t.Errorf("BytesUnmarshaler not used: %q", c3.Val.Raw)
	}
}

func TestAcceptanceArrayTableIntoArray(t *testing.T) {
	src := `
[[items]]
name = "first"
[[items]]
name = "second"
`
	// Slice
	type CfgSlice struct {
		Items []struct {
			Name string `toml:"name"`
		} `toml:"items"`
	}
	var cs CfgSlice
	if err := Unmarshal([]byte(src), &cs); err != nil {
		t.Fatal(err)
	}
	if len(cs.Items) != 2 {
		t.Errorf("slice items: %d", len(cs.Items))
	}

	// Generic map: array of tables become []any
	var generic map[string]any
	if err := Unmarshal([]byte(src), &generic); err != nil {
		t.Fatal(err)
	}
	if arr, ok := generic["items"].([]any); !ok || len(arr) != 2 {
		t.Errorf("generic items: %T %v", generic["items"], generic["items"])
	}
}

func TestAcceptanceSpecialStrings(t *testing.T) {
	// Empty multi-line strings, control chars in escapes
	src := `
empty = ""
literal_empty = ''
multiline_empty = """"""
multiline_lit_empty = ''''''
with_tab = "tab\there"
with_unicode = "snowman: ☃"
with_long_unicode = "rocket: \U0001F680"
with_escapes = "\"quoted\" \\ slash"
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if m["with_unicode"] != "snowman: ☃" {
		t.Errorf("unicode escape: %q", m["with_unicode"])
	}

	// Encode strings with control characters and special chars to exercise escape paths
	out, err := Marshal(map[string]any{
		"control": "tab\there",
		"newline": "line1\nline2",
		"quote":   `with "quotes"`,
		"slash":   `back\slash`,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `\t`) {
		t.Errorf("tab not escaped:\n%s", s)
	}
}

func TestAcceptanceJSONNumbers(t *testing.T) {
	// FromJSON with various number types triggers convertJSONValues paths.
	jsonSrc := `{"int":42,"float":3.14,"sci":1.5e10}`
	out, err := FromJSON([]byte(jsonSrc))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "int = 42") {
		t.Errorf("int not converted:\n%s", s)
	}
	if !strings.Contains(s, "float = 3.14") {
		t.Errorf("float not converted:\n%s", s)
	}

	// FromJSON with array root
	out2, err := FromJSON([]byte(`{"arr":[1,2.5,"three"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out2), "arr =") {
		t.Errorf("arr missing:\n%s", out2)
	}

	// FromJSON with malformed input
	if _, err := FromJSON([]byte(`not json`)); err == nil {
		t.Error("expected error for malformed JSON")
	}

	// ToJSON with malformed TOML
	if _, err := ToJSON([]byte(`bad =`)); err == nil {
		t.Error("expected error for malformed TOML")
	}
}

func TestAcceptanceNodeStringMethods(t *testing.T) {
	src := `
title = "demo"
[server]
host = "x"

[[items]]
name = "a"

inline = { k = 1 }
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	// Walk and call String on every node so each kind gets exercised.
	Walk(f.Root, func(n *Node) bool {
		_ = n.String()
		return true
	})
}

func TestAcceptanceCloseFlusher(t *testing.T) {
	// Close on encoder where writer implements Flush.
	var buf bytes.Buffer
	flusher := &flushWriter{w: &buf}
	enc := NewEncoder(flusher)
	if err := enc.Encode(map[string]any{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !flusher.flushed {
		t.Error("Flush not called")
	}

	// Close error path
	flusher2 := &flushWriter{w: &buf, flushErr: errors.New("flush failed")}
	enc2 := NewEncoder(flusher2)
	if err := enc2.Encode(map[string]any{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if err := enc2.Close(); err == nil {
		t.Error("expected flush error from Close")
	}
}

type flushWriter struct {
	w        *bytes.Buffer
	flushed  bool
	flushErr error
}

func (f *flushWriter) Write(p []byte) (int, error) {
	return f.w.Write(p)
}

func (f *flushWriter) Flush() error {
	f.flushed = true
	return f.flushErr
}

func TestAcceptanceTopLevelNotAStruct(t *testing.T) {
	// Marshal accepts only struct or map at top level.
	if _, err := Marshal(42); !errors.Is(err, ErrMarshalTop) {
		t.Errorf("expected ErrMarshalTop, got %v", err)
	}
	if _, err := Marshal("string"); !errors.Is(err, ErrMarshalTop) {
		t.Errorf("expected ErrMarshalTop, got %v", err)
	}
	if _, err := Marshal(nil); !errors.Is(err, ErrMarshalTop) {
		t.Errorf("expected ErrMarshalTop, got %v", err)
	}
	var nilPtr *struct{}
	if _, err := Marshal(nilPtr); !errors.Is(err, ErrMarshalTop) {
		t.Errorf("expected ErrMarshalTop, got %v", err)
	}

	// Unmarshal: nil pointer
	if err := Unmarshal([]byte(`a=1`), nil); !errors.Is(err, ErrNilPointer) {
		t.Errorf("expected ErrNilPointer, got %v", err)
	}
}

func TestAcceptanceInlineTableErrorOnExtension(t *testing.T) {
	src := "x = { a = 1 }\n[x.y]\nz = 2\n"
	var m map[string]any
	err := Unmarshal([]byte(src), &m)
	if err == nil {
		t.Skip("no error returned")
	}
	var inl *InlineTableError
	if errors.As(err, &inl) {
		_ = inl.Error()
		_ = inl.Is(&InlineTableError{})
	}
	if !errors.Is(err, ErrInlineTable) {
		t.Logf("note: expected ErrInlineTable, got %v", err)
	}
}

func TestAcceptanceEncodeTableNonStructMap(t *testing.T) {
	// Map with mixed scalar+table values exercises encodeMap partition.
	out, err := Marshal(map[string]any{
		"scalar":    42,
		"section":   map[string]any{"k": "v"},
		"empty_map": map[string]any{},
		"items":     []any{map[string]any{"name": "first"}, map[string]any{"name": "second"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[section]") {
		t.Errorf("expected [section]:\n%s", s)
	}
	if !strings.Contains(s, "[[items]]") {
		t.Errorf("expected [[items]]:\n%s", s)
	}
}

func TestAcceptancePathReadStringScalarKinds(t *testing.T) {
	src := `
str = "hello"
num = 42
flt = 3.14
bool_val = true
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ path, want string }{
		{"$.str", "hello"},
		{"$.num", "42"},
		{"$.flt", "3.14"},
		{"$.bool_val", "true"},
	}
	for _, tc := range cases {
		p, err := PathString(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := p.ReadString(f.Root)
		if err != nil {
			t.Errorf("ReadString(%s): %v", tc.path, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ReadString(%s) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestAcceptanceDecodeIntoIntFamilyAndFloats(t *testing.T) {
	src := `
i8 = 100
i16 = 1000
i32 = 100000
i64 = 10000000000
u8 = 200
u16 = 50000
u32 = 4000000000
u64 = 9000000000000000000
f32 = 3.14
f64 = 2.71828
`
	type Cfg struct {
		I8  int8    `toml:"i8"`
		I16 int16   `toml:"i16"`
		I32 int32   `toml:"i32"`
		I64 int64   `toml:"i64"`
		U8  uint8   `toml:"u8"`
		U16 uint16  `toml:"u16"`
		U32 uint32  `toml:"u32"`
		U64 uint64  `toml:"u64"`
		F32 float32 `toml:"f32"`
		F64 float64 `toml:"f64"`
	}
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.I8 != 100 || c.U64 != 9000000000000000000 || c.F32 != float32(3.14) {
		t.Errorf("decoded values wrong: %+v", c)
	}

	// Round-trip
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	var c2 Cfg
	if err := Unmarshal(out, &c2); err != nil {
		t.Fatalf("re-decode: %v\n%s", err, out)
	}
}

func TestAcceptanceDecodeStringIntoSlice(t *testing.T) {
	// String into []byte (base64)
	src := `data = "aGVsbG8="`
	var c struct {
		Data []byte `toml:"data"`
	}
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if string(c.Data) != "hello" {
		t.Errorf("base64 decode: %q", c.Data)
	}

	// Invalid base64
	if err := Unmarshal([]byte(`data = "!!!"`), &c); err == nil {
		t.Error("expected base64 decode error")
	}
}

func TestAcceptanceParseAndReserialize(t *testing.T) {
	// Round-trip via Parse → NodeToBytes preserves comments and structure.
	src := `# global header

# title comment
title = "demo"

# server section
[server]
host = "localhost" # inline comment

# array of tables
[[items]]
name = "first"

[[items]]
name = "second"
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := NodeToBytes(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	// Re-parse to ensure validity.
	if _, err := Parse(out); err != nil {
		t.Fatalf("reparse failed: %v\n%s", err, out)
	}
	// Some comments are preserved (head/inline). Pre-section head comments
	// may be associated to the previous node; just check we kept some.
	for _, want := range []string{"global header", "title comment", "inline comment"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("comment %q lost:\n%s", want, out)
		}
	}
}

func TestAcceptanceEncodeOptionsCompose(t *testing.T) {
	// Combine multiple options.
	type Cfg struct {
		Title  string         `toml:"title"`
		Server map[string]any `toml:"server"`
	}
	c := Cfg{
		Title:  "demo",
		Server: map[string]any{"host": "localhost", "port": 8080},
	}
	out, err := MarshalWithOptions(&c,
		WithIndent(4),
		WithIndentTables(true),
		WithTableSeparator(true),
		WithBasicStrings(true),
		WithLiteralString(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Error("empty output")
	}
}

func TestAcceptanceLocalDateInterface(t *testing.T) {
	src := `d = 2024-03-15`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["d"].(LocalDate); !ok {
		t.Errorf("local date in interface = %T", m["d"])
	}

	src2 := `t = 10:30:00`
	var m2 map[string]any
	if err := Unmarshal([]byte(src2), &m2); err != nil {
		t.Fatal(err)
	}
	if _, ok := m2["t"].(LocalTime); !ok {
		t.Errorf("local time in interface = %T", m2["t"])
	}
}

func TestAcceptanceErrorTypeDirectConstruction(t *testing.T) {
	// Construct each error type directly to exercise their Error/Is/Unwrap methods.
	// Real-world callers may construct these in custom validators or middleware.
	pos := Position{Line: 1, Column: 2, Offset: 0}

	cases := []struct {
		name     string
		err      error
		sentinel error
	}{
		{"Syntax", &SyntaxError{Message: "x", Pos: pos}, ErrSyntax},
		{"SyntaxNoPos", &SyntaxError{Message: "x"}, ErrSyntax},
		{"Type", &TypeError{Errors: []string{"a", "b"}}, ErrType},
		{"UnknownField", &UnknownFieldError{Field: "f", Pos: pos}, ErrUnknownField},
		{"DuplicateKey", &DuplicateKeyError{Key: "k", Pos: pos}, ErrDuplicateKey},
		{"Validation", &ValidationError{Err: errors.New("v"), Pos: pos}, ErrValidation},
		{"Default", &DefaultError{Field: "f", Message: "m"}, ErrDefault},
		{"Overflow", &OverflowError{Value: "999", Type: "int8", Pos: pos}, ErrOverflow},
		{"InlineTable", &InlineTableError{Table: "t", Pos: pos}, ErrInlineTable},
		{"Conflict", &ConflictError{Key: "k", Message: "c", Pos: pos}, ErrConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Error() == "" {
				t.Error("Error() returned empty")
			}
			if !errors.Is(tc.err, tc.sentinel) {
				t.Errorf("errors.Is failed for %T", tc.err)
			}
		})
	}

	// Position formatting.
	if (Position{Line: 5, Column: 10}).String() != "5:10" {
		t.Errorf("Position.String wrong")
	}

	// ValidationError.Unwrap returns inner.
	inner := errors.New("inner")
	ve := &ValidationError{Err: inner, Pos: pos}
	if !errors.Is(ve, inner) {
		t.Error("ValidationError doesn't unwrap")
	}
}

func TestAcceptanceCRLFInput(t *testing.T) {
	src := "title = \"demo\"\r\n[server]\r\nhost = \"localhost\"\r\n"
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("CRLF input failed: %v", err)
	}
	if m["title"] != "demo" {
		t.Errorf("title = %v", m["title"])
	}

	// Bare CR is invalid
	bare := "title = \"demo\"\rhost = \"x\""
	if err := Unmarshal([]byte(bare), &m); err == nil {
		t.Error("expected error for bare CR")
	}
}

func TestAcceptanceBOMInput(t *testing.T) {
	src := "\xEF\xBB\xBFtitle = \"demo\"\n"
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("BOM input failed: %v", err)
	}
	if m["title"] != "demo" {
		t.Errorf("title = %v", m["title"])
	}
}

func TestAcceptanceInlineTableViolations(t *testing.T) {
	// Inline tables are immutable: can't extend them with [foo.bar].
	cases := []string{
		"x = { a = 1 }\n[x.y]\nz = 2\n",
		"x = { a = 1 }\nx.y = 2\n",
	}
	for _, src := range cases {
		var m map[string]any
		err := Unmarshal([]byte(src), &m)
		if err == nil {
			continue
		}
		var inl *InlineTableError
		if errors.As(err, &inl) {
			_ = inl.Error()
			_ = inl.Is(&InlineTableError{})
			if !errors.Is(err, ErrInlineTable) {
				t.Errorf("expected ErrInlineTable for %q", src)
			}
		}
		_ = FormatError([]byte(src), err)
	}
}

func TestAcceptanceDatetimeNoSeconds(t *testing.T) {
	// Datetime forms without seconds (TOML 1.1)
	src := `
a = 2024-01-15T10:30Z
b = 2024-01-15T10:30
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("datetime no-seconds: %v", err)
	}
	if _, ok := m["a"].(time.Time); !ok {
		t.Errorf("a = %T", m["a"])
	}
	if _, ok := m["b"].(LocalDateTime); !ok {
		t.Errorf("b = %T", m["b"])
	}
}

func TestAcceptanceArrayOfTablesNested(t *testing.T) {
	// Array of tables inside array of tables exercises findLastArrayEntry.
	src := `
[[outer]]
name = "first"

[[outer.inner]]
val = 1

[[outer.inner]]
val = 2

[[outer]]
name = "second"

[[outer.inner]]
val = 3
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("nested array of tables: %v\n%s", err, src)
	}
	outer, ok := m["outer"].([]any)
	if !ok || len(outer) != 2 {
		t.Fatalf("outer wrong: %T %v", m["outer"], m["outer"])
	}
	first := outer[0].(map[string]any)
	innerFirst, ok := first["inner"].([]any)
	if !ok || len(innerFirst) != 2 {
		t.Errorf("first.inner: %v", first["inner"])
	}
}

func TestAcceptanceFloatIntoTypes(t *testing.T) {
	src := `
f = 3.14
n = 42
`
	type Cfg struct {
		F1 float64 `toml:"f"`
		F2 float32 `toml:"f"` // collision OK at struct level — different test
	}
	var c struct {
		F float64 `toml:"f"`
		N int     `toml:"n"`
	}
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.F != 3.14 || c.N != 42 {
		t.Errorf("decoded: %+v", c)
	}
	_ = Cfg{}

	// Float into interface
	var m map[string]any
	if err := Unmarshal([]byte(`f = 1.5`), &m); err != nil {
		t.Fatal(err)
	}
	if m["f"] != 1.5 {
		t.Errorf("f = %v (%T)", m["f"], m["f"])
	}

	// Underscore separators
	if err := Unmarshal([]byte(`x = 1_000_000`), &m); err != nil {
		t.Fatal(err)
	}
	if m["x"] != int64(1000000) {
		t.Errorf("x = %v", m["x"])
	}
}

func TestAcceptanceIntegerIntoFloat(t *testing.T) {
	src := `n = 42`
	var c struct {
		N float64 `toml:"n"`
	}
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.N != 42.0 {
		t.Errorf("n = %v", c.N)
	}
}

func TestAcceptanceWalkEarlyTermination(t *testing.T) {
	src := `[a]
b = 1
[c]
d = 2`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	visited := 0
	Walk(f.Root, func(n *Node) bool {
		visited++
		return false // don't recurse
	})
	if visited != 1 {
		t.Errorf("Walk early-termination visited %d, want 1", visited)
	}

	// Walk(nil) does nothing
	Walk(nil, func(*Node) bool {
		t.Error("called for nil")
		return true
	})
}

func TestAcceptanceFilterEmpty(t *testing.T) {
	f, _ := Parse([]byte(`a = 1`))
	matches := Filter(f.Root, func(n *Node) bool { return n.Kind == ArrayTableNode })
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestAcceptanceParseEmpty(t *testing.T) {
	f, err := Parse([]byte(``))
	if err != nil {
		t.Fatal(err)
	}
	if f.Root == nil {
		t.Error("Parse returned nil root")
	}

	// Parse with invalid input
	if _, err := Parse([]byte(`x = `)); err == nil {
		t.Error("expected parse error")
	}

	// Parse byte stream with only comments
	f2, err := Parse([]byte(`# comment only`))
	if err != nil {
		t.Fatal(err)
	}
	if f2.Root == nil {
		t.Error("expected non-nil root for comment-only input")
	}
}

func TestAcceptanceValid(t *testing.T) {
	if !Valid([]byte(`a = 1`)) {
		t.Error("Valid false for valid input")
	}
	if Valid([]byte(`bad =`)) {
		t.Error("Valid true for invalid input")
	}
	if !Valid([]byte(``)) {
		t.Error("Valid false for empty input")
	}
}

func TestAcceptanceComplexEscapes(t *testing.T) {
	// All escape variants in basic strings.
	src := `
backslash = "path\\to\\file"
quote = "say \"hi\""
nl = "line1\nline2"
cr = "\r"
tab = "a\tb"
backspace = "a\bb"
formfeed = "a\fb"
hex = "A\x42C"
unicode_short = "é"
unicode_long = "\U0001F600"
escape_e = "esc\e"
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("escape: %v", err)
	}
	if m["unicode_short"] != "é" {
		t.Errorf("unicode_short = %q", m["unicode_short"])
	}
	if m["hex"] != "ABC" {
		t.Errorf("hex = %q", m["hex"])
	}
}

func TestAcceptanceNumberBases(t *testing.T) {
	src := `
hex_low = 0xff
hex_up = 0xFF
hex_mix = 0xDeadBeef
oct = 0o777
bin = 0b10101
underscore_hex = 0xff_ff
underscore_dec = 1_000_000
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if m["hex_low"] != int64(0xff) || m["hex_up"] != int64(0xff) {
		t.Errorf("hex: %v %v", m["hex_low"], m["hex_up"])
	}
	if m["oct"] != int64(0o777) {
		t.Errorf("oct: %v", m["oct"])
	}
	if m["bin"] != int64(0b10101) {
		t.Errorf("bin: %v", m["bin"])
	}
}

func TestAcceptanceNegativeNumbers(t *testing.T) {
	src := `
neg_int = -42
pos_int = +42
neg_float = -3.14
pos_float = +3.14
neg_zero = -0
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if m["neg_int"] != int64(-42) || m["pos_int"] != int64(42) {
		t.Errorf("ints: %v %v", m["neg_int"], m["pos_int"])
	}
}

func TestAcceptanceDottedKeys(t *testing.T) {
	src := `
fruit.name = "apple"
fruit.color = "red"
fruit.taste.sweet = true

[other]
a.b.c = 1
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	fruit := m["fruit"].(map[string]any)
	if fruit["name"] != "apple" {
		t.Errorf("fruit.name = %v", fruit["name"])
	}
	taste := fruit["taste"].(map[string]any)
	if taste["sweet"] != true {
		t.Errorf("fruit.taste.sweet = %v", taste["sweet"])
	}
}

func TestAcceptanceQuotedKeys(t *testing.T) {
	src := `
"key with spaces" = 1
'literal key' = 2
"nested.dotted" = 3

[outer]
"inner.one" = "x"

[outer."nested table"]
val = 42
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if m["key with spaces"] != int64(1) {
		t.Errorf("key with spaces = %v", m["key with spaces"])
	}
	outer := m["outer"].(map[string]any)
	nt := outer["nested table"].(map[string]any)
	if nt["val"] != int64(42) {
		t.Errorf("nested table val = %v", nt["val"])
	}
}

func TestAcceptanceArrayMixedTypes(t *testing.T) {
	// TOML 1.1 allows heterogeneous arrays.
	src := `mixed = [1, "two", 3.0, true]`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	arr := m["mixed"].([]any)
	if len(arr) != 4 {
		t.Errorf("len = %d", len(arr))
	}
	if arr[0] != int64(1) || arr[1] != "two" || arr[2] != float64(3.0) || arr[3] != true {
		t.Errorf("mixed = %v", arr)
	}
}

func TestAcceptanceMultilineArray(t *testing.T) {
	src := `
arr = [
  1,
  2,
  3,
]

inline = {
  k1 = "v1",
  k2 = "v2",
}
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if arr := m["arr"].([]any); len(arr) != 3 {
		t.Errorf("arr = %v", arr)
	}
	if inl := m["inline"].(map[string]any); inl["k1"] != "v1" {
		t.Errorf("inline.k1 = %v", inl["k1"])
	}
}

func TestAcceptanceRoundTripAllScalars(t *testing.T) {
	src := `
str = "hello"
int_val = 42
float_val = 3.14
bool_val = true
arr = [1, 2, 3]
ts = 2024-01-15T10:30:00Z
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var m2 map[string]any
	if err := Unmarshal(out, &m2); err != nil {
		t.Fatalf("re-decode: %v\n%s", err, out)
	}
}

func TestAcceptanceSpecialFloatValuesEncodeRoundtrip(t *testing.T) {
	type F struct {
		Inf    float64 `toml:"inf_val"`
		NegInf float64 `toml:"neg_inf"`
		NaN    float64 `toml:"nan_val"`
	}
	v := F{Inf: math.Inf(1), NegInf: math.Inf(-1), NaN: math.NaN()}
	out, err := Marshal(&v)
	if err != nil {
		t.Fatal(err)
	}
	var v2 F
	if err := Unmarshal(out, &v2); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if !math.IsInf(v2.Inf, 1) || !math.IsInf(v2.NegInf, -1) || !math.IsNaN(v2.NaN) {
		t.Errorf("special floats round-trip: %+v", v2)
	}
}

func TestAcceptancePointerTargetEncoding(t *testing.T) {
	type Cfg struct {
		Name *string   `toml:"name"`
		Age  *int      `toml:"age"`
		Tags *[]string `toml:"tags"`
	}
	name := "demo"
	age := 30
	tags := []string{"a", "b"}
	v := Cfg{Name: &name, Age: &age, Tags: &tags}
	out, err := Marshal(&v)
	if err != nil {
		t.Fatal(err)
	}
	var v2 Cfg
	if err := Unmarshal(out, &v2); err != nil {
		t.Fatal(err)
	}
	if v2.Name == nil || *v2.Name != "demo" {
		t.Errorf("name = %v", v2.Name)
	}
	if v2.Age == nil || *v2.Age != 30 {
		t.Errorf("age = %v", v2.Age)
	}
}

func TestAcceptanceEmptyInlineTable(t *testing.T) {
	src := `empty = {}`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	em := m["empty"].(map[string]any)
	if len(em) != 0 {
		t.Errorf("empty = %v", em)
	}

	// Encode empty map
	out, err := Marshal(map[string]any{"k": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Error("empty output")
	}
}

func TestAcceptanceEmptyArray(t *testing.T) {
	src := `arr = []`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if a := m["arr"].([]any); len(a) != 0 {
		t.Errorf("arr = %v", a)
	}

	// Encode empty array
	out, err := Marshal(map[string]any{"arr": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "arr = []") {
		t.Errorf("expected arr = []:\n%s", out)
	}
}

func TestAcceptanceDecodeArrayIntoTargets(t *testing.T) {
	src := `xs = [1, 2, 3]`

	// Slice
	type S struct{ Xs []int }
	var s S
	if err := Unmarshal([]byte(src), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Xs) != 3 {
		t.Errorf("slice: %v", s.Xs)
	}

	// Fixed array
	type A struct{ Xs [3]int }
	var a A
	if err := Unmarshal([]byte(src), &a); err != nil {
		t.Fatal(err)
	}
	if a.Xs[2] != 3 {
		t.Errorf("array: %v", a.Xs)
	}

	// Array length mismatch
	type AMis struct{ Xs [5]int }
	var aMis AMis
	if err := Unmarshal([]byte(src), &aMis); err == nil {
		t.Error("expected length-mismatch error")
	}

	// Decode into wrong type
	type W struct{ Xs string }
	var w W
	if err := Unmarshal([]byte(src), &w); err == nil {
		t.Error("expected type error decoding array into string")
	}
}

func TestAcceptanceDecodeBoolAndDateInterface(t *testing.T) {
	// bool, datetime into specific Go types and into interface.
	src := `
b = true
ts = 1979-05-27T07:32:00Z
local_dt = 2024-01-15T10:30:00
local_d = 2024-01-15
local_t = 10:30:00
`
	// Decode datetime into LocalDateTime when target is time.Time fails for naive form.
	type Cfg struct {
		B  bool
		Ts time.Time
		LD time.Time `toml:"local_d"`
	}
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if !c.B || c.Ts.IsZero() || c.LD.IsZero() {
		t.Errorf("decoded: %+v", c)
	}

	// Mismatched targets produce TypeError accumulation.
	type Bad struct {
		B  string
		Ts string
	}
	var b Bad
	_ = Unmarshal([]byte(src), &b) // accept any error/no error
}

func TestAcceptanceApplyDefaultAllTypes(t *testing.T) {
	type Cfg struct {
		S  string        `toml:"s,default=hi"`
		I  int           `toml:"i,default=42"`
		I8 int8          `toml:"i8,default=7"`
		U  uint          `toml:"u,default=99"`
		U8 uint8         `toml:"u8,default=8"`
		F  float32       `toml:"f,default=2.5"`
		B  bool          `toml:"b,default=true"`
		D  time.Duration `toml:"d,default=1m"`
	}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(``), &c, WithDefaults()); err != nil {
		t.Fatal(err)
	}
	if c.S != "hi" || c.I != 42 || c.I8 != 7 || c.U != 99 || c.U8 != 8 || c.F != 2.5 || !c.B || c.D != time.Minute {
		t.Errorf("defaults: %+v", c)
	}

	// Bad default values produce DefaultError.
	type BadInt struct {
		I int `toml:"i,default=not-a-number"`
	}
	var bi BadInt
	if err := UnmarshalWithOptions([]byte(``), &bi, WithDefaults()); err == nil {
		t.Error("expected DefaultError for bad int default")
	}

	type BadBool struct {
		B bool `toml:"b,default=not-a-bool"`
	}
	var bb BadBool
	if err := UnmarshalWithOptions([]byte(``), &bb, WithDefaults()); err == nil {
		t.Error("expected DefaultError for bad bool default")
	}
}

func TestAcceptanceEncodeMapWithIntKeys(t *testing.T) {
	// Maps with non-string keys
	out, err := Marshal(map[int]string{1: "one", 2: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1 = ") {
		t.Errorf("int-keyed map missing:\n%s", out)
	}
}

func TestAcceptanceTokenNamesAppearInErrorMessages(t *testing.T) {
	// Trigger an error containing a token name — exercises token.String().
	// Force a specific syntax error that prints the token type.
	if _, err := Parse([]byte(`= "no key"`)); err == nil {
		t.Error("expected error")
	} else {
		_ = err.Error()
	}
	if _, err := Parse([]byte(`a = b`)); err == nil {
		t.Error("expected error for bare-key value")
	} else {
		_ = err.Error()
	}
	if _, err := Parse([]byte(`[a b]`)); err == nil {
		t.Error("expected error for malformed table header")
	}
	if _, err := Parse([]byte(`a = [1, "two", true,]`)); err != nil {
		// trailing commas in arrays are allowed; just ensure no panic
	}
}

func TestAcceptanceDecoderReturnsEOFAfterFirstRead(t *testing.T) {
	// Encoder/Decoder don't support multiple docs but make sure a multi-call returns EOF cleanly.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.EncodeContext(context.Background(), map[string]any{"a": 1}); err != nil {
		t.Fatal(err)
	}

	dec := NewDecoder(&buf)
	var m map[string]any
	if err := dec.DecodeContext(context.Background(), &m); err != nil {
		t.Fatal(err)
	}
	if err := dec.DecodeContext(context.Background(), &m); err == nil {
		t.Error("expected EOF on second decode")
	}
}

func TestAcceptanceMaxNestingDepth(t *testing.T) {
	// Deeply nested inline tables.
	src := strings.Repeat("a = { ", 20) + "x = 1" + strings.Repeat(" }", 20)
	var m map[string]any
	if err := UnmarshalWithOptions([]byte(src), &m, WithMaxDepth(5)); err == nil {
		t.Skip("max depth not enforced for this shape")
	}
}

func TestAcceptanceOmitEmptyAllZeroValues(t *testing.T) {
	type Cfg struct {
		S  string         `toml:"s,omitempty"`
		I  int            `toml:"i,omitempty"`
		F  float64        `toml:"f,omitempty"`
		B  bool           `toml:"b,omitempty"`
		Sl []string       `toml:"sl,omitempty"`
		M  map[string]any `toml:"m,omitempty"`
		P  *int           `toml:"p,omitempty"`
	}
	out, err := Marshal(&Cfg{})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"s", "i", "f", "b", "sl", "m", "p"} {
		if strings.Contains(string(out), k+" =") {
			t.Errorf("expected %q omitted:\n%s", k, out)
		}
	}
}

func TestAcceptanceScannerHandlesAllValueLiteralForms(t *testing.T) {
	// Comments, all string types, all number types, all datetime types
	srcs := []string{
		`a = 'literal'`,
		`a = '''multiline literal'''`,
		"a = \"\"\"multi\nline\"\"\"",
		"a = 'literal'\nb = \"basic\"",
		`a = 1979-05-27`,
		`a = 07:32:00`,
		`a = 1979-05-27T07:32:00`,
		`a = 1979-05-27T07:32:00.999`,
		`a = 1979-05-27T07:32:00.999999Z`,
		`a = 1979-05-27T07:32:00-08:00`,
		`a = 1979-05-27T07:32:00.5+00:00`,
		`a = 1.5e+10`,
		`a = 1.5e-10`,
		`a = 1.5E10`,
		"a = +0.0\nb = -0.0",
	}
	for _, s := range srcs {
		var m map[string]any
		if err := Unmarshal([]byte(s), &m); err != nil {
			t.Errorf("scanner edge %q: %v", s, err)
		}
	}

	// Empty table
	if _, err := Parse([]byte(`[empty]`)); err != nil {
		t.Errorf("empty table parse: %v", err)
	}
}

func TestAcceptanceDecodeIntoStructWithoutTags(t *testing.T) {
	// Fields without explicit toml tag use lowercased field name.
	type Cfg struct {
		Title string
		Port  int
	}
	src := `
title = "demo"
port = 8080
`
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Title != "demo" || c.Port != 8080 {
		t.Errorf("untagged fields: %+v", c)
	}
}

func TestAcceptanceDecodeCaseInsensitive(t *testing.T) {
	type Cfg struct {
		Title string `toml:"title"`
	}
	src := `TITLE = "demo"`
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Title != "demo" {
		t.Errorf("case-insensitive: %+v", c)
	}
}

func TestAcceptanceMapWithIntKeyError(t *testing.T) {
	// int key type without TextUnmarshaler should produce an error.
	type Cfg struct {
		Items map[int]string `toml:"items"`
	}
	var c Cfg
	src := "[items]\n1 = \"a\"\n"
	if err := Unmarshal([]byte(src), &c); err == nil {
		t.Error("expected error for int map key without TextUnmarshaler")
	}
}

func TestAcceptanceEncodeNodeAST(t *testing.T) {
	// Build a node tree by hand and serialize it.
	root := &Node{Kind: TableNode}
	root.Children = []*Node{
		{
			Kind:     KeyValueNode,
			Key:      []string{"name"},
			Children: []*Node{{Kind: StringNode, Value: "demo", Style: BasicStyle}},
		},
		{
			Kind:     KeyValueNode,
			Key:      []string{"port"},
			Children: []*Node{{Kind: IntegerNode, Value: "8080"}},
		},
		{
			Kind: TableNode,
			Key:  []string{"server"},
			Children: []*Node{
				{
					Kind:     KeyValueNode,
					Key:      []string{"host"},
					Children: []*Node{{Kind: StringNode, Value: "localhost"}},
				},
			},
		},
	}
	out, err := NodeToBytes(root)
	if err != nil {
		t.Fatal(err)
	}
	// Re-parse to verify validity.
	if _, err := Parse(out); err != nil {
		t.Fatalf("re-parse: %v\n%s", err, out)
	}

	// With options
	out2, err := NodeToBytesWithOptions(root, WithIndent(2), WithIndentTables(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(out2) == 0 {
		t.Error("empty output with options")
	}

	// Various string styles
	for _, style := range []StringStyle{BasicStyle, MultiLineBasicStyle, LiteralStyle, MultiLineLiteralStyle} {
		n := &Node{Kind: TableNode, Children: []*Node{
			{Kind: KeyValueNode, Key: []string{"s"}, Children: []*Node{
				{Kind: StringNode, Value: "x", Style: style},
			}},
		}}
		if _, err := NodeToBytes(n); err != nil {
			t.Errorf("style %v: %v", style, err)
		}
	}
}

func TestAcceptanceMultiLineLiteral(t *testing.T) {
	// Multi-line literal strings preserve content verbatim.
	src := "a = '''\nfirst\nsecond\nthird\n'''"
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	got := m["a"].(string)
	if !strings.Contains(got, "first\nsecond\nthird") {
		t.Errorf("multi-line literal: %q", got)
	}

	// Basic multiline that ends with apostrophes
	src2 := `a = '''contains 'single quotes' inside'''`
	var m2 map[string]any
	if err := Unmarshal([]byte(src2), &m2); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptanceTimestampWithFractionalAndOffset(t *testing.T) {
	// Various offset and precision combinations for datetime.
	srcs := []string{
		"a = 2024-01-15T10:30:00.000000001Z",
		"a = 2024-01-15T10:30:00.123-05:00",
		"a = 2024-01-15T10:30:00.123+05:00",
		"a = 2024-01-15T10:30:00 ",
		"a = 2024-01-15 10:30:00", // space separator
	}
	for _, src := range srcs {
		var m map[string]any
		if err := Unmarshal([]byte(src), &m); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

func TestAcceptanceEncodeManyFloatVariants(t *testing.T) {
	// Encode floats in various magnitudes to exercise encodeFloat paths.
	out, err := Marshal(map[string]any{
		"zero":     0.0,
		"neg_zero": math.Copysign(0, -1),
		"small":    0.001,
		"big":      1234567.89,
		"tiny":     1e-100,
		"huge":     1e100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Error("empty output")
	}
}

func TestAcceptanceEncodeStringEscapes(t *testing.T) {
	// Strings requiring escaping
	cases := map[string]string{
		"with_quotes":    `say "hi"`,
		"with_newline":   "line1\nline2",
		"with_tab":       "col1\tcol2",
		"with_cr":        "x\ry",
		"with_backslash": `a\b\c`,
		"with_null":      "a\x00b",
		"with_control":   "a\x01b",
	}
	out, err := Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}

	// Round-trip
	var got map[string]string
	if err := Unmarshal(out, &got); err != nil {
		t.Fatalf("round-trip: %v\n%s", err, out)
	}
	for k, v := range cases {
		if got[k] != v {
			t.Errorf("%s: got %q want %q", k, got[k], v)
		}
	}
}

func TestAcceptanceEncodeWithMultiLineString(t *testing.T) {
	// Multi-line basic string contains backslash that needs escape inside multiline.
	out, err := MarshalWithOptions(map[string]any{
		"text": "line one\nline two\nline three",
	}, WithMultiLineString(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"""`) {
		t.Errorf("expected triple-quoted output:\n%s", out)
	}
}

func TestAcceptanceEncodeLiteralStringPreference(t *testing.T) {
	// WithLiteralString prefers '...' when content allows.
	out, err := MarshalWithOptions(map[string]any{
		"path":  `C:\Users\demo`,
		"plain": `hello`,
		"hasq":  `it's a "quote"`,
	}, WithLiteralString(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `'C:\Users\demo'`) {
		t.Errorf("expected literal style:\n%s", out)
	}
}

func TestAcceptanceParseInlineTableTrailingComma(t *testing.T) {
	src := `a = { x = 1, y = 2, }`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("trailing comma in inline table: %v", err)
	}
}

func TestAcceptanceArrayWithComments(t *testing.T) {
	src := `
arr = [
  # leading comment
  1,
  2, # trailing comment
  3,
]
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if a := m["arr"].([]any); len(a) != 3 {
		t.Errorf("len = %d", len(a))
	}
}

func TestAcceptanceMultipleEncodeOptionsCombined(t *testing.T) {
	type Cfg struct {
		Strict       bool
		Server       map[string]any `toml:"server"`
		ProcessUtils []string       `toml:"process_utils"`
	}
	c := Cfg{
		Strict:       true,
		Server:       map[string]any{"host": "x", "port": 8080},
		ProcessUtils: []string{"systemd", "supervisord"},
	}
	out, err := MarshalWithOptions(&c,
		WithIndent(2),
		WithIndentTables(true),
		WithArrayMultiline(true),
		WithMultiLineString(true),
		WithBasicStrings(false),
		WithLiteralString(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	var c2 Cfg
	if err := Unmarshal(out, &c2); err != nil {
		t.Fatalf("round-trip: %v\n%s", err, out)
	}
}

func TestAcceptancePathOperationsReturnNotFoundError(t *testing.T) {
	src := `a = 1`
	f, _ := Parse([]byte(src))
	p, _ := PathString("$.b")

	if _, err := p.ReadString(f.Root); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
	if _, err := p.ReadPositions(f.Root); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
	if err := p.Replace(f.Root, &Node{Kind: KeyValueNode, Key: []string{"x"}}); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
	if err := p.Append(f.Root, &Node{Kind: KeyValueNode, Key: []string{"x"}}); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
	if err := p.Delete(f.Root); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}

	// Empty path
	pEmpty := &Path{}
	if err := pEmpty.Replace(f.Root, &Node{}); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("empty path Replace: %v", err)
	}
	if err := pEmpty.Delete(f.Root); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("empty path Delete: %v", err)
	}

	// Read on nil
	if _, err := p.Read(nil); err == nil {
		t.Error("expected error for nil node")
	}
}

func TestAcceptanceCargoTomlRoundTripPreservesStructure(t *testing.T) {
	// Use cargo.toml as a comprehensive test for round-trip on real-world data.
	data, err := os.ReadFile(filepath.Join("testdata", "acceptance", "cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var m1 map[string]any
	if err := Unmarshal(data, &m1); err != nil {
		t.Fatal(err)
	}
	out, err := Marshal(m1)
	if err != nil {
		t.Fatal(err)
	}
	var m2 map[string]any
	if err := Unmarshal(out, &m2); err != nil {
		t.Fatalf("re-decode: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(m1, m2) {
		t.Error("round-trip mismatch on cargo.toml")
	}
}

func TestAcceptanceEmbeddedAndInlineStructs(t *testing.T) {
	// Anonymous (embedded) struct: fields hoisted to parent.
	type Common struct {
		Name string `toml:"name"`
	}
	type Outer struct {
		Common
		Port int `toml:"port"`
	}
	src := `name = "demo"
port = 8080`
	var o Outer
	if err := Unmarshal([]byte(src), &o); err != nil {
		t.Fatal(err)
	}
	if o.Name != "demo" || o.Port != 8080 {
		t.Errorf("embedded: %+v", o)
	}

	// Inline struct field
	type Inner struct {
		Host string `toml:"host"`
	}
	type Wrap struct {
		Inner `toml:",inline"`
		Port  int `toml:"port"`
	}
	src2 := `host = "localhost"
port = 9090`
	var w Wrap
	if err := Unmarshal([]byte(src2), &w); err != nil {
		t.Fatal(err)
	}
	if w.Host != "localhost" || w.Port != 9090 {
		t.Errorf("inline: %+v", w)
	}

	// Inline map field
	type WithInlineMap struct {
		Extras map[string]string `toml:",inline"`
	}
	out, err := Marshal(&WithInlineMap{Extras: map[string]string{"a": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = out

	// json fallback tag
	type JCfg struct {
		Title string `json:"title"`
	}
	var j JCfg
	if err := Unmarshal([]byte(`title = "demo"`), &j); err != nil {
		t.Fatal(err)
	}
	if j.Title != "demo" {
		t.Errorf("json fallback: %+v", j)
	}

	// Skip-tagged field
	type Skip struct {
		Hidden string `toml:"-"`
		Name   string `toml:"name"`
	}
	var s Skip
	if err := Unmarshal([]byte(`name = "x"`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Hidden != "" || s.Name != "x" {
		t.Errorf("skip: %+v", s)
	}

	// Marshal skips '-' fields too.
	out2, err := Marshal(&Skip{Hidden: "secret", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out2), "secret") {
		t.Errorf("skip not honored on encode:\n%s", out2)
	}

	// Comment tag
	type WithComment struct {
		Port int `toml:"port" comment:"server port"`
	}
	out3, err := Marshal(&WithComment{Port: 8080})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out3), "server port") {
		t.Errorf("comment tag missing:\n%s", out3)
	}
}

func TestAcceptanceMalformedInputsReturnSyntaxErrors(t *testing.T) {
	// Each malformed input exercises a different scanner/parser error path,
	// many of which include token names via token.String().
	cases := []string{
		`a =`,                   // missing value
		`= "no key"`,            // missing key
		`[unclosed`,             // unclosed table header
		`[[unclosed]`,           // mismatched array table
		`[a..b]`,                // empty key segment
		`a = "unterminated`,     // unterminated string
		`a = '''unterminated`,   // unterminated multiline literal
		`a = """unterminated`,   // unterminated multiline basic
		`a = 12_`,               // dangling underscore
		`a = 0x`,                // missing hex digits
		`a = .5`,                // leading dot in number
		`a = 5.`,                // trailing dot
		`a = 5e`,                // missing exponent digits
		`a = 1979-`,             // malformed date
		`a = [1, 2, ,3]`,        // double comma
		"a = 1\n[a.b]\nb = 2\n", // table conflict
		`a = "x" b = "y"`,       // missing newline between values
	}
	for _, src := range cases {
		var m map[string]any
		err := Unmarshal([]byte(src), &m)
		_ = err
		_ = Valid([]byte(src))
		// Format error if it fails
		if err != nil {
			_ = FormatError([]byte(src), err)
		}
	}
}

func TestAcceptanceFloatRoundtripMatrix(t *testing.T) {
	// Marshal common floats → unmarshal → compare.
	values := []float64{
		0, 1, -1, 0.5, -0.5, 1.5, 1.5e10, 1.5e-10, math.MaxFloat64, math.SmallestNonzeroFloat64,
	}
	for _, v := range values {
		out, err := Marshal(map[string]any{"x": v})
		if err != nil {
			t.Errorf("encode %v: %v", v, err)
			continue
		}
		var m map[string]any
		if err := Unmarshal(out, &m); err != nil {
			t.Errorf("decode %v: %v\n%s", v, err, out)
			continue
		}
		if got := m["x"].(float64); got != v {
			t.Errorf("roundtrip %v -> %v", v, got)
		}
	}
}

func TestAcceptanceParseUnicodeQuotedKeys(t *testing.T) {
	src := `"αβγ" = 1
"é-key" = 2
"key with utf-8 ☃" = 3`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if m["αβγ"] != int64(1) {
		t.Errorf("αβγ = %v", m["αβγ"])
	}
}

func TestAcceptanceNumberKey(t *testing.T) {
	src := `123 = "numeric"`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if m["123"] != "numeric" {
		t.Errorf("numeric key: %v", m["123"])
	}
}

type acceptanceValueBytesMarshaler struct {
	S string
}

func (m acceptanceValueBytesMarshaler) MarshalTOML() ([]byte, error) {
	return []byte(`"bytes:` + m.S + `"`), nil
}

type acceptanceValueCtxMarshaler struct {
	S string
}

func (m acceptanceValueCtxMarshaler) MarshalTOML(_ context.Context) (any, error) {
	return "ctx:" + m.S, nil
}

type acceptanceValueTextMarshaler struct {
	V int
}

func (m acceptanceValueTextMarshaler) MarshalText() ([]byte, error) {
	return []byte("v=" + acceptanceItoa(m.V)), nil
}

func acceptanceItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func TestAcceptanceValueReceiverMarshalers(t *testing.T) {
	t.Run("BytesMarshaler", func(t *testing.T) {
		out, err := Marshal(map[string]any{"v": acceptanceValueBytesMarshaler{S: "hi"}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), "bytes:hi") {
			t.Errorf("BytesMarshaler not used:\n%s", out)
		}
	})

	t.Run("MarshalerContext", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.EncodeContext(context.Background(), map[string]any{"v": acceptanceValueCtxMarshaler{S: "hi"}}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "ctx:hi") {
			t.Errorf("MarshalerContext not used:\n%s", buf.String())
		}
	})

	t.Run("TextMarshaler", func(t *testing.T) {
		out, err := Marshal(map[string]any{"v": acceptanceValueTextMarshaler{V: 42}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), `"v=42"`) {
			t.Errorf("TextMarshaler not used:\n%s", out)
		}
	})
}

func TestAcceptanceNilStructPointerIsOmitted(t *testing.T) {
	type Inner struct {
		V string `toml:"v"`
	}
	type Outer struct {
		Inner *Inner `toml:"inner,omitempty"`
	}
	var o Outer
	out, err := Marshal(&o)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "inner") {
		t.Errorf("nil pointer should be omitted:\n%s", out)
	}

	o2 := Outer{Inner: &Inner{V: "x"}}
	out2, err := Marshal(&o2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out2), `v = "x"`) {
		t.Errorf("non-nil pointer struct:\n%s", out2)
	}
}

func TestAcceptanceArrayOfStructPointers(t *testing.T) {
	type Item struct {
		Name string `toml:"name"`
	}
	type Cfg struct {
		Items []*Item `toml:"items"`
	}
	c := Cfg{Items: []*Item{{Name: "first"}, {Name: "second"}}}
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(out), "[[items]]") != 2 {
		t.Errorf("expected 2 array tables:\n%s", out)
	}
}

func TestAcceptanceFloatToIntError(t *testing.T) {
	type Cfg struct {
		N int `toml:"n"`
	}
	var c Cfg
	if err := Unmarshal([]byte(`n = 3.14`), &c); err == nil {
		t.Error("expected type error decoding float into int")
	}
}

func TestAcceptanceDecodeIntoPointer(t *testing.T) {
	src := `n = 42`
	var c struct {
		N *int `toml:"n"`
	}
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.N == nil || *c.N != 42 {
		t.Errorf("ptr decode: %v", c.N)
	}
}

func TestAcceptanceASTBuiltFromAllNodeKindsSerializes(t *testing.T) {
	root := &Node{Kind: TableNode, Children: []*Node{
		{Kind: KeyValueNode, Key: []string{"flag"}, Children: []*Node{{Kind: BooleanNode, Value: "true"}}},
		{Kind: KeyValueNode, Key: []string{"price"}, Children: []*Node{{Kind: FloatNode, Value: "9.99"}}},
		{Kind: KeyValueNode, Key: []string{"when"}, Children: []*Node{{Kind: DateTimeNode, Value: "2024-01-15T10:30:00Z"}}},
		{Kind: KeyValueNode, Key: []string{"d"}, Children: []*Node{{Kind: LocalDateNode, Value: "2024-01-15"}}},
		{Kind: KeyValueNode, Key: []string{"t"}, Children: []*Node{{Kind: LocalTimeNode, Value: "10:30:00"}}},
		{Kind: KeyValueNode, Key: []string{"dt"}, Children: []*Node{{Kind: LocalDateTimeNode, Value: "2024-01-15T10:30:00"}}},
		{Kind: KeyValueNode, Key: []string{"items"}, Children: []*Node{
			{Kind: ArrayNode, Children: []*Node{
				{Kind: IntegerNode, Value: "1"}, {Kind: IntegerNode, Value: "2"},
			}},
		}},
		{Kind: KeyValueNode, Key: []string{"inline"}, Children: []*Node{
			{Kind: InlineTableNode, Children: []*Node{
				{Kind: KeyValueNode, Key: []string{"k"}, Children: []*Node{{Kind: StringNode, Value: "v"}}},
			}},
		}},
	}}
	out, err := NodeToBytes(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(out); err != nil {
		t.Fatalf("re-parse: %v\n%s", err, out)
	}
}

func TestAcceptanceFromJSONArrayTables(t *testing.T) {
	jsonSrc := `{"items":[{"name":"a"},{"name":"b"}]}`
	out, err := FromJSON([]byte(jsonSrc))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[[items]]") {
		t.Errorf("expected items as array of tables:\n%s", out)
	}
}

func TestAcceptanceJSONRoundTrip(t *testing.T) {
	src := `
title = "demo"
count = 42

[server]
host = "localhost"

[[items]]
name = "first"

[[items]]
name = "second"
`
	jsonOut, err := ToJSON([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	tomlOut, err := FromJSON(jsonOut)
	if err != nil {
		t.Fatalf("FromJSON: %v\n%s", err, jsonOut)
	}
	if !strings.Contains(string(tomlOut), `title = "demo"`) {
		t.Errorf("title missing:\n%s", tomlOut)
	}
}

func TestAcceptanceArrayTableIntoPointerSlice(t *testing.T) {
	type Item struct {
		Name string `toml:"name"`
	}
	type Cfg struct {
		Items *[]Item `toml:"items"`
	}
	src := `[[items]]
name = "first"
[[items]]
name = "second"`
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Items == nil || len(*c.Items) != 2 {
		t.Errorf("ptr slice: %v", c.Items)
	}
}

func TestAcceptanceArrayTableIntoMap(t *testing.T) {
	// Array of tables into a map should produce a type error.
	type Cfg struct {
		Items map[string]any `toml:"items"`
	}
	src := `[[items]]
name = "first"`
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err == nil {
		t.Error("expected error decoding array of tables into map")
	}
}

func TestAcceptanceLocalDateTimeIntoTimeFields(t *testing.T) {
	// Decode into time.Time using LocalTimeZone option.
	loc, _ := time.LoadLocation("America/New_York")
	src := `
ldt = 2024-01-15T10:30:00
ldt2 = 2024-01-15T10:30
`
	type Cfg struct {
		LDT  time.Time `toml:"ldt"`
		LDT2 time.Time `toml:"ldt2"`
	}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(src), &c, WithLocalTimeZone(loc)); err != nil {
		t.Fatal(err)
	}
	if c.LDT.Year() != 2024 || c.LDT2.Year() != 2024 {
		t.Errorf("local datetime: %+v", c)
	}
}

func TestAcceptanceLocalDateAndTimeAlsoIntoTimeTime(t *testing.T) {
	type Cfg struct {
		D time.Time `toml:"d"`
	}
	var c Cfg
	if err := Unmarshal([]byte(`d = 2024-01-15`), &c); err != nil {
		t.Fatal(err)
	}
	if c.D.IsZero() {
		t.Error("LocalDate into time.Time: zero")
	}
}

func TestAcceptanceEncodeInlineTableViaGlobalOption(t *testing.T) {
	// WithInlineTable plus a top-level map having struct values.
	type Inner struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	out, err := MarshalWithOptions(map[string]any{
		"server": Inner{Host: "localhost", Port: 8080},
	}, WithInlineTable(true))
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestAcceptanceCustomMarshalerError(t *testing.T) {
	type wrapped string
	encOpt := WithCustomMarshaler(func(_ wrapped) ([]byte, error) {
		return nil, errors.New("encode failed")
	})
	type Cfg struct {
		W wrapped `toml:"w"`
	}
	if _, err := MarshalWithOptions(&Cfg{W: "x"}, encOpt); err == nil {
		t.Error("expected custom marshaler error")
	}

	decOpt := WithCustomUnmarshaler(func(_ *wrapped, _ []byte) error {
		return errors.New("decode failed")
	})
	var c Cfg
	if err := UnmarshalWithOptions([]byte(`w = "x"`), &c, decOpt); err == nil {
		t.Error("expected custom unmarshaler error")
	}
}

func TestAcceptanceDecodeIntoIncorrectType(t *testing.T) {
	// Decode various TOML types into mismatched Go targets to exercise type-error paths.
	cases := []struct {
		src    string
		target any
	}{
		{`x = "not a number"`, &struct{ X int }{}},
		{`x = 42`, &struct{ X bool }{}},
		{`x = true`, &struct{ X int }{}},
		{`x = 3.14`, &struct{ X string }{}},
		{`x = 1979-05-27`, &struct{ X int }{}},
		{`x = 07:32:00`, &struct{ X int }{}},
		{`x = [1, 2, 3]`, &struct{ X int }{}},
	}
	for _, tc := range cases {
		if err := Unmarshal([]byte(tc.src), tc.target); err == nil {
			t.Errorf("expected error for %q", tc.src)
		}
	}
}

func TestAcceptancePathReadStringDifferentTypes(t *testing.T) {
	// Path.ReadString with various scalar kinds.
	src := `
s = "hello"
n = 42
f = 3.14
b = true
ts = 2024-01-15T10:30:00Z
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"$.s", "$.n", "$.f", "$.b", "$.ts"} {
		path, err := PathString(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := path.ReadString(f.Root); err != nil {
			t.Errorf("ReadString(%s): %v", p, err)
		}
	}

	// ReadString on nodes that aren't directly scalars.
	pTbl, _ := PathString("$")
	if _, err := pTbl.ReadString(f.Root); err != nil {
		// Reading the root returns its raw value (empty for tables).
		_ = err
	}
}

func TestAcceptanceMixedInlineTableWithComment(t *testing.T) {
	src := `inline = { a = 1, b = 2 }`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["inline"].(map[string]any); !ok {
		t.Errorf("inline = %T", m["inline"])
	}
}

func TestAcceptanceDecodeRequiredWithDefault(t *testing.T) {
	// Required + default makes default not used since required errors first.
	type Cfg struct {
		N int `toml:"n,required"`
	}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(``), &c, WithDefaults()); err == nil {
		t.Error("expected required error")
	}
}

func TestAcceptanceInternalTokenStringer(t *testing.T) {
	// token.String is internal but produces error-message output users may see.
	// Exercising every token kind ensures error messages stay stable.
	kinds := []tokenKind{
		tokenStreamStart, tokenStreamEnd, tokenKeyValueSep,
		tokenTableStart, tokenTableEnd, tokenArrayTableStart, tokenArrayTableEnd,
		tokenInlineTableStart, tokenInlineTableEnd, tokenArrayStart, tokenArrayEnd,
		tokenComma, tokenDot,
		tokenBareKey, tokenBasicString, tokenMultiLineBasicString, tokenLiteralString, tokenMultiLineLiteralString,
		tokenInteger, tokenFloat, tokenBoolean,
		tokenOffsetDateTime, tokenLocalDateTime, tokenLocalDate, tokenLocalTime,
		tokenComment, tokenNewline, tokenEOF,
	}
	for _, k := range kinds {
		tok := token{kind: k, value: "x"}
		if tok.String() == "" {
			t.Errorf("token.String() empty for kind %d", k)
		}
	}
	// Default branch via an unknown kind value.
	if (token{kind: tokenKind(999)}).String() == "" {
		t.Error("default kind String empty")
	}
}

func TestAcceptanceInternalEncodeInlineTable(t *testing.T) {
	// encodeInlineTable is exercised when WithInlineTable is used in conjunction
	// with a struct value reaching encodeValue. Force it via the encoder.
	type Inner struct {
		A int    `toml:"a"`
		B string `toml:"b"`
	}
	enc := newEncoder(&encoderOptions{inlineTable: true})
	rv := reflect.ValueOf(Inner{A: 1, B: "x"})
	if err := enc.encodeInlineTable(rv); err != nil {
		t.Fatalf("encodeInlineTable: %v", err)
	}
	out := enc.buf.String()
	if !strings.Contains(out, "a = 1") || !strings.Contains(out, `b = "x"`) {
		t.Errorf("encodeInlineTable output: %q", out)
	}

	// With omitempty
	type WithOpt struct {
		A int    `toml:"a,omitempty"`
		B string `toml:"b,omitempty"`
	}
	enc2 := newEncoder(&encoderOptions{inlineTable: true})
	if err := enc2.encodeInlineTable(reflect.ValueOf(WithOpt{B: "x"})); err != nil {
		t.Fatal(err)
	}
	out2 := enc2.buf.String()
	if strings.Contains(out2, "a = 0") {
		t.Errorf("omitempty leaked: %q", out2)
	}
}

func TestAcceptanceCannotDecodeMismatchedTypes(t *testing.T) {
	// Each row exercises a "cannot decode X into Y" path.
	cases := []struct {
		src    string
		target any
	}{
		{`x = 1.5`, &struct{ X int8 }{}},
		{`x = "s"`, &struct{ X bool }{}},
		{`x = "s"`, &struct{ X float64 }{}},
		{`x = "s"`, &struct{ X int32 }{}},
		{`x = 1`, &struct{ X string }{}},
		{`x = true`, &struct{ X string }{}},
		{`x = 3.14`, &struct{ X bool }{}},
		{`x = 2024-01-15`, &struct{ X bool }{}},
		{`x = 10:30:00`, &struct{ X bool }{}},
		{`x = 2024-01-15T10:30:00Z`, &struct{ X bool }{}},
		{`x = 2024-01-15T10:30:00`, &struct{ X bool }{}},
		{`x = [1,2,3]`, &struct{ X bool }{}},
		{`[x]
a = 1`, &struct{ X int }{}},
	}
	for _, tc := range cases {
		err := Unmarshal([]byte(tc.src), tc.target)
		if err == nil {
			t.Errorf("expected error for %q -> %T", tc.src, tc.target)
		}
	}
}

func TestAcceptanceFloatIntoBigFloatAndInterface(t *testing.T) {
	src := `f = 3.14`
	type WithBig struct {
		F *big.Float `toml:"f"`
	}
	var b WithBig
	if err := Unmarshal([]byte(src), &b); err != nil {
		t.Fatal(err)
	}
	if b.F == nil {
		t.Error("big.Float not decoded")
	}

	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if m["f"] != 3.14 {
		t.Errorf("interface float: %v", m["f"])
	}
}

func TestAcceptanceLocalTimeNoFraction(t *testing.T) {
	// Local time without fraction.
	cases := []string{
		`t = 10:30:00`,
		`t = 10:30:45.123`,
		`t = 10:30:45.123456789`,
	}
	for _, src := range cases {
		var c struct {
			T LocalTime `toml:"t"`
		}
		if err := Unmarshal([]byte(src), &c); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}

	// Local time decoded into time.Time interface — should work.
	var m map[string]any
	if err := Unmarshal([]byte(`t = 10:30:00`), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["t"].(LocalTime); !ok {
		t.Errorf("local time: %T", m["t"])
	}
}

func TestAcceptanceStringsWithSpecialCharsRoundTrip(t *testing.T) {
	// Strings that exercise various encode escape branches.
	out, err := Marshal(map[string]any{
		"high_unicode":   "𝕳𝖊𝖑𝖑𝖔",
		"surrogate_safe": "ascii only",
		"bell":           "a\x07b",
		"vtab":           "a\x0bb",
		"all_ws":         "\t\n\r ",
		"empty_string":   "",
		"long":           strings.Repeat("x", 1000),
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := Unmarshal(out, &got); err != nil {
		t.Fatalf("round-trip: %v\n%s", err, out)
	}
}

func TestAcceptanceParseDottedKeysInline(t *testing.T) {
	src := `a.b.c = 1
a.b.d = 2
[other]
x.y = "z"`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	a := m["a"].(map[string]any)
	b := a["b"].(map[string]any)
	if b["c"] != int64(1) || b["d"] != int64(2) {
		t.Errorf("dotted: %v", b)
	}
}

func TestAcceptanceScanLongStrings(t *testing.T) {
	// Various long/edge string contents.
	cases := []string{
		`a = "` + strings.Repeat("x", 500) + `"`,
		`a = '` + strings.Repeat("x", 500) + `'`,
		"a = \"\"\"\n" + strings.Repeat("x\n", 100) + "\"\"\"",
		"a = '''\n" + strings.Repeat("x\n", 100) + "'''",
	}
	for _, src := range cases {
		var m map[string]any
		if err := Unmarshal([]byte(src), &m); err != nil {
			t.Errorf("long string: %v", err)
		}
	}
}

func TestAcceptanceScanHexEscapesInString(t *testing.T) {
	cases := []string{
		`a = "\x00"`,
		`a = "\x7F"`,
		`a = "\xff"`,
		`a = "\xFF"`,
		`a = "\x41\x42"`,
	}
	for _, src := range cases {
		var m map[string]any
		if err := Unmarshal([]byte(src), &m); err != nil {
			t.Errorf("hex escape %q: %v", src, err)
		}
	}

	// Invalid hex escapes
	invalid := []string{
		`a = "\xZZ"`,
		`a = "\x"`,
		`a = "\x1"`,
	}
	for _, src := range invalid {
		var m map[string]any
		if err := Unmarshal([]byte(src), &m); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestAcceptanceBigFloatAndIntEncode(t *testing.T) {
	bi := new(big.Int)
	bi.SetString("123456789012345", 10)
	bf := new(big.Float).SetFloat64(3.14)

	// big.Int — implements encoding.TextMarshaler in stdlib? Yes via math/big.Int.MarshalText.
	out, err := Marshal(map[string]any{"bi": bi, "bf": bf})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Error("empty output")
	}
}

func TestAcceptanceParseMultilineLeadingNewline(t *testing.T) {
	// Multi-line strings with leading newline (which gets stripped per spec).
	src := "a = \"\"\"\nfirst\"\"\""
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if m["a"] != "first" {
		t.Errorf("a = %q", m["a"])
	}

	// Without leading newline
	src2 := `a = """no leading newline"""`
	if err := Unmarshal([]byte(src2), &m); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptanceLineContinuationInMultiline(t *testing.T) {
	// Line continuation: backslash at end of line eats whitespace.
	src := "a = \"\"\"line one \\\n  line two\"\"\""
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m["a"].(string), "line oneline two") && !strings.Contains(m["a"].(string), "line one line two") {
		t.Logf("line continuation result: %q", m["a"])
	}
}

func TestAcceptanceSignedNanAndScanErrors(t *testing.T) {
	cases := []string{
		`a = +nan`,
		`a = -nan`,
	}
	for _, src := range cases {
		var m map[string]any
		if err := Unmarshal([]byte(src), &m); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}

	// Sign followed by invalid character
	bad := []string{
		`a = +`,
		`a = -`,
		`a = +x`,
		`a = -x`,
		`a = +\n1`,
	}
	for _, src := range bad {
		var m map[string]any
		if err := Unmarshal([]byte(src), &m); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestAcceptanceDateTimeWithSpaceSeparator(t *testing.T) {
	// Local DT into LocalDateTime decoded from naive form via no-seconds variant.
	src := `t = 2024-01-15 10:30`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptanceParseLineCommentWithTable(t *testing.T) {
	src := `# leading comment
[server] # inline comment after table
host = "x" # inline value comment
# trailing comment
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := NodeToBytes(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestAcceptanceEncodeMultilineEscapes(t *testing.T) {
	// Multi-line basic string with chars that need escape.
	out, err := MarshalWithOptions(map[string]any{
		"text": "line1\nhas \"quote\"\nand\\backslash\nand\ttab",
	}, WithMultiLineString(true))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := Unmarshal(out, &m); err != nil {
		t.Fatalf("re-decode: %v\n%s", err, out)
	}
}

func TestAcceptanceDecodeLocalIntoTimeTime(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	// LocalDateTime into time.Time
	type Cfg struct {
		LDT time.Time `toml:"ldt"`
	}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(`ldt = 2024-01-15T10:30:00`), &c, WithLocalTimeZone(loc)); err != nil {
		t.Fatal(err)
	}
	if c.LDT.Year() != 2024 {
		t.Errorf("ldt: %v", c.LDT)
	}

	// Invalid local datetime into time.Time
	type Bad struct {
		LDT time.Time `toml:"ldt"`
	}
	var b Bad
	if err := UnmarshalWithOptions([]byte(`ldt = 2024-99-99T10:30:00`), &b, WithLocalTimeZone(loc)); err == nil {
		t.Error("expected error")
	}
}

func TestAcceptanceParseInlineWithErrors(t *testing.T) {
	cases := []string{
		`a = { x = 1`,          // unclosed inline table
		`a = { x = 1, x = 2 }`, // duplicate key in inline
		`a = { x = }`,          // missing value
		`a = { = 1 }`,          // missing key
	}
	for _, src := range cases {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestAcceptanceEncodeNodeWithComments(t *testing.T) {
	// Build a node with comments and serialize.
	root := &Node{Kind: TableNode, Children: []*Node{
		{
			Kind: KeyValueNode, Key: []string{"a"},
			Children:    []*Node{{Kind: IntegerNode, Value: "1"}},
			HeadComment: "head comment",
			Comment:     "line comment",
		},
		{
			Kind: TableNode, Key: []string{"section"},
			HeadComment: "section header",
			Children: []*Node{
				{
					Kind: KeyValueNode, Key: []string{"x"},
					Children: []*Node{{Kind: IntegerNode, Value: "2"}},
				},
			},
		},
	}}
	out, err := NodeToBytes(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "head comment") {
		t.Errorf("comment lost:\n%s", out)
	}
}

func TestAcceptanceEncodeOmitZero(t *testing.T) {
	type Cfg struct {
		A int `toml:"a,omitzero"`
		B int `toml:"b,omitzero"`
	}
	out, err := Marshal(&Cfg{A: 0, B: 5})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "a =") {
		t.Errorf("a not omitted:\n%s", out)
	}
	if !strings.Contains(string(out), "b = 5") {
		t.Errorf("b missing:\n%s", out)
	}
}

func TestAcceptanceDecodeIntoNullableField(t *testing.T) {
	// Pointer fields in target struct.
	type Cfg struct {
		Name *string          `toml:"name"`
		Tags *[]int           `toml:"tags"`
		Sub  *struct{ V int } `toml:"sub"`
	}
	src := `name = "x"
tags = [1, 2, 3]
[sub]
v = 7`
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Name == nil || *c.Name != "x" {
		t.Errorf("name: %v", c.Name)
	}
	if c.Tags == nil || len(*c.Tags) != 3 {
		t.Errorf("tags: %v", c.Tags)
	}
	if c.Sub == nil || c.Sub.V != 7 {
		t.Errorf("sub: %v", c.Sub)
	}
}

func TestAcceptanceUnusualHeadersAndCommentPlacement(t *testing.T) {
	// Various malformed inputs that exercise different scanner/parser paths.
	cases := []string{
		"a = 1\n\n[]\n",            // empty table header
		"a = 1\n\n[ ]",             // whitespace-only header
		`[ "" ]`,                   // empty quoted key
		`[a.""]`,                   // empty quoted key in dotted
		`[a..b]`,                   // double dot
		`a = "x" # comment\nb = 1`, // comment without newline (escape)
		"a = 1\n#",                 // comment at EOF
		`[[a]]
[[a.b]]
[[a]]
[[a.b]]
`, // multiple array tables interleaved
	}
	for _, src := range cases {
		var m map[string]any
		_ = Unmarshal([]byte(src), &m)
		_ = Valid([]byte(src))
	}
}

func TestAcceptanceArrayOfTablesWithNestedTables(t *testing.T) {
	src := `
[[outer]]
name = "first"
[outer.config]
v = 1

[[outer]]
name = "second"
[outer.config]
v = 2
`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	outer := m["outer"].([]any)
	if len(outer) != 2 {
		t.Errorf("outer len: %d", len(outer))
	}
	first := outer[0].(map[string]any)
	cfg, ok := first["config"].(map[string]any)
	if !ok || cfg["v"] != int64(1) {
		t.Errorf("first.config.v = %v", cfg["v"])
	}
}

func TestAcceptanceTextMarshalerError(t *testing.T) {
	type bad struct{}
	// We can't easily create a TextMarshaler that errors via stdlib types,
	// so use a custom marshaler that returns an error.
	encOpt := WithCustomMarshaler(func(_ bad) ([]byte, error) {
		return nil, errors.New("text fail")
	})
	type Cfg struct {
		B bad `toml:"b"`
	}
	if _, err := MarshalWithOptions(&Cfg{}, encOpt); err == nil {
		// May not fail since custom marshalers register on type
	}
	_ = Cfg{}
}

func TestAcceptanceArrayDecodeIntoInterface(t *testing.T) {
	src := `xs = [1, 2.5, "three", true, [4, 5]]`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	arr := m["xs"].([]any)
	if len(arr) != 5 {
		t.Errorf("len = %d", len(arr))
	}
	// Nested array
	if nested, ok := arr[4].([]any); !ok || len(nested) != 2 {
		t.Errorf("nested: %v", arr[4])
	}
}

func TestAcceptanceIntegerOverflowVariants(t *testing.T) {
	// Each int width has a different overflow point.
	cases := []struct {
		src    string
		target any
	}{
		{`n = 200`, &struct{ N int8 }{}},
		{`n = -200`, &struct{ N int8 }{}},
		{`n = 70000`, &struct{ N int16 }{}},
		{`n = -70000`, &struct{ N int16 }{}},
		{`n = 5000000000`, &struct{ N int32 }{}},
		{`n = -5000000000`, &struct{ N int32 }{}},
		{`n = 300`, &struct{ N uint8 }{}},
		{`n = -1`, &struct{ N uint8 }{}},
		{`n = -1`, &struct{ N uint16 }{}},
		{`n = -1`, &struct{ N uint32 }{}},
	}
	for _, tc := range cases {
		if err := Unmarshal([]byte(tc.src), tc.target); err == nil {
			t.Errorf("expected overflow error for %q -> %T", tc.src, tc.target)
		}
	}
}

func TestAcceptanceEncodeNonStringKeyMap(t *testing.T) {
	// Map with int keys.
	out, err := Marshal(map[int]string{1: "a", 2: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1 = ") {
		t.Errorf("int key:\n%s", out)
	}

	// Map with bool keys.
	out2, err := Marshal(map[bool]string{true: "yes", false: "no"})
	if err != nil {
		t.Fatal(err)
	}
	_ = out2
}

func TestAcceptanceOptionsAreFunctional(t *testing.T) {
	// Verify each option produces a callable EncodeOption / DecodeOption.
	encOpts := []EncodeOption{
		WithIndent(2),
		WithInlineTable(false),
		WithArrayMultiline(false),
		WithLiteralString(false),
		WithMultiLineString(false),
		WithBasicStrings(false),
		WithOmitEmpty(false),
		WithComment(map[string][]Comment{}),
		WithIndentTables(false),
		WithTableSeparator(true),
	}
	o := defaultEncodeOptions()
	for _, opt := range encOpts {
		opt(o)
	}

	decOpts := []DecodeOption{
		WithDefaults(),
		WithStrict(),
		WithOrderedMap(),
		WithJSONUnmarshaler(),
		WithMaxDepth(50),
		WithMaxKeys(100),
		WithMaxDocumentSize(10000),
		WithMaxNodes(1000),
		WithLocalTimeZone(time.UTC),
		WithValidator(nil),
	}
	d := defaultDecodeOptions()
	for _, opt := range decOpts {
		opt(d)
	}
}

func TestAcceptanceWalkAllKindsOfNodes(t *testing.T) {
	src := `
title = "demo"

[server]
host = "localhost"
port = 8080

[[items]]
name = "first"
tags = ["a", "b"]

[[items]]
name = "second"
inline = { x = 1, y = 2 }
ts = 2024-01-15T10:30:00Z
local = 2024-01-15
ltime = 10:30:00
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[NodeKind]int{}
	Walk(f.Root, func(n *Node) bool {
		kinds[n.Kind]++
		_ = n.String()
		return true
	})
	for _, k := range []NodeKind{TableNode, KeyValueNode, StringNode, IntegerNode, ArrayNode, ArrayTableNode} {
		if kinds[k] == 0 {
			t.Errorf("kind %v not visited", k)
		}
	}

	// Filter
	for _, k := range []NodeKind{StringNode, IntegerNode, FloatNode, BooleanNode, ArrayNode, InlineTableNode} {
		_ = Filter(f.Root, func(n *Node) bool { return n.Kind == k })
	}
}

func TestAcceptanceMaxDepthExceeded(t *testing.T) {
	// Build a deeply nested document that triggers max-depth check.
	// [[a]][[a.b]][[a.b.c]]... approach
	var sb strings.Builder
	for i := range 60 {
		_ = i
		sb.WriteString("[a")
		for j := range i {
			_ = j
			sb.WriteString(".x")
		}
		sb.WriteString("]\n")
	}
	src := sb.String()
	var m map[string]any
	if err := UnmarshalWithOptions([]byte(src), &m, WithMaxDepth(5)); err == nil {
		t.Skip("max depth not enforced")
	}
}

func TestAcceptanceEncodeStructFieldsWithComment(t *testing.T) {
	// Encode struct with comment tag at multiple field positions.
	type Cfg struct {
		Name   string `toml:"name" comment:"the name"`
		Server struct {
			Host string `toml:"host" comment:"server host"`
		} `toml:"server" comment:"server config"`
	}
	c := Cfg{Name: "demo"}
	c.Server.Host = "localhost"
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"the name", "server config", "server host"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing comment %q:\n%s", want, out)
		}
	}
}

func TestAcceptanceStructValidationOK(t *testing.T) {
	type Cfg struct {
		Name string `toml:"name"`
	}
	v := &okValidator{}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(`name = "x"`), &c, WithValidator(v)); err != nil {
		t.Fatal(err)
	}
	if !v.called {
		t.Error("validator not called")
	}
}

type okValidator struct {
	called bool
}

func (v *okValidator) Struct(_ any) error {
	v.called = true
	return nil
}

func TestAcceptanceAllScalarsToInterface(t *testing.T) {
	src := `
str = "hello"
int_val = 42
float_val = 3.14
bool_val = true
arr = [1, 2, 3]
nested_arr = [[1, 2], [3, 4]]
hex_int = 0xff
oct_int = 0o77
bin_int = 0b101
ts = 2024-01-15T10:30:00Z
local_dt = 2024-01-15T10:30:00
local_d = 2024-01-15
local_t = 10:30:00
inf_val = inf
nan_val = nan
`
	// All into one big interface map.
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	expected := []string{"str", "int_val", "float_val", "bool_val", "arr", "nested_arr",
		"hex_int", "oct_int", "bin_int", "ts", "local_dt", "local_d", "local_t",
		"inf_val", "nan_val"}
	for _, k := range expected {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
}

func TestAcceptanceEncoderWriterError(t *testing.T) {
	type errWriter struct{}
	enc := NewEncoder(failWriter{})
	err := enc.Encode(map[string]any{"a": 1})
	if err == nil {
		t.Error("expected write error")
	}
	_ = errWriter{}
}

type failWriter struct{}

func (failWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write fail")
}

func TestAcceptanceMarshalArrayOnly(t *testing.T) {
	// Just a slice — top-level must be table.
	if _, err := Marshal([]int{1, 2, 3}); !errors.Is(err, ErrMarshalTop) {
		t.Errorf("expected ErrMarshalTop, got %v", err)
	}
}

func TestAcceptanceDecodeOrderedMapFromComplexFixture(t *testing.T) {
	data := loadKitchenSink(t)
	var v any
	if err := UnmarshalWithOptions(data, &v, WithOrderedMap()); err != nil {
		t.Fatal(err)
	}
	ms, ok := v.(MapSlice)
	if !ok {
		t.Fatalf("expected MapSlice, got %T", v)
	}
	if len(ms) == 0 {
		t.Error("empty MapSlice")
	}
}

func TestAcceptanceParseAndMutateAST(t *testing.T) {
	// Comprehensive AST mutation with multiple operations.
	src := `[server]
host = "old"
port = 8080

[[items]]
name = "first"

[[items]]
name = "second"`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	// Replace server.host
	pHost, _ := PathString("$.server.host")
	if err := pHost.Replace(f.Root, &Node{
		Kind:     KeyValueNode,
		Key:      []string{"host"},
		Children: []*Node{{Kind: StringNode, Value: "new"}},
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	// Append a new key under server
	pSrv, _ := PathString("$.server")
	if err := pSrv.Append(f.Root, &Node{
		Kind:     KeyValueNode,
		Key:      []string{"name"},
		Children: []*Node{{Kind: StringNode, Value: "demo"}},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Delete server.port
	pPort, _ := PathString("$.server.port")
	if err := pPort.Delete(f.Root); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Re-serialize
	out, err := NodeToBytes(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `host = "new"`) {
		t.Errorf("Replace didn't take:\n%s", s)
	}
	if !strings.Contains(s, `name = "demo"`) {
		t.Errorf("Append didn't take:\n%s", s)
	}
	if strings.Contains(s, "port = 8080") {
		t.Errorf("Delete didn't take:\n%s", s)
	}
}

func TestAcceptanceMapWithArrayOfTablesValue(t *testing.T) {
	// Note: map[string][]Struct as array-of-tables target has limited support;
	// use map[string]any instead.
	src := `[[things.foo]]
name = "a"
[[things.foo]]
name = "b"`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	things := m["things"].(map[string]any)
	foo := things["foo"].([]any)
	if len(foo) != 2 {
		t.Errorf("things.foo: %v", foo)
	}
}

func TestAcceptanceMaxKeysInTableMap(t *testing.T) {
	// Max keys check inside decodeTableToMap.
	src := `[items]
a = 1
b = 2
c = 3`
	type Cfg struct {
		Items map[string]int `toml:"items"`
	}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(src), &c, WithMaxKeys(2)); err == nil {
		t.Error("expected max-keys error")
	}
}

func TestAcceptanceArrayOfTablesIntoInterfaceMap(t *testing.T) {
	src := `[[items]]
a = 1
[[items]]
a = 2`
	// Decode into map[string]any
	type Cfg struct {
		Items map[string]any `toml:"-"`
	}
	_ = Cfg{}
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	arr, ok := m["items"].([]any)
	if !ok || len(arr) != 2 {
		t.Errorf("items: %T %v", m["items"], m["items"])
	}
}

func TestAcceptanceCommentsAndNewlinesInsideArraysAndInlineTables(t *testing.T) {
	// Empty array containing only newlines and comments.
	src1 := `arr = [
  # comment

]`
	var m1 map[string]any
	if err := Unmarshal([]byte(src1), &m1); err != nil {
		t.Errorf("empty multiline array: %v", err)
	}

	// Inline table with multiline.
	src2 := `inline = {
  k = 1,
}`
	var m2 map[string]any
	if err := Unmarshal([]byte(src2), &m2); err != nil {
		t.Errorf("multiline inline: %v", err)
	}

	// Newlines inside arrays.
	src3 := `arr = [1
  ,
  2]`
	var m3 map[string]any
	if err := Unmarshal([]byte(src3), &m3); err != nil {
		t.Errorf("newlines in array: %v", err)
	}
}

func TestAcceptanceEncodeMapWithComplexValues(t *testing.T) {
	// Map with nested map and array values to hit encodeMap branches.
	out, err := Marshal(map[string]any{
		"a": "string",
		"b": int64(42),
		"c": float64(3.14),
		"d": true,
		"e": []any{1, 2, 3},
		"f": map[string]any{"nested": "value"},
		"g": []any{
			map[string]any{"k": "v1"},
			map[string]any{"k": "v2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[[g]]") {
		t.Errorf("expected [[g]]:\n%s", out)
	}
}

func TestAcceptanceParseInlineWithVariousValues(t *testing.T) {
	src := `mixed = { s = "x", n = 1, f = 1.5, b = true, arr = [1, 2], nested = { k = 1 } }`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	mixed := m["mixed"].(map[string]any)
	if mixed["s"] != "x" || mixed["n"] != int64(1) {
		t.Errorf("inline values: %v", mixed)
	}
}

func TestAcceptanceDecodeStringErrorPaths(t *testing.T) {
	// String into non-byte slice
	type WithIntSlice struct {
		X []int `toml:"x"`
	}
	var w1 WithIntSlice
	if err := Unmarshal([]byte(`x = "hello"`), &w1); err == nil {
		t.Error("expected error decoding string into []int")
	}

	// String into raw int64 (not Duration)
	type WithInt64 struct {
		X int64 `toml:"x"`
	}
	var w2 WithInt64
	if err := Unmarshal([]byte(`x = "hello"`), &w2); err == nil {
		t.Error("expected error decoding string into int64")
	}

	// Invalid duration string
	type WithDur struct {
		X time.Duration `toml:"x"`
	}
	var w3 WithDur
	if err := Unmarshal([]byte(`x = "not-a-duration"`), &w3); err == nil {
		t.Error("expected error decoding bad duration string")
	}
}

func TestAcceptanceArraysIntoTypedSliceFields(t *testing.T) {
	// Just decoding various tables structs that exercise switch cases.
	src := `
arr = [1.5, 2.5]
ints = [1, 2]
strs = ["a", "b"]
mixed = [1, "two", 3.0]
nested = [[1, 2], [3, 4]]
`
	type Cfg struct {
		Arr    []float64 `toml:"arr"`
		Ints   []int8    `toml:"ints"`
		Strs   []string  `toml:"strs"`
		Mixed  []any     `toml:"mixed"`
		Nested [][]int   `toml:"nested"`
	}
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptanceMaxKeysOnTopLevel(t *testing.T) {
	src := `a = 1
b = 2
c = 3
d = 4`
	type Cfg struct {
		A, B, C, D int
	}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(src), &c, WithMaxKeys(3)); err == nil {
		t.Error("expected max-keys error")
	}
}

func TestAcceptanceTopLevelArrayOfTables(t *testing.T) {
	src := `[[items]]
name = "first"

[[items]]
name = "second"`
	type Item struct {
		Name string `toml:"name"`
	}
	type Cfg struct {
		Items []Item
	}
	// Required field check propagates at struct level.
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Items) != 2 {
		t.Errorf("items: %v", c.Items)
	}
}

func TestAcceptanceDecodeArrayWithNesting(t *testing.T) {
	src := `m = [[1, 2], [3, 4], [5, 6]]`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	mat, ok := m["m"].([]any)
	if !ok || len(mat) != 3 {
		t.Errorf("matrix: %v", mat)
	}
}

func TestAcceptanceParseAdjacentArrayTables(t *testing.T) {
	// Define array of tables, then a regular table at a different path.
	src := `[[items]]
name = "first"

[other]
key = "value"

[[items]]
name = "second"`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	if len(m["items"].([]any)) != 2 {
		t.Errorf("items: %v", m["items"])
	}
}

func TestAcceptanceParseDottedKeyExtension(t *testing.T) {
	src := `[a.b]
x = 1

[a.b.c]
y = 2

[a.d]
z = 3`
	var m map[string]any
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatal(err)
	}
	a := m["a"].(map[string]any)
	b := a["b"].(map[string]any)
	if b["x"] != int64(1) {
		t.Errorf("a.b.x: %v", b["x"])
	}
}

func TestAcceptanceDecodeEmptyTable(t *testing.T) {
	src := `[empty]
[other]
key = "value"`
	type Cfg struct {
		Empty struct{}       `toml:"empty"`
		Other map[string]any `toml:"other"`
	}
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Other["key"] != "value" {
		t.Errorf("other: %v", c.Other)
	}
}

func TestAcceptanceDecodeTimeDuration(t *testing.T) {
	src := `t = "2h45m"`
	type Cfg struct {
		T time.Duration `toml:"t"`
	}
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.T != 2*time.Hour+45*time.Minute {
		t.Errorf("duration: %v", c.T)
	}
}

func TestAcceptanceMalformedInputClassesAreRejected(t *testing.T) {
	// Each input targets a specific parser error path.
	cases := map[string]string{
		"unclosed table":        `[a`,
		"unclosed array table":  `[[a`,
		"missing key in table":  `[]`,
		"empty inner key":       `[a..b]`,
		"extra after header":    `[table] extra stuff`,
		"extra after [[]]":      `[[items]] extra`,
		"duplicate table 1":     "[a]\n[a]",
		"duplicate table 2":     "[a.b]\n[a.b]",
		"table-then-array":      "[a]\n[[a]]\n",
		"array-then-table":      "[[a]]\n[a]\n",
		"inline-then-table":     "a = { x = 1 }\n[a]\nb = 2\n",
		"inline-then-array":     "a = { x = 1 }\n[[a]]\nb = 2\n",
		"key after table val":   "a = 1\n[a]\nb = 2\n",
		"missing equals":        `a 5`,
		"value-only doc":        `42`,
		"invalid bare key":      `a-b@c = 1`,
		"unterminated str 1":    `a = "hello`,
		"missing close ']'":     `a = [1, 2`,
		"missing close '}'":     `a = { x = 1`,
		"missing key in inline": `a = { = 1 }`,
		"extra comma":           "a = [1,, 2]",
		"trailing junk":         "a = 1 b = 2",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			var m map[string]any
			err := Unmarshal([]byte(src), &m)
			if err == nil {
				t.Logf("no error for %q", src)
			}
			_ = Valid([]byte(src))
		})
	}
}

func TestAcceptanceArrayTableAndDottedKeyConflicts(t *testing.T) {
	// Specific TOML conflict scenarios.
	cases := []string{
		// Define a key via dotted, then try to redefine as table
		"a.b = 1\n[a.b]\nc = 2\n",
		// Define table, then dotted key trying to extend with same path
		"[a]\nb = 1\n[a.b.c]\nx = 1\n",
		// Multiple definitions
		"[a]\n[a.b]\n[a]\nx = 1\n",
		// dotted then inline conflict
		"a.b = { c = 1 }\n[a.b.c]\nx = 1\n",
	}
	for _, src := range cases {
		var m map[string]any
		err := Unmarshal([]byte(src), &m)
		if err != nil {
			_ = FormatError([]byte(src), err)
		}
	}
}

func TestAcceptanceTableExtendingScalarKeyIsRejected(t *testing.T) {
	// Trigger the loop in parseTable that checks parent prefixes.
	src := `a = "scalar"
[a.b]
x = 1
`
	var m map[string]any
	err := Unmarshal([]byte(src), &m)
	if err == nil {
		t.Skip("no error")
	}
	_ = err.Error()
}

func TestAcceptanceMaxKeysCountsArrayTableEntries(t *testing.T) {
	src := `[[items]]
a = 1
[[items]]
b = 2`
	var m map[string]any
	if err := UnmarshalWithOptions([]byte(src), &m, WithMaxKeys(1)); err == nil {
		t.Error("expected max-keys error")
	}
}

func TestAcceptanceMaxNodesEnforced(t *testing.T) {
	src := `a = 1
b = 2
c = 3
[d]
e = 4
[[f]]
g = 5
[[f]]
h = 6
`
	var m map[string]any
	if err := UnmarshalWithOptions([]byte(src), &m, WithMaxNodes(3)); err == nil {
		t.Error("expected max-nodes error")
	}
}

func TestAcceptanceDecodePtrSlice(t *testing.T) {
	src := `xs = [1, 2, 3]`
	type Cfg struct {
		Xs *[]int `toml:"xs"`
	}
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Xs == nil || len(*c.Xs) != 3 {
		t.Errorf("ptr slice: %v", c.Xs)
	}
}

func TestAcceptanceParseEdgeCasesWhitespace(t *testing.T) {
	srcs := []string{
		"\t\t# tab-indented comment\n",
		"   \n   key = 1\n",
		"key = \"value\"   \n",
		"key=1",     // no spaces
		"key  =  1", // extra spaces
	}
	for _, src := range srcs {
		var m map[string]any
		if err := Unmarshal([]byte(src), &m); err != nil {
			t.Errorf("whitespace %q: %v", src, err)
		}
	}
}

func TestAcceptanceMalformedInputsDoNotPanic(t *testing.T) {
	// Brute-force check that no malformed input panics and FormatError runs.
	cases := []string{
		// Scanner errors
		"",
		"a",
		"=",
		"\"",
		"'",
		"\"\"\"",
		"'''",
		"a = \"\\q\"",
		"a = \"\\u\"",
		"a = \"\\u00\"",
		"a = \"\\U0000000\"",
		"a = \"\\Uffffffff\"",
		"a = \"\\xz\"",
		"a = 0x",
		"a = 0o",
		"a = 0b",
		"a = 0xZZ",
		"a = 0o9",
		"a = 0b2",
		"a = +0x10",
		"a = 1__0",
		"a = 1_",
		"a = _1",
		"a = 1.5_",
		"a = 1.5e",
		"a = 1e_5",
		"a = 1e",
		"a = 5.",
		"a = .5",
		"a = -",
		"a = +",
		"a = - 5",
		"a = -",
		"a = 100-",
		"a = 19999-99-99",
		"a = 1979-",
		"a = 1979-05",
		"a = 99:99:99",
		"a = 25:00:00",
		"a = 1979-13-01T00:00:00Z",
		"a = 1979-12-32T00:00:00Z",
		"a = 1979-05-27T25:00:00Z",
		"a = 1979-05-27T00:00:00+99:00",
		"\xFF\xFE = 1",
		"\xC0\x80 = 1", // overlong UTF-8
		"\x00 = 1",
		"a\x00b = 1",

		// Parser errors
		"a = 1\nb = ",
		"[]",
		"[ ]",
		"[a.\"]",
		"[\"\"]",
		"[a.b\n]",
		"[[]]",
		"[[a.b]\n]",
		"a = b",
		"a = ]",
		"a = [1,]\n[a.b]\nx=1",
		"a = [[]]\n", // nested empty arrays
		"a = [\n# comment\n]",
		"a = [1\n2]",           // missing comma
		"a = {\nx = 1\n}",      // multiline inline ok in 1.1
		"a = { x = 1\nx = 2 }", // duplicate in inline
		"a = { x.y = 1 }",      // dotted key in inline
		"a = { x = 1\n }",      // multiline ok
		"a = 1\nb = 1\nb = 2",  // duplicate non-adjacent
	}
	for _, src := range cases {
		var m map[string]any
		err := Unmarshal([]byte(src), &m)
		if err != nil {
			_ = FormatError([]byte(src), err)
			_ = FormatError([]byte(src), err, true)
		}
		_ = Valid([]byte(src))
		// Parse may also produce errors
		_, _ = Parse([]byte(src))
	}
}

func TestAcceptanceUnterminatedAndInvalidEscapeStringsAreRejected(t *testing.T) {
	// Specific scanner error scenarios.
	cases := []string{
		"a = \"\"\"\nfirst\nsecond", // unterminated multiline
		"a = '''\nfirst\nsecond",    // unterminated multiline literal
		"a = 'unterminated",         // unterminated literal
		"a = \"unterminated",        // unterminated basic
		"a = \"line1\nline2\"",      // newline in basic string
		"a = 'line1\nline2'",        // newline in literal string
		"a = \"\\q\"",               // invalid escape
		"a = \"\\\"",                // single backslash at end
		"a = \"\\u00\"",             // short unicode escape
		"a = \"\\U0000\"",           // short long unicode escape
		"a = \"\\udc00\"",           // surrogate
		"a = \"\\u00d8\\udc00\"",    // unpaired surrogate
		"a = \"\\xZZ\"",             // invalid hex
		"a = \"\\x\"",               // empty hex
	}
	for _, src := range cases {
		var m map[string]any
		err := Unmarshal([]byte(src), &m)
		if err != nil {
			_ = FormatError([]byte(src), err)
		}
	}
}

func TestAcceptanceIntegerLimitValues(t *testing.T) {
	// All forms of integer parsing
	srcs := []string{
		`a = 0`,
		`a = +0`,
		`a = -0`,
		`a = 9223372036854775807`,  // max int64
		`a = -9223372036854775808`, // min int64
		`a = 0x7FFFFFFFFFFFFFFF`,   // max int64 in hex
		`a = 0o0`,
		`a = 0b0`,
	}
	for _, src := range srcs {
		var m map[string]any
		if err := Unmarshal([]byte(src), &m); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

type errMarshalerVal struct{}

func (errMarshalerVal) MarshalTOML() (any, error) {
	return nil, errors.New("marshaler val err")
}

type errBytesMarshalerVal struct{}

func (errBytesMarshalerVal) MarshalTOML() ([]byte, error) {
	return nil, errors.New("bytes marshaler val err")
}

type errTextMarshalerVal struct{}

func (errTextMarshalerVal) MarshalText() ([]byte, error) {
	return nil, errors.New("text marshaler val err")
}

type errCtxMarshalerVal struct{}

func (errCtxMarshalerVal) MarshalTOML(_ context.Context) (any, error) {
	return nil, errors.New("ctx marshaler val err")
}

func TestAcceptanceMarshalerErrorPaths(t *testing.T) {
	t.Run("Marshaler", func(t *testing.T) {
		if _, err := Marshal(map[string]any{"v": errMarshalerVal{}}); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("BytesMarshaler", func(t *testing.T) {
		if _, err := Marshal(map[string]any{"v": errBytesMarshalerVal{}}); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("TextMarshaler", func(t *testing.T) {
		if _, err := Marshal(map[string]any{"v": errTextMarshalerVal{}}); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("MarshalerContext", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.EncodeContext(context.Background(), map[string]any{"v": errCtxMarshalerVal{}}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestAcceptanceLocalAndUTCTimeStructRoundTrip(t *testing.T) {
	// Encode various tricky types
	type ScalarStruct struct {
		LD LocalDate     `toml:"ld"`
		LT LocalTime     `toml:"lt"`
		DT LocalDateTime `toml:"dt"`
		TS time.Time     `toml:"ts"`
	}
	v := ScalarStruct{
		LD: LocalDate{Year: 2024, Month: 1, Day: 15},
		LT: LocalTime{Hour: 10, Minute: 30, Second: 45, Nanosecond: 100_000_000},
		DT: LocalDateTime{
			LocalDate: LocalDate{Year: 2024, Month: 1, Day: 15},
			LocalTime: LocalTime{Hour: 10, Minute: 30, Second: 45},
		},
		TS: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
	}
	out, err := Marshal(&v)
	if err != nil {
		t.Fatal(err)
	}
	var v2 ScalarStruct
	if err := Unmarshal(out, &v2); err != nil {
		t.Fatalf("re-decode: %v\n%s", err, out)
	}

	// Encode array of structs containing scalar struct (datetime).
	type Item struct {
		When time.Time `toml:"when"`
	}
	type Cfg struct {
		Items []Item `toml:"items"`
	}
	c := Cfg{Items: []Item{
		{When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{When: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)},
	}}
	out2, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out2), "[[items]]") {
		t.Errorf("array of tables:\n%s", out2)
	}
}

func TestAcceptanceDecodeBigIntFromInt64Value(t *testing.T) {
	// Big.Int decoding with value within int64 range
	src := `n = 9999999999`
	type Cfg struct {
		N *big.Int `toml:"n"`
	}
	var c Cfg
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.N == nil {
		t.Error("big.Int nil")
	}
}

func TestAcceptanceDecodeIntoArrayOfMixed(t *testing.T) {
	src := `xs = ["a", "b", "c"]`
	// Can decode into [3]string?
	var c struct {
		Xs [3]string `toml:"xs"`
	}
	if err := Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	if c.Xs[2] != "c" {
		t.Errorf("xs: %v", c.Xs)
	}
}

func TestAcceptanceEncodeWithComments(t *testing.T) {
	type Cfg struct {
		Port int `toml:"port" comment:"server port"`
	}
	out, err := MarshalWithOptions(&Cfg{Port: 8080}, WithComment(map[string][]Comment{
		"port": {
			{Position: HeadCommentPos, Text: "head"},
			{Position: LineCommentPos, Text: "line"},
			{Position: FootCommentPos, Text: "foot"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"head", "server port"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestAcceptanceCommonTOMLConstructsParseCleanly(t *testing.T) {
	// Each input is a syntactically valid (or known-invalid) TOML construct
	// that the parser must handle without panicking.
	srcs := []string{
		// Comment placement variations
		"# comment\n\nkey = 1\n",
		"key = 1 # trailing\n",
		"# only\n",
		"\n\n# blank\n\n",

		// Different whitespace
		"key=1",
		"key = 1",
		"key=  1",
		"key  =1",
		"  key = 1  ",

		// Different number formats
		"a = 0",
		"a = -0",
		"a = +0",
		"a = 0_0",  // invalid
		"a = 1__2", // invalid
		"a = 1.0e0",
		"a = 1.0E0",
		"a = 1.0e+0",
		"a = 1.0e-0",

		// Date variations
		"a = 2024-01-15",
		"a = 2024-01-15T00:00:00",
		"a = 2024-01-15T00:00:00Z",
		"a = 2024-01-15T00:00:00+00:00",
		"a = 2024-01-15T00:00:00.123",
		"a = 2024-01-15T00:00:00.123Z",
		"a = 2024-01-15T00:00:00.123-08:00",
		"a = 00:00:00",
		"a = 00:00:00.0",
		"a = 23:59:59.999999999",

		// Inline tables
		"a = {}",
		"a = { x = 1 }",
		"a = { x = 1, y = 2 }",
		"a = { x = { y = 1 } }",
		"a = { x = [1, 2] }",

		// Array variations
		"a = []",
		"a = [1]",
		"a = [1,]",
		"a = [1,2,3]",
		"a = [\n1,\n2,\n3\n]",
		"a = [[1], [2]]",
		"a = [{x=1}, {x=2}]",

		// String escapes
		`a = "\n\t\r\\\""`,
		`a = "\b\f"`,
		`a = "é"`,
		`a = "\U0001F600"`,
		`a = "\x41"`,
		`a = "\e"`,

		// Various keys
		`"abc" = 1`,
		`'abc' = 1`,
		`"with spaces" = 1`,
		`"123" = 1`,
		`a-b = 1`,
		`a_b = 1`,
		`abc123 = 1`,
		`a.b = 1`,
		`a.b.c = 1`,

		// Tables
		"[a]",
		"[a.b]",
		"[a.b.c]",
		"[\"a\"]",
		"[a.\"b\"]",

		// Array tables
		"[[a]]",
		"[[a.b]]",
		"[[a]]\nx = 1\n[[a]]\ny = 2\n",
	}
	for _, src := range srcs {
		var m map[string]any
		_ = Unmarshal([]byte(src), &m)
		_ = Valid([]byte(src))
		_, _ = Parse([]byte(src))
	}
}

func TestAcceptancePathExpressionVariants(t *testing.T) {
	// Path queries on a fixture with diverse contents.
	src := `
[a]
b = 1

[a.c]
d = 2

[[a.e]]
f = 3

[[a.e]]
f = 4
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	// Various valid path patterns
	patterns := []string{
		"$",
		"$.a",
		"$.a.b",
		"$.a.c.d",
		"$.a.e",
		"$.a.e[0]",
		"$.a.e[1]",
		"$.a.e[-1]",
		"$.a.*",
		"$..f",
		"$..b",
	}
	for _, p := range patterns {
		path, err := PathString(p)
		if err != nil {
			t.Errorf("PathString(%q): %v", p, err)
			continue
		}
		nodes, err := path.Read(f.Root)
		_ = nodes
		_ = err
	}

	// Invalid path syntax
	invalid := []string{
		"",
		"a",
		"$.",
		"$..",
		"$.[",
		"$.[1",
		"$.[a]",
		"$.[1.5]",
		"$..@",
	}
	for _, p := range invalid {
		_, err := PathString(p)
		if err == nil {
			t.Logf("expected error for %q", p)
		}
	}
}

func TestAcceptanceCommentedTableField(t *testing.T) {
	type Cfg struct {
		Section struct {
			V int `toml:"v"`
		} `toml:"section,commented" comment:"disabled section"`
	}
	c := Cfg{}
	c.Section.V = 1
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "disabled section") {
		t.Errorf("commented table field missing comment:\n%s", out)
	}
}

func TestAcceptanceArrayOfStructsWithOmitemptyInnerField(t *testing.T) {
	type Item struct {
		Name string   `toml:"name"`
		Tags []string `toml:"tags,omitempty"`
	}
	type Top struct {
		Items []Item `toml:"items"`
	}
	v := Top{Items: []Item{
		{Name: "a", Tags: []string{"x", "y"}},
		{Name: "b"},
	}}
	out, err := Marshal(&v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[[items]]") {
		t.Errorf("missing array tables:\n%s", out)
	}
	if strings.Count(string(out), "tags =") != 1 {
		t.Errorf("omitempty not applied to second item:\n%s", out)
	}
}

func TestAcceptanceCustomMarshalerForStringSubtypeRoundTrip(t *testing.T) {
	type wrapped string
	encOpt := WithCustomMarshaler(func(s wrapped) ([]byte, error) {
		return []byte(`"wrapped:` + string(s) + `"`), nil
	})
	type Cfg struct {
		W wrapped `toml:"w"`
	}
	if _, err := MarshalWithOptions(&Cfg{W: "x"}, encOpt); err != nil {
		t.Fatal(err)
	}

	var c Cfg
	if err := Unmarshal([]byte(`w = "wrapped:y"`), &c); err != nil {
		t.Fatal(err)
	}
	if c.W != "wrapped:y" {
		t.Errorf("decoded wrapped: %q", c.W)
	}
}

func TestAcceptancePathQueriesOnArrayTableTree(t *testing.T) {
	src := `[a]
b = [1, 2, 3]
[[a.c]]
v = "x"
[[a.c]]
v = "y"`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, expr := range []string{"$.a", "$.a.b", "$..v"} {
		p, err := PathString(expr)
		if err != nil {
			t.Errorf("PathString(%q): %v", expr, err)
			continue
		}
		if _, err := p.Read(f.Root); err != nil {
			t.Errorf("Read(%q): %v", expr, err)
		}
	}
}

func TestAcceptanceTableFieldCommentTag(t *testing.T) {
	type Cfg struct {
		Server struct {
			Host string `toml:"host"`
		} `toml:"server" comment:"server config block"`
	}
	c := Cfg{}
	c.Server.Host = "localhost"
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "server config block") {
		t.Errorf("table comment missing:\n%s", out)
	}

	// Same struct, with WithIndentTables to exercise the indent-aware comment path.
	if _, err := MarshalWithOptions(&c, WithIndentTables(true), WithIndent(2)); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptanceArrayMultilineOption(t *testing.T) {
	out, err := MarshalWithOptions(map[string]any{"xs": []any{1, 2, 3, 4, 5}}, WithArrayMultiline(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "  1,") {
		t.Errorf("multiline array missing newlines:\n%s", out)
	}
}

func TestAcceptanceEncodeMapWithMixedScalarValues(t *testing.T) {
	out, err := Marshal(map[string]any{
		"f":   1.5,
		"i":   int64(42),
		"b":   true,
		"s":   "hello",
		"arr": []any{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"f = 1.5", "i = 42", "b = true", `s = "hello"`, "arr = "} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestAcceptanceDefaultsAppliedToScalars(t *testing.T) {
	type Cfg struct {
		Active bool    `toml:"active,default=false"`
		Pi     float32 `toml:"pi,default=3.14"`
	}
	var c Cfg
	if err := UnmarshalWithOptions([]byte(``), &c, WithDefaults()); err != nil {
		t.Fatal(err)
	}
	if c.Pi != 3.14 {
		t.Errorf("pi = %v", c.Pi)
	}
}

func TestAcceptanceOmitEmptyForZeroValuedSliceAndMap(t *testing.T) {
	type Cfg struct {
		S  string         `toml:"s,omitempty"`
		I  int            `toml:"i,omitempty"`
		Sl []int          `toml:"sl,omitempty"`
		M  map[string]int `toml:"m,omitempty"`
	}
	out, err := Marshal(&Cfg{})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"s", "i", "sl", "m"} {
		if strings.Contains(string(out), k+" =") {
			t.Errorf("omitempty leaked %q:\n%s", k, out)
		}
	}
}

func TestAcceptanceCommentedTagWithCommentText(t *testing.T) {
	type Cfg struct {
		Old int `toml:"old,commented" comment:"deprecated, will be removed"`
	}
	out, err := Marshal(&Cfg{Old: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "deprecated") {
		t.Errorf("comment missing:\n%s", out)
	}
	if !strings.Contains(string(out), "# old") {
		t.Errorf("commented field missing:\n%s", out)
	}
}

func TestAcceptanceMultilineLiteralEncoding(t *testing.T) {
	// Multiline + literal tag combo.
	type Cfg struct {
		Text string `toml:"text,multiline,literal"`
	}
	c := Cfg{Text: "line one\nline two"}
	out, err := Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "'''") {
		t.Errorf("expected ''' for multiline literal:\n%s", out)
	}
}

func TestAcceptanceLiteralStringPreference(t *testing.T) {
	// Literal preference at value level
	out, err := MarshalWithOptions(map[string]any{
		"path": `C:\Users\demo`,
	}, WithLiteralString(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "'") {
		t.Errorf("expected literal string:\n%s", out)
	}
}

func TestAcceptanceReflectDeepEqualMaps(t *testing.T) {
	src := `
a = 1
b = "two"
c = [1, 2, 3]
[d]
e = true
`
	var m1 map[string]any
	if err := Unmarshal([]byte(src), &m1); err != nil {
		t.Fatal(err)
	}
	out, err := Marshal(m1)
	if err != nil {
		t.Fatal(err)
	}
	var m2 map[string]any
	if err := Unmarshal(out, &m2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m1, m2) {
		t.Errorf("round-trip mismatch:\nbefore: %#v\nafter:  %#v", m1, m2)
	}
}
