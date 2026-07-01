package job

import (
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// nodeInboundSpeedWindowMs is the poll window node-inbound speed deltas are
// normalized to; it MUST match the dashboard's TRAFFIC_POLL_INTERVAL_S (5s),
// the fixed divisor the frontend applies to turn a delta into a rate.
const nodeInboundSpeedWindowMs int64 = 5000

// inboundSample is a node inbound's last-seen cumulative up/down and the time
// (unix millis) its counter last changed, used to derive a normalized speed.
type inboundSample struct {
	up, down, at int64
}

type NodeTrafficSyncJob struct {
	inboundService service.InboundService
	settingService service.SettingService
	xrayService    service.XrayService
	running        sync.Mutex
	structural     atomicBool
	// prevInboundTotals holds the previous poll's cumulative up/down (and the time
	// the counter last changed) per node inbound tag, so the next poll can derive
	// a per-inbound speed delta — node inbounds have no local Xray poll. Touched
	// only from Run (serialized).
	prevInboundTotals map[string]inboundSample
}

type atomicBool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomicBool) set() {
	a.mu.Lock()
	a.v = true
	a.mu.Unlock()
}

func (a *atomicBool) takeAndReset() bool {
	a.mu.Lock()
	v := a.v
	a.v = false
	a.mu.Unlock()
	return v
}

func NewNodeTrafficSyncJob() *NodeTrafficSyncJob {
	return &NodeTrafficSyncJob{}
}

func (j *NodeTrafficSyncJob) Run() {
	if !j.running.TryLock() {
		return
	}
	defer j.running.Unlock()

	_, clientsDisabled, err := j.inboundService.AddTraffic(nil, nil)
	if err != nil {
		logger.Warning("node traffic sync: depletion check failed:", err)
	}
	if clientsDisabled {
		if restartOnDisable, settingErr := j.settingService.GetRestartXrayOnClientDisable(); settingErr == nil && restartOnDisable {
			if err := j.xrayService.RestartXray(true); err != nil {
				logger.Warning("node traffic sync: restart xray after disabling clients failed:", err)
				j.xrayService.SetToNeedRestart()
			}
		} else if settingErr != nil {
			logger.Warning("node traffic sync: get RestartXrayOnClientDisable failed:", settingErr)
		}
		j.structural.set()
	}

	lastOnline, err := j.inboundService.GetClientsLastOnline()
	if err != nil {
		logger.Warning("node traffic sync: get last-online failed:", err)
	}
	if lastOnline == nil {
		lastOnline = map[string]int64{}
	}

	// Prune stale local-online entries (no local active emails or inbound tags
	// to add here — only the local xray poll feeds those) so a stopped local
	// xray's clients and inbounds still age out between traffic polls.
	j.inboundService.RefreshLocalOnlineClients(nil, nil)

	// Derive per-node-inbound speed every tick (keeps the baseline fresh even
	// with no dashboard open); only broadcast it when someone is watching.
	inboundSpeed := j.nodeInboundSpeed()

	if !websocket.HasClients() {
		return
	}

	online := j.inboundService.GetOnlineClients()
	if online == nil {
		online = []string{}
	}
	trafficPayload := map[string]any{
		"onlineClients":  online,
		"onlineByGuid":   j.inboundService.GetOnlineClientsByGuid(),
		"activeInbounds": j.inboundService.GetActiveInboundsByGuid(),
		"lastOnlineMap":  lastOnline,
	}
	// Always send the key so the dashboard clears node inbounds that went idle
	// this tick. A nil result (query error) marshals to null and is skipped
	// client-side, leaving the last shown value untouched; an empty (non-nil)
	// slice marshals to [] and clears stale speeds.
	trafficPayload["nodeTraffics"] = inboundSpeed
	websocket.BroadcastTraffic(trafficPayload)

	clientStats := map[string]any{}
	if stats, err := j.inboundService.GetAllClientTraffics(); err != nil {
		logger.Warning("node traffic sync: get all client traffics for websocket failed:", err)
	} else if len(stats) > 0 {
		clientStats["clients"] = stats
	}
	if summary, err := j.inboundService.GetInboundsTrafficSummary(); err != nil {
		logger.Warning("node traffic sync: get inbounds summary for websocket failed:", err)
	} else if len(summary) > 0 {
		clientStats["inbounds"] = summary
	}
	if len(clientStats) > 0 {
		websocket.BroadcastClientStats(clientStats)
	}

	if j.structural.takeAndReset() {
		websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
		websocket.BroadcastInvalidate(websocket.MessageTypeClients)
	}
}

// nodeInboundSpeed derives a per-node-inbound speed delta by diffing the current
// cumulative up/down against the previous poll's, keyed by the central tag the
// dashboard matches. The node's counter keeps climbing while the master can't
// reach it, so the first delta after a gap (node outage, skipped poll, slow
// node) spans more than one poll window; it is normalized to the fixed
// nodeInboundSpeedWindowMs using the real elapsed time so the dashboard's fixed
// divisor yields the true average rate over the gap instead of an impossible
// one-tick spike. The change timestamp only advances when the value actually
// moves, so an idle stretch is averaged correctly when traffic resumes. A reset
// rebaselines to the lower value; a first-seen tag yields no delta until the
// next poll.
func (j *NodeTrafficSyncJob) nodeInboundSpeed() []*xray.Traffic {
	totals, err := j.inboundService.GetNodeInboundTrafficTotals()
	if err != nil {
		return nil
	}
	now := time.Now().UnixMilli()
	deltas := make([]*xray.Traffic, 0, len(totals))
	next := make(map[string]inboundSample, len(totals))
	for tag, cur := range totals {
		prev, ok := j.prevInboundTotals[tag]
		if !ok {
			next[tag] = inboundSample{up: cur[0], down: cur[1], at: now}
			continue
		}
		dUp := cur[0] - prev.up
		dDown := cur[1] - prev.down
		if dUp <= 0 && dDown <= 0 {
			// No movement, or a counter reset: hold the change timestamp so a
			// later jump is averaged over the real elapsed window, not shown as a
			// spike. Adopt the lower value on a reset.
			if cur[0] < prev.up || cur[1] < prev.down {
				next[tag] = inboundSample{up: cur[0], down: cur[1], at: now}
			} else {
				next[tag] = prev
			}
			continue
		}
		if dUp < 0 {
			dUp = 0
		}
		if dDown < 0 {
			dDown = 0
		}
		elapsed := max(now-prev.at, nodeInboundSpeedWindowMs)
		up := dUp * nodeInboundSpeedWindowMs / elapsed
		down := dDown * nodeInboundSpeedWindowMs / elapsed
		if up > 0 || down > 0 {
			deltas = append(deltas, &xray.Traffic{Tag: tag, IsInbound: true, Up: up, Down: down})
		}
		next[tag] = inboundSample{up: cur[0], down: cur[1], at: now}
	}
	j.prevInboundTotals = next
	return deltas
}
