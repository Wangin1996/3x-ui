package agent

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	MasterURL          string
	NodeGuid           string
	ApiToken           string
	TLSCertFile        string
	TLSKeyFile         string
	TLSCAFile          string
	InsecureSkipVerify bool
	PollInterval       time.Duration
	ReportInterval     time.Duration
}

func ConfigFromEnv() Config {
	return Config{
		MasterURL:          os.Getenv("XUI_NODE_MASTER_URL"),
		NodeGuid:           os.Getenv("XUI_NODE_GUID"),
		ApiToken:           os.Getenv("XUI_NODE_API_TOKEN"),
		TLSCertFile:        os.Getenv("XUI_NODE_TLS_CERT"),
		TLSKeyFile:         os.Getenv("XUI_NODE_TLS_KEY"),
		TLSCAFile:          os.Getenv("XUI_NODE_TLS_CA"),
		InsecureSkipVerify: os.Getenv("XUI_NODE_TLS_SKIP_VERIFY") == "true",
		PollInterval:       envDuration("XUI_NODE_POLL_INTERVAL", 30*time.Second),
		ReportInterval:     envDuration("XUI_NODE_REPORT_INTERVAL", 10*time.Second),
	}
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d
	}
	return def
}

func (c Config) Validate() error {
	if c.MasterURL == "" {
		return errors.New("master URL is required (XUI_NODE_MASTER_URL or -master)")
	}
	if c.NodeGuid == "" {
		return errors.New("node guid is required (XUI_NODE_GUID or -guid)")
	}
	return nil
}
