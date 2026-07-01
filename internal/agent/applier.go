package agent

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type Applier struct {
	mu   sync.Mutex
	proc *xray.Process
}

func (a *Applier) Apply(cfg *xray.Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proc == nil || !a.proc.IsRunning() {
		return a.startLocked(cfg)
	}
	old := a.proc.GetConfig()
	diff, ok := xray.ComputeHotDiff(old, cfg)
	if !ok {
		return a.restartLocked(cfg)
	}
	if diff.Empty() {
		a.proc.SetConfig(cfg)
		return nil
	}
	apiPort := a.proc.GetAPIPort()
	if apiPort <= 0 {
		return a.restartLocked(cfg)
	}
	api := xray.XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		return a.restartLocked(cfg)
	}
	defer api.Close()
	if err := xray.ApplyHotDiff(&api, diff); err != nil {
		return a.restartLocked(cfg)
	}
	a.proc.SetConfig(cfg)
	return nil
}

func (a *Applier) startLocked(cfg *xray.Config) error {
	p := xray.NewProcess(cfg)
	if err := p.Start(); err != nil {
		return err
	}
	a.proc = p
	return nil
}

func (a *Applier) restartLocked(cfg *xray.Config) error {
	if a.proc != nil {
		_ = a.proc.Stop()
		a.proc = nil
	}
	return a.startLocked(cfg)
}

func (a *Applier) Process() *xray.Process {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.proc
}

func (a *Applier) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proc == nil {
		return nil
	}
	err := a.proc.Stop()
	a.proc = nil
	return err
}
