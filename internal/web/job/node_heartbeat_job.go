package job

import (
	"strconv"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
)

const (
	nodeHeartbeatConcurrency    = 32
	nodeHeartbeatRequestTimeout = 4 * time.Second
	agentStaleSeconds           = 90
)

type NodeHeartbeatJob struct {
	nodeService service.NodeService
	running     sync.Mutex
}

func NewNodeHeartbeatJob() *NodeHeartbeatJob {
	return &NodeHeartbeatJob{}
}

func (j *NodeHeartbeatJob) Run() {
	if !j.running.TryLock() {
		return
	}
	defer j.running.Unlock()

	nodes, err := j.nodeService.GetAll()
	if err != nil {
		logger.Warning("node heartbeat: load nodes failed:", err)
		return
	}
	if len(nodes) == 0 {
		return
	}

	for _, n := range nodes {
		if n.Enable {
			j.sweepAgent(n)
		}
	}

	if !websocket.HasClients() {
		return
	}
	updated, err := j.nodeService.GetAll()
	if err != nil {
		logger.Warning("node heartbeat: load nodes for broadcast failed:", err)
		return
	}
	websocket.BroadcastNodes(updated)
}

func (j *NodeHeartbeatJob) sweepAgent(n *model.Node) {
	if n.Status != "online" {
		return
	}
	if websocket.AgentConnected(n.Id) {
		return
	}
	if time.Now().Unix()-n.LastHeartbeat <= agentStaleSeconds {
		return
	}
	patch := service.HeartbeatPatch{Status: "offline", LastHeartbeat: n.LastHeartbeat, LastError: "agent stopped reporting"}
	if err := j.nodeService.UpdateHeartbeat(n.Id, patch); err != nil {
		logger.Warning("node heartbeat: mark agent", n.Id, "offline failed:", err)
		return
	}
	publishNodeTransition(n, n.Status, patch)
}

// publishNodeTransition emits node.down / node.up only on a genuine state change.
// An "unknown"/empty previous status (fresh start) is treated as not-online, so a
// node coming up for the first time fires node.up but never a spurious node.down.
func publishNodeTransition(n *model.Node, prevStatus string, patch service.HeartbeatPatch) {
	if EventBus == nil {
		return
	}
	var eventType eventbus.EventType
	switch {
	case prevStatus == "online" && patch.Status == "offline":
		eventType = eventbus.EventNodeDown
	case prevStatus != "online" && patch.Status == "online":
		eventType = eventbus.EventNodeUp
	default:
		return
	}
	source := n.Name
	if source == "" {
		source = "node-" + strconv.Itoa(n.Id)
	}
	EventBus.Publish(eventbus.Event{
		Type:   eventType,
		Source: source,
		Data: &eventbus.NodeHealthData{
			NodeId:    n.Id,
			LatencyMs: patch.LatencyMs,
			CpuPct:    patch.CpuPct,
			MemPct:    patch.MemPct,
			XrayState: patch.XrayState,
			XrayError: patch.XrayError,
		},
	})
}
