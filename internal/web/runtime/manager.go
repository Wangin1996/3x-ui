package runtime

import (
	"sync"
)

// Manager routes inbound operations to the local xray runtime (inbounds with a
// nil NodeID) or to a no-op runtime (node-assigned inbounds, which are served by
// pull-mode agents and never dialed from the master).
type Manager struct {
	local Runtime
	noop  Runtime
}

func NewManager(localDeps LocalDeps) *Manager {
	return &Manager{
		local: NewLocal(localDeps),
		noop:  noopRuntime{},
	}
}

func (m *Manager) RuntimeFor(nodeID *int) (Runtime, error) {
	if nodeID == nil {
		return m.local, nil
	}
	return m.noop, nil
}

var (
	managerMu sync.RWMutex
	manager   *Manager
)

func SetManager(m *Manager) {
	managerMu.Lock()
	defer managerMu.Unlock()
	manager = m
}

func GetManager() *Manager {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return manager
}
