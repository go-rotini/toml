package toml_test

import (
	"fmt"

	"github.com/go-rotini/toml"
)

// Replacing a value's source span edits it in place: comments, key order and
// layout are left exactly as written.
func ExampleSpan() {
	data := []byte(`# deploy settings
[deploy]
region = "us-east-1" # primary
timeout = 30
`)
	file, err := toml.Parse(data)
	if err != nil {
		panic(err)
	}
	var value toml.Span
	toml.Walk(file.Root, func(n *toml.Node) bool {
		if n.Kind == toml.KeyValueNode && n.Key[0] == "region" {
			value = n.Children[0].Span
		}
		return true
	})

	edited := string(data[:value.Start.Offset]) + `"eu-west-1"` + string(data[value.End.Offset:])
	fmt.Print(edited)
	// Output:
	// # deploy settings
	// [deploy]
	// region = "eu-west-1" # primary
	// timeout = 30
}
