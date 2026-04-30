package toml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
