package sub

import (
	"testing"

	yaml "github.com/goccy/go-yaml"
)

func TestRenderClashTemplate(t *testing.T) {
	tmpl := `mixed-port: 7890
mode: rule
proxies: []
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - PROXIES
  - name: Direct
    type: select
    proxies:
      - DIRECT
      - PROXIES
  - name: Auto
    type: url-test
    proxies: []
  - name: Providers
    type: select
    use:
      - my-provider
rules:
  - DOMAIN-SUFFIX,example.com,DIRECT
  - MATCH,PROXY
`
	proxies := []map[string]any{
		{"name": "N1", "type": "vless", "server": "1.1.1.1", "port": 443},
		{"name": "N2", "type": "trojan", "server": "2.2.2.2", "port": 443},
	}
	names := []string{"N1", "N2"}

	out, err := renderClashTemplate(tmpl, proxies, names)
	if err != nil {
		t.Fatalf("renderClashTemplate: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if cfg["mode"] != "rule" {
		t.Fatalf("template top-level keys lost: mode=%v", cfg["mode"])
	}
	rules, _ := cfg["rules"].([]any)
	if len(rules) != 2 || rules[0] != "DOMAIN-SUFFIX,example.com,DIRECT" {
		t.Fatalf("rules not preserved verbatim: %v", rules)
	}
	ps, _ := cfg["proxies"].([]any)
	if len(ps) != 2 {
		t.Fatalf("merged proxies count = %d, want 2", len(ps))
	}

	groups, _ := cfg["proxy-groups"].([]any)
	if len(groups) != 4 {
		t.Fatalf("proxy-groups count = %d, want 4", len(groups))
	}
	g0 := groups[0].(map[string]any)["proxies"].([]any)
	if len(g0) != 2 || g0[0] != "N1" || g0[1] != "N2" {
		t.Fatalf("PROXY group (marker) = %v, want [N1 N2]", g0)
	}
	g1 := groups[1].(map[string]any)["proxies"].([]any)
	if len(g1) != 3 || g1[0] != "DIRECT" || g1[1] != "N1" || g1[2] != "N2" {
		t.Fatalf("Direct group = %v, want [DIRECT N1 N2]", g1)
	}
	g2 := groups[2].(map[string]any)["proxies"].([]any)
	if len(g2) != 2 || g2[0] != "N1" || g2[1] != "N2" {
		t.Fatalf("Auto group (empty, auto-filled) = %v, want [N1 N2]", g2)
	}
	g3 := groups[3].(map[string]any)
	if _, has := g3["proxies"]; has {
		t.Fatalf("Providers group (use:) should not get proxies, got %v", g3["proxies"])
	}
}

func TestRenderClashTemplateInvalidYAML(t *testing.T) {
	if _, err := renderClashTemplate("\tnot: [valid", nil, nil); err == nil {
		t.Fatal("expected error on invalid YAML")
	}
}
