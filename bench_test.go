package toml

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

const benchConfig = `# Service configuration
title = "orders-api"
version = "2.14.3"
debug = false
started = 2024-03-01T09:30:00Z

[server]
host = "0.0.0.0"
port = 8443
read_timeout = "15s"
write_timeout = "30s"
allowed_origins = ["https://app.example.com", "https://admin.example.com", "http://localhost:3000"]

[server.tls]
enabled = true
cert_file = "/etc/ssl/certs/orders.pem"
key_file = "/etc/ssl/private/orders.key"
min_version = "1.2"

[database]
driver = "postgres"
dsn = "postgres://orders:secret@db.internal:5432/orders?sslmode=require"
max_open = 50
max_idle = 10
conn_max_lifetime = "1h"
replicas = ["db-r1.internal", "db-r2.internal", "db-r3.internal"]

[cache]
backend = "redis"
addr = "cache.internal:6379"
ttl_seconds = 300
ratio = 0.75

[logging]
level = "info"
format = "json"
sample_rate = 0.1
fields = { service = "orders", region = "us-east-1", team = "payments" }

[[upstreams]]
name = "inventory"
url = "https://inventory.internal/v2"
timeout_ms = 1500
retries = 3
tags = ["critical", "sync"]

[[upstreams]]
name = "pricing"
url = "https://pricing.internal/v1"
timeout_ms = 800
retries = 2
tags = ["cached"]

[[upstreams]]
name = "shipping"
url = "https://shipping.internal/v3"
timeout_ms = 2500
retries = 5
tags = ["async", "batch"]

[features]
new_checkout = true
loyalty_points = false
gift_cards = true
regional_pricing = true

[limits]
requests_per_minute = 6000
burst = 200
max_body_bytes = 10485760
`

type benchService struct {
	Title   string    `toml:"title"`
	Version string    `toml:"version"`
	Debug   bool      `toml:"debug"`
	Started time.Time `toml:"started"`
	Server  struct {
		Host           string   `toml:"host"`
		Port           int      `toml:"port"`
		ReadTimeout    string   `toml:"read_timeout"`
		WriteTimeout   string   `toml:"write_timeout"`
		AllowedOrigins []string `toml:"allowed_origins"`
		TLS            struct {
			Enabled    bool   `toml:"enabled"`
			CertFile   string `toml:"cert_file"`
			KeyFile    string `toml:"key_file"`
			MinVersion string `toml:"min_version"`
		} `toml:"tls"`
	} `toml:"server"`
	Database struct {
		Driver          string   `toml:"driver"`
		DSN             string   `toml:"dsn"`
		MaxOpen         int      `toml:"max_open"`
		MaxIdle         int      `toml:"max_idle"`
		ConnMaxLifetime string   `toml:"conn_max_lifetime"`
		Replicas        []string `toml:"replicas"`
	} `toml:"database"`
	Cache struct {
		Backend    string  `toml:"backend"`
		Addr       string  `toml:"addr"`
		TTLSeconds int     `toml:"ttl_seconds"`
		Ratio      float64 `toml:"ratio"`
	} `toml:"cache"`
	Logging struct {
		Level      string            `toml:"level"`
		Format     string            `toml:"format"`
		SampleRate float64           `toml:"sample_rate"`
		Fields     map[string]string `toml:"fields"`
	} `toml:"logging"`
	Upstreams []struct {
		Name      string   `toml:"name"`
		URL       string   `toml:"url"`
		TimeoutMS int      `toml:"timeout_ms"`
		Retries   int      `toml:"retries"`
		Tags      []string `toml:"tags"`
	} `toml:"upstreams"`
	Features map[string]bool `toml:"features"`
	Limits   struct {
		RPM          int `toml:"requests_per_minute"`
		Burst        int `toml:"burst"`
		MaxBodyBytes int `toml:"max_body_bytes"`
	} `toml:"limits"`
}

func BenchmarkUnmarshalStruct(b *testing.B) {
	data := []byte(benchConfig)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		var c benchService
		if err := Unmarshal(data, &c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalMap(b *testing.B) {
	data := []byte(benchConfig)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		var m map[string]any
		if err := Unmarshal(data, &m); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	data := []byte(benchConfig)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValid(b *testing.B) {
	data := []byte(benchConfig)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if !Valid(data) {
			b.Fatal("invalid")
		}
	}
}

func BenchmarkMarshalStruct(b *testing.B) {
	var c benchService
	if err := Unmarshal([]byte(benchConfig), &c); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Marshal(c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalMap(b *testing.B) {
	var m map[string]any
	if err := Unmarshal([]byte(benchConfig), &m); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Marshal(m); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNodeToBytes(b *testing.B) {
	f, err := Parse([]byte(benchConfig))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NodeToBytes(f.Root); err != nil {
			b.Fatal(err)
		}
	}
}

// benchScaling returns documents whose parse time grows with n; a linear
// parser keeps time per byte flat as n grows.
func benchScaling(n int) map[string]string {
	var keys, tables, inline strings.Builder
	inline.WriteString("a = {")
	for i := range n {
		fmt.Fprintf(&keys, "k%d = %d\n", i, i)
		fmt.Fprintf(&tables, "[t%d]\nx = %d\n", i, i)
		if i > 0 {
			inline.WriteString(", ")
		}
		fmt.Fprintf(&inline, "k%d = %d", i, i)
	}
	inline.WriteString("}\n")
	dotted := strings.Repeat("a.", n) + "a = 1\n"
	header := "[" + strings.Repeat("a.", n) + "a]\n"
	return map[string]string{
		"keys":   keys.String(),
		"tables": tables.String(),
		"inline": inline.String(),
		"dotted": dotted,
		"header": header,
	}
}

func BenchmarkParseScaling(b *testing.B) {
	for _, n := range []int{1000, 10000} {
		docs := benchScaling(n)
		for _, name := range []string{"keys", "tables", "inline", "dotted", "header"} {
			data := []byte(docs[name])
			b.Run(fmt.Sprintf("%s/%d", name, n), func(b *testing.B) {
				b.SetBytes(int64(len(data)))
				for b.Loop() {
					_, _ = Parse(data)
				}
			})
		}
	}
}
