package services

import (
	"time"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

type DemoService struct {
	st        *State
	Scenarios []string
}

func NewDemoService(st *State, scenarios []string) *DemoService {
	return &DemoService{st: st, Scenarios: scenarios}
}

type DemoInfo struct {
	Active    bool     `json:"active"`
	Scenario  string   `json:"scenario"`
	Scenarios []string `json:"scenarios"`
}

func (d *DemoService) Info() DemoInfo {
	d.st.mu.Lock()
	defer d.st.mu.Unlock()
	return DemoInfo{Active: d.st.Env.Demo, Scenario: d.st.Env.Scenario, Scenarios: append([]string{}, d.Scenarios...)}
}

func (d *DemoService) SetScenario(name string) (DemoInfo, error) {
	if !d.st.Env.Demo || d.st.Env.ResetDemo == nil {
		return DemoInfo{}, ErrInvalid
	}
	known := false
	for _, s := range d.Scenarios {
		if s == name {
			known = true
		}
	}
	if !known {
		return DemoInfo{}, ErrInvalid
	}
	d.st.mu.Lock()
	if d.st.running {
		d.st.mu.Unlock()
		return DemoInfo{}, ErrBusy
	}
	d.st.mu.Unlock()
	if !d.st.Env.ResetDemo(name) {
		return DemoInfo{}, ErrInvalid
	}
	var ids []string
	for _, e := range d.st.Store.Journal.List() {
		ids = append(ids, e.ID)
	}
	_ = d.st.Store.Journal.Remove(ids)
	d.st.mu.Lock()
	d.st.Env.Scenario = name
	d.st.results = map[string]core.Result{}
	d.st.verdict = verdict.Verdict{}
	d.st.ranAt = time.Time{}
	d.st.history = nil
	d.st.mu.Unlock()
	return d.Info(), nil
}
