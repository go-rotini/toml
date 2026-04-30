package toml_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rotini/toml"
)

func ExampleMarshal() {
	type Server struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type Config struct {
		Title  string `toml:"title"`
		Server Server `toml:"server"`
	}
	out, err := toml.Marshal(Config{
		Title:  "My App",
		Server: Server{Host: "localhost", Port: 8080},
	})
	if err != nil {
		panic(err)
	}
	fmt.Print(string(out))
	// Output:
	// title = "My App"
	//
	// [server]
	// host = "localhost"
	// port = 8080
}

func ExampleMarshalWithOptions() {
	type Config struct {
		Name  string `toml:"name"`
		Debug bool   `toml:"debug"`
		Port  int    `toml:"port"`
	}
	out, err := toml.MarshalWithOptions(
		Config{Name: "app", Port: 3000},
		toml.WithOmitEmpty(true),
	)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(out))
	// Output:
	// name = "app"
	// port = 3000
}

func ExampleUnmarshal() {
	input := []byte(`
title = "TOML Example"

[server]
host = "localhost"
port = 8080
`)
	type Server struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type Config struct {
		Title  string `toml:"title"`
		Server Server `toml:"server"`
	}

	var cfg Config
	if err := toml.Unmarshal(input, &cfg); err != nil {
		panic(err)
	}
	fmt.Printf("title=%s host=%s port=%d\n", cfg.Title, cfg.Server.Host, cfg.Server.Port)
	// Output:
	// title=TOML Example host=localhost port=8080
}

func ExampleUnmarshalTo() {
	type Config struct {
		Name    string `toml:"name"`
		Version int    `toml:"version"`
	}
	cfg, err := toml.UnmarshalTo[Config]([]byte(`
name = "myapp"
version = 2
`))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s v%d\n", cfg.Name, cfg.Version)
	// Output:
	// myapp v2
}

func ExampleUnmarshalWithOptions_strict() {
	type Config struct {
		Name string `toml:"name"`
	}
	err := toml.UnmarshalWithOptions(
		[]byte("name = \"test\"\nunknown = true\n"),
		&Config{},
		toml.WithStrict(),
	)
	fmt.Println(err)
	// Output:
	// toml: line 2: unknown field "unknown"
}

func ExampleUnmarshalWithOptions_defaults() {
	type Config struct {
		Host string `toml:"host,default=localhost"`
		Port int    `toml:"port,default=8080"`
	}
	var cfg Config
	if err := toml.UnmarshalWithOptions([]byte(""), &cfg, toml.WithDefaults()); err != nil {
		panic(err)
	}
	fmt.Printf("host=%s port=%d\n", cfg.Host, cfg.Port)
	// Output:
	// host=localhost port=8080
}

func ExampleDecodeFile() {
	// Create a temporary TOML file for demonstration.
	dir, err := os.MkdirTemp("", "toml-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("name = \"from-file\"\n"), 0644); err != nil {
		panic(err)
	}

	type Config struct {
		Name string `toml:"name"`
	}
	var cfg Config
	if err := toml.DecodeFile(path, &cfg); err != nil {
		panic(err)
	}
	fmt.Println(cfg.Name)
	// Output:
	// from-file
}

func ExampleNewDecoder() {
	input := strings.NewReader(`name = "decoded"
value = 42
`)
	type Config struct {
		Name  string `toml:"name"`
		Value int    `toml:"value"`
	}
	var cfg Config
	dec := toml.NewDecoder(input)
	if err := dec.Decode(&cfg); err != nil {
		panic(err)
	}
	fmt.Printf("%s: %d\n", cfg.Name, cfg.Value)
	// Output:
	// decoded: 42
}

func ExampleNewEncoder() {
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(Config{Name: "app", Port: 9090}); err != nil {
		panic(err)
	}
	fmt.Print(buf.String())
	// Output:
	// name = "app"
	// port = 9090
}

func ExampleParse() {
	file, err := toml.Parse([]byte(`
title = "Example"

[database]
port = 5432
`))
	if err != nil {
		panic(err)
	}
	fmt.Printf("root kind: TableNode=%v\n", file.Root.Kind == toml.TableNode)
	fmt.Printf("children: %d\n", len(file.Root.Children))
	// Output:
	// root kind: TableNode=true
	// children: 2
}

func ExampleWalk() {
	file, err := toml.Parse([]byte(`
name = "test"
version = 1

[server]
host = "0.0.0.0"
`))
	if err != nil {
		panic(err)
	}
	toml.Walk(file.Root, func(n *toml.Node) bool {
		if n.Kind == toml.KeyValueNode {
			fmt.Println(strings.Join(n.Key, "."))
		}
		return true
	})
	// Output:
	// name
	// version
	// host
}

func ExampleFilter() {
	file, err := toml.Parse([]byte(`
name = "test"
count = 42
active = true
`))
	if err != nil {
		panic(err)
	}
	strings := toml.Filter(file.Root, func(n *toml.Node) bool {
		return n.Kind == toml.StringNode
	})
	for _, n := range strings {
		fmt.Println(n.Value)
	}
	// Output:
	// test
}

