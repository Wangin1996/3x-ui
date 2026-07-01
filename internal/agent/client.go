package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/agent/wire"
)

const maxResponseBytes = 64 << 20

type envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

type Client struct {
	cfg     Config
	http    *http.Client
	baseURL string
}

func buildTLSConfig(cfg Config) (*tls.Config, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	if cfg.TLSCAFile != "" {
		caPEM, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("parse CA: no certificates in %s", cfg.TLSCAFile)
		}
		tlsCfg.RootCAs = pool
	}
	return tlsCfg, nil
}

func NewClient(cfg Config) (*Client, error) {
	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
		baseURL: strings.TrimRight(cfg.MasterURL, "/"),
	}, nil
}

func (c *Client) PullConfig(ctx context.Context, sha string) (*wire.ConfigResponse, error) {
	url := c.baseURL + "/panel/api/agent/config"
	if sha != "" {
		url += "?sha=" + sha
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	var out wire.ConfigResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Report(ctx context.Context, report *wire.Report) (*wire.ReportResponse, error) {
	body, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/panel/api/agent/report", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var out wire.ReportResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) do(req *http.Request, out any) error {
	req.Header.Set(wire.HeaderNodeGuid, c.cfg.NodeGuid)
	if c.cfg.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.ApiToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: status %d: %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if !env.Success {
		return fmt.Errorf("%s %s: %s", req.Method, req.URL.Path, env.Msg)
	}
	if out != nil && len(env.Obj) > 0 {
		if err := json.Unmarshal(env.Obj, out); err != nil {
			return fmt.Errorf("decode obj: %w", err)
		}
	}
	return nil
}
