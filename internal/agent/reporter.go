package agent

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/agent/wire"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const onlineGraceMs int64 = 20000

type reporter struct {
	applier *Applier
	api     *xray.XrayAPI
	apiPort int
	cum     map[string]*xray.ClientTraffic
}

func newReporter(applier *Applier) *reporter {
	return &reporter{applier: applier, cum: make(map[string]*xray.ClientTraffic)}
}

func (r *reporter) close() {
	if r.api != nil {
		r.api.Close()
		r.api = nil
	}
}

func (r *reporter) build() (*wire.Report, bool) {
	proc := r.applier.Process()
	if proc == nil || !proc.IsRunning() {
		r.close()
		return nil, false
	}
	port := proc.GetAPIPort()
	if port <= 0 {
		return nil, false
	}
	if r.api == nil || r.apiPort != port {
		r.close()
		api := &xray.XrayAPI{}
		if err := api.Init(port); err != nil {
			return nil, false
		}
		r.api = api
		r.apiPort = port
	}

	traffics, clientTraffics, err := r.api.GetTraffic()
	if err != nil {
		r.close()
		return nil, false
	}

	nowMs := time.Now().UnixMilli()
	var activeEmails []string
	for _, ct := range clientTraffics {
		if ct.Email == "" {
			continue
		}
		acc, ok := r.cum[ct.Email]
		if !ok {
			acc = &xray.ClientTraffic{Email: ct.Email}
			r.cum[ct.Email] = acc
		}
		acc.Up += ct.Up
		acc.Down += ct.Down
		if ct.Up > 0 || ct.Down > 0 {
			acc.LastOnline = nowMs
			activeEmails = append(activeEmails, ct.Email)
		}
	}

	var activeTags []string
	for _, t := range traffics {
		if t.IsInbound && (t.Up > 0 || t.Down > 0) {
			activeTags = append(activeTags, t.Tag)
		}
	}

	cumList := make([]*xray.ClientTraffic, 0, len(r.cum))
	for _, acc := range r.cum {
		cumList = append(cumList, acc)
	}

	proc.RefreshLocalOnline(activeEmails, activeTags, nowMs, onlineGraceMs)
	online := proc.GetLocalOnlineClients()

	return &wire.Report{
		InboundTraffics: traffics,
		ClientTraffics:  cumList,
		OnlineEmails:    online,
		Sys:             r.sysMetrics(proc),
	}, true
}

func (r *reporter) sysMetrics(proc *xray.Process) wire.SysMetrics {
	m := wire.SysMetrics{
		UptimeSecs:  proc.GetUptime(),
		XrayVersion: proc.GetXrayVersion(),
		XrayState:   "running",
	}
	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		m.CpuPct = pct[0]
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		m.MemPct = vm.UsedPercent
	}
	return m
}