func ExampleNodeToBytes() {
	file, err := toml.Parse([]byte(`name = "roundtrip"
value = 100
`))
	if err != nil {
		panic(err)
	}
	out, err := toml.NodeToBytes(file.Root)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(out))
	// Output:
	// name = "roundtrip"
	// value = 100
}

func ExamplePathString() {
	file, err := toml.Parse([]byte(`
[server]
host = "example.com"
port = 443
`))
	if err != nil {
		panic(err)
	}
	path, err := toml.PathString("$.server.host")
	if err != nil {
		panic(err)
	}
	val, err := path.ReadString(file.Root)
	if err != nil {
		panic(err)
	}
	fmt.Println(val)
	// Output:
	// example.com
}

func ExamplePath_Read() {
	file, err := toml.Parse([]byte(`
[server]
host = "example.com"
port = 443
`))
	if err != nil {
		panic(err)
	}
	path, err := toml.PathString("$.server")
	if err != nil {
		panic(err)
	}
	nodes, err := path.Read(file.Root)
	if err != nil {
		panic(err)
	}
	fmt.Printf("found %d node(s), kind=TableNode: %v\n", len(nodes), nodes[0].Kind == toml.TableNode)
	// Output:
	// found 1 node(s), kind=TableNode: true
}

func ExamplePath_ReadString() {
	file, err := toml.Parse([]byte(`title = "Hello"
`))
	if err != nil {
		panic(err)
	}
	path, err := toml.PathString("$.title")
	if err != nil {
		panic(err)
	}
	val, err := path.ReadString(file.Root)
	if err != nil {
		panic(err)
	}
	fmt.Println(val)
	// Output:
	// Hello
}

func ExamplePath_Delete() {
	file, err := toml.Parse([]byte(`name = "keep"
remove_me = true
`))
	if err != nil {
		panic(err)
	}
	path, err := toml.PathString("$.remove_me")
	if err != nil {
		panic(err)
	}
	if err := path.Delete(file.Root); err != nil {
		panic(err)
	}
	out, err := toml.NodeToBytes(file.Root)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(out))
	// Output:
	// name = "keep"
}

func ExampleToJSON() {
	jsonOut, err := toml.ToJSON([]byte(`name = "alice"
age = 30
`))
	if err != nil {
		panic(err)
	}
	fmt.Println(string(jsonOut))
	// Output:
	// {"age":30,"name":"alice"}
}

func ExampleFromJSON() {
	tomlOut, err := toml.FromJSON([]byte(`{"name":"bob","score":99}`))
	if err != nil {
		panic(err)
	}
	fmt.Print(string(tomlOut))
	// Output:
	// name = "bob"
	// score = 99
}

func ExampleFormatError() {
	input := []byte("key =\nother = 1")
	var m map[string]any
	err := toml.Unmarshal(input, &m)
	if err != nil {
		formatted := toml.FormatError(input, err)
		fmt.Print(formatted)
	}
	// Output:
	// toml: line 1, column 6: unexpected token NEWLINE in value
	//   key =
	//        ^
}

func ExampleValid() {
	fmt.Println(toml.Valid([]byte(`key = "value"`)))
	fmt.Println(toml.Valid([]byte(`key = `)))
	// Output:
	// true
	// false
}

func ExampleLocalDate() {
	d := toml.LocalDate{Year: 2024, Month: time.March, Day: 15}
	fmt.Println(d.String())

	t := d.AsTime(time.UTC)
	fmt.Println(t.Format("2006-01-02"))
	// Output:
	// 2024-03-15
	// 2024-03-15
}

func ExampleLocalTime() {
	t := toml.LocalTime{Hour: 14, Minute: 30, Second: 0}
	fmt.Println(t.String())
	// Output:
	// 14:30:00
}

func ExampleLocalDateTime() {
	dt := toml.LocalDateTime{
		LocalDate: toml.LocalDate{Year: 2024, Month: time.March, Day: 15},
		LocalTime: toml.LocalTime{Hour: 14, Minute: 30, Second: 0},
	}
	fmt.Println(dt.String())

	t := dt.AsTime(time.UTC)
	fmt.Println(t.Format(time.RFC3339))
	// Output:
	// 2024-03-15T14:30:00
	// 2024-03-15T14:30:00Z
}

func ExampleMapSlice() {
	ms := toml.MapSlice{
		{Key: "first", Value: 1},
		{Key: "second", Value: 2},
		{Key: "third", Value: 3},
	}
	for _, item := range ms {
		fmt.Printf("%s=%v\n", item.Key, item.Value)
	}
	// Output:
	// first=1
	// second=2
	// third=3
}

func ExampleRawValue() {
	raw := toml.RawValue([]byte(`"hello world"`))
	var s struct {
		Raw string `toml:"_raw_"`
	}
	if err := raw.Unmarshal(&s); err != nil {
		panic(err)
	}
	fmt.Println(s.Raw)
	// Output:
	// hello world
}

// Ensure unused imports are referenced.
var _ = errors.New
var _ = time.Now
