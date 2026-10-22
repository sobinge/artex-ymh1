package server

import (
	"testing"

	"github.com/Autumn-27/artex/db"
)

func TestRowToFofaRow(t *testing.T) {
	row := []string{"https://example.com:8443", "203.0.113.10", "8443", "https", "example.com", "Example", "nginx", "ICP"}
	got := rowToFofaRow(row)
	if got.Host != "https://example.com:8443" || got.IP != "203.0.113.10" || got.Port != "8443" ||
		got.Protocol != "https" || got.Domain != "example.com" || got.Title != "Example" || got.Server != "nginx" {
		t.Fatalf("rowToFofaRow mismatch: %+v", got)
	}
}

func TestSplitFofaHost(t *testing.T) {
	cases := []struct {
		in                     string
		scheme, hostname, port string
	}{
		{"https://example.com", "https", "example.com", ""},
		{"http://example.com:8080/path", "http", "example.com", "8080"},
		{"example.com:443", "", "example.com", "443"},
		{"1.2.3.4:8443", "", "1.2.3.4", "8443"},
		{"example.com", "", "example.com", ""},
	}
	for _, c := range cases {
		s, h, p := splitFofaHost(c.in)
		if s != c.scheme || h != c.hostname || p != c.port {
			t.Fatalf("splitFofaHost(%q) = (%q,%q,%q), want (%q,%q,%q)", c.in, s, h, p, c.scheme, c.hostname, c.port)
		}
	}
}

func TestBuildFofaURL(t *testing.T) {
	cases := []struct {
		scheme, protocol, host, port, want string
	}{
		{"https", "https", "example.com", "443", "https://example.com"},
		{"http", "http", "example.com", "80", "http://example.com"},
		{"", "https", "example.com", "8443", "https://example.com:8443"},
		{"", "ftp", "example.com", "21", "http://example.com:21"},
	}
	for _, c := range cases {
		if got := buildFofaURL(c.scheme, c.protocol, c.host, c.port); got != c.want {
			t.Fatalf("buildFofaURL = %q, want %q", got, c.want)
		}
	}
}

func TestFofaIsMasked(t *testing.T) {
	if !fofaIsMasked("__masked__:…abcdef") {
		t.Fatal("masked value not detected")
	}
	if fofaIsMasked("real-api-key") {
		t.Fatal("real value wrongly treated as masked")
	}
}

// TestFofaIngestUpsertsAssets 在真实 PG 上验证一行 FOFA 结果会被映射为
// IP / 根域 / 子域 / HTTP 服务四类资产。
func TestFofaIngestUpsertsAssets(t *testing.T) {
	dsn, _, err := db.DSN()
	if err != nil {
		t.Skipf("no database config (%v)", err)
	}
	d, err := db.Open(dsn)
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer d.Close()
	defer d.Exec(`DELETE FROM assets WHERE domain IN ('fofa-test.example','www.fofa-test.example') OR ip='203.0.113.55'`)

	synced := map[string]int{"service": 0, "ip": 0, "subdomain": 0, "domain": 0}
	root, errStr := fofaIngest(d.Assets(), []string{
		"https://www.fofa-test.example:8443", "203.0.113.55", "8443", "https",
		"fofa-test.example", "FOFA Test", "nginx", "",
	}, synced, nil)
	if errStr != "" {
		t.Fatalf("fofaIngest error: %s", errStr)
	}
	if root != "fofa-test.example" {
		t.Fatalf("root = %q, want fofa-test.example", root)
	}
	if synced["ip"] == 0 || synced["domain"] == 0 || synced["subdomain"] == 0 || synced["service"] == 0 {
		t.Fatalf("unexpected synced counts: %#v", synced)
	}
	var cnt int
	if err := d.QueryRow(`SELECT count(*) FROM assets WHERE domain='www.fofa-test.example' AND type='subdomain'`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("subdomain asset count = %d, want 1", cnt)
	}
	if err := d.QueryRow(`SELECT count(*) FROM assets WHERE url LIKE 'https://www.fofa-test.example:8443%'`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("http service asset count = %d, want 1", cnt)
	}
}
