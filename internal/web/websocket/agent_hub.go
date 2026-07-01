package websocket

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	ws "github.com/gorilla/websocket"
)

// AgentConn wraps a pull-mode node agent's WebSocket connection with a write
// mutex, since the hub may push from one goroutine while the controller's read
// loop runs in another (gorilla allows one concurrent reader and one writer).
type AgentConn struct {
	conn *ws.Conn
	mu   sync.Mutex
}

func (a *AgentConn) writeJSON(v any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.conn.WriteJSON(v)
}

type agentHub struct {
	mu    sync.RWMutex
	conns map[int]*AgentConn
}

var agents = &agentHub{conns: make(map[int]*AgentConn)}

// RegisterAgent records an agent's connection, replacing and closing any prior
// connection for the same node. The returned handle is passed to UnregisterAgent.
func RegisterAgent(nodeID int, conn *ws.Conn) *AgentConn {
	ac := &AgentConn{conn: conn}
	agents.mu.Lock()
	if old := agents.conns[nodeID]; old != nil {
		_ = old.conn.Close()
	}
	agents.conns[nodeID] = ac
	agents.mu.Unlock()
	return ac
}

// UnregisterAgent drops the node's connection, but only if it is still the one
// registered (a newer reconnect must not be evicted by an older one's teardown).
func UnregisterAgent(nodeID int, ac *AgentConn) {
	agents.mu.Lock()
	if agents.conns[nodeID] == ac {
		delete(agents.conns, nodeID)
	}
	agents.mu.Unlock()
}

// NotifyAgentConfigChanged pushes a config-changed event to a connected agent so
// it re-pulls immediately. A no-op (returns false) when the agent is not
// connected over WebSocket — the agent's report-driven dirty flag is the fallback.
func NotifyAgentConfigChanged(nodeID int) bool {
	agents.mu.RLock()
	ac := agents.conns[nodeID]
	agents.mu.RUnlock()
	if ac == nil {
		return false
	}
	if err := ac.writeJSON(map[string]string{"type": "config_changed"}); err != nil {
		logger.Warning("agent hub: notify node", nodeID, "failed:", err)
		return false
	}
	return true
}

// AgentConnected reports whether a node currently holds a live agent WebSocket.
func AgentConnected(nodeID int) bool {
	agents.mu.RLock()
	defer agents.mu.RUnlock()
	return agents.conns[nodeID] != nil
}
