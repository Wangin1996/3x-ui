package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/agent/wire"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	c, err := NewClient(Config{MasterURL: baseURL, NodeGuid: "guid-1", ApiToken: "tok"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, obj any) {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal obj: %v", err)
	}
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"msg":     "",
		"obj":     json.RawMessage(raw),
	}); err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
}

func TestPullConfig_DecodesEnvelopeAndSendsHeaders(t *testing.T) {
	want := &wire.ConfigResponse{
		Sha256: "abc123",
		Config: &xray.Config{LogConfig: json_util.RawMessage(`{"loglevel":"warning"}`)},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/agent/config" {
			t.Errorf("path = %q, want /panel/api/agent/config", r.URL.Path)
		}
		if got := r.Header.Get(wire.HeaderNodeGuid); got != "guid-1" {
			t.Errorf("%s = %q, want guid-1", wire.HeaderNodeGuid, got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want Bearer tok", got)
		}
		if got := r.URL.Query().Get("sha"); got != "prev" {
			t.Errorf("sha query = %q, want prev", got)
		}
		writeEnvelope(t, w, want)
	}))
	defer srv.Close()

	got, err := newTestClient(t, srv.URL).PullConfig(context.Background(), "prev")
	if err != nil {
		t.Fatalf("PullConfig: %v", err)
	}
	if got.Sha256 != want.Sha256 {
		t.Errorf("Sha256 = %q, want %q", got.Sha256, want.Sha256)
	}
	if got.Config == nil {
		t.Fatalf("Config is nil")
	}
}

func TestPullConfig_Unchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, &wire.ConfigResponse{Unchanged: true, Sha256: "same"})
	}))
	defer srv.Close()

	got, err := newTestClient(t, srv.URL).PullConfig(context.Background(), "same")
	if err != nil {
		t.Fatalf("PullConfig: %v", err)
	}
	if !got.Unchanged {
		t.Errorf("Unchanged = false, want true")
	}
	if got.Config != nil {
		t.Errorf("Config = %v, want nil on unchanged", got.Config)
	}
}

func TestPullConfig_EnvelopeFailureSurfacesMsg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "msg": "render agent config (boom)"})
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv.URL).PullConfig(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error on success=false envelope")
	}
}

func TestPullConfig_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv.URL).PullConfig(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error on 404")
	}
}
