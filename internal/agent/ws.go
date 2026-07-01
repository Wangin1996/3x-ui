package agent

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/agent/wire"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	ws "github.com/gorilla/websocket"
)

func toWSURL(masterURL string) string {
	u := strings.TrimRight(masterURL, "/")
	if after, ok := strings.CutPrefix(u, "https://"); ok {
		return "wss://" + after
	}
	if after, ok := strings.CutPrefix(u, "http://"); ok {
		return "ws://" + after
	}
	return u
}

func (a *Agent) runWS(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := a.dialWS(ctx); err != nil && ctx.Err() == nil {
			logger.Warning("agent ws:", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (a *Agent) dialWS(ctx context.Context) error {
	tlsCfg, err := buildTLSConfig(a.cfg)
	if err != nil {
		return err
	}
	dialer := ws.Dialer{HandshakeTimeout: 15 * time.Second, TLSClientConfig: tlsCfg}
	header := http.Header{}
	header.Set(wire.HeaderNodeGuid, a.cfg.NodeGuid)
	if a.cfg.ApiToken != "" {
		header.Set("Authorization", "Bearer "+a.cfg.ApiToken)
	}
	conn, resp, err := dialer.DialContext(ctx, toWSURL(a.cfg.MasterURL)+"/panel/api/agent/ws", header)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	logger.Info("agent ws: connected")
	for {
		var msg struct {
			Type string `json:"type"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}
		if msg.Type == "config_changed" {
			a.signalConfigChanged()
		}
	}
}
