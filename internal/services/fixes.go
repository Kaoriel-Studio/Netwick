package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/app"
	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/fixes"
	"github.com/kaoriel-studio/netwick/internal/logscan"
	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/probe"
	"github.com/kaoriel-studio/netwick/internal/profile"
	"github.com/kaoriel-studio/netwick/internal/undo"
	"github.com/kaoriel-studio/netwick/internal/worker"
)

type FixService struct{ st *State }

func NewFixService(st *State) *FixService { return &FixService{st: st} }

type FixView struct {
	fixes.Fix
	AskOnDemand    bool   `json:"askOnDemand"`
	Adapter        string `json:"adapter"`
	AdapterMissing bool   `json:"adapterMissing"`
	Allowed        bool   `json:"allowed"`
	BlockReason    string `json:"blockReason"`
}

type GuideView struct {
	fixes.Guide
}

type ApplyOptions struct {
	OnDemand string   `json:"onDemand"`
	Context  string   `json:"context"`
	Hosts    []string `json:"hosts"`
}

type Outcome struct {
	FixID           string `json:"fixId"`
	OK              bool   `json:"ok"`
	Changed         bool   `json:"changed"`
	ErrorKey        string `json:"errorKey"`
	RebootRequired  bool   `json:"rebootRequired"`
	BlockedOnDemand bool   `json:"blockedOnDemand"`
	Declined        bool   `json:"declined"`
	UndoAvailable   bool   `json:"undoAvailable"`
}

type UndoItem struct {
	ID    string    `json:"id"`
	FixID string    `json:"fixId"`
	At    time.Time `json:"at"`
}

type UndoOutcome struct {
	OK       bool     `json:"ok"`
	Undone   int      `json:"undone"`
	Failed   []string `json:"failed"`
	Declined bool     `json:"declined"`
	ErrorKey string   `json:"errorKey"`
}

func (f *FixService) onDemandPossible() bool {
	sys := f.st.Env.Sys
	v, _ := sys.OSVersion()
	tpm, _ := sys.TPM()
	sb, _ := sys.SecureBoot()
	dg, _ := sys.DeviceGuard()
	od := checks.EvaluateOnDemand(v, tpm, sb, dg)
	return len(od.Missing) == 0
}

func (f *FixService) dnsAdapter() (platform.Adapter, bool) {
	ads, err := f.st.Env.Sys.Adapters()
	if err != nil {
		return platform.Adapter{}, false
	}
	act := checks.ActiveAdapters(ads)
	for _, a := range act {
		if a.IfType == platform.IfTypeEthernet {
			return a, true
		}
	}
	if len(act) > 0 {
		return act[0], true
	}
	return platform.Adapter{}, false
}

func contextAllowed(fix fixes.Fix, ctx string) bool {
	return fix.Scenario == "" || fix.Scenario == ctx
}

func (f *FixService) Describe(id string, context string) (*FixView, error) {
	fix, ok := fixes.Get(id)
	if !ok || len(context) > 40 {
		return nil, ErrInvalid
	}
	v := &FixView{Fix: fix, Allowed: true}
	if !contextAllowed(fix, context) {
		v.Allowed = false
		v.BlockReason = "scenario"
	}
	if id == "F07" && f.onDemandPossible() {
		v.AskOnDemand = true
	}
	if id == "F04" {
		if a, ok := f.dnsAdapter(); ok {
			v.Adapter = a.Name
		} else {
			v.AdapterMissing = true
			v.Allowed = false
			v.BlockReason = "adapter"
		}
	}
	sys := f.st.Env.Sys
	switch id {
	case "F13":
		if svc, err := sys.Service("vgc"); err != nil || !svc.Exists {
			v.Allowed = false
			v.BlockReason = "vgc_missing"
		}
	case "F15":
		if svc, err := sys.Service(checks.BonjourService); err != nil || !svc.Exists || svc.State != platform.StateRunning {
			v.Allowed = false
			v.BlockReason = "not_needed"
		}
	}
	return v, nil
}

func validRouteHosts(hosts []string) bool {
	if len(hosts) == 0 || len(hosts) > worker.MaxRouteHosts {
		return false
	}
	seen := map[string]bool{}
	for _, h := range hosts {
		if seen[h] || !probe.IsRiotHost(h) || h != strings.ToLower(h) {
			return false
		}
		seen[h] = true
	}
	return true
}

func (f *FixService) Guide(id string) (*GuideView, error) {
	g, ok := fixes.GetGuide(id)
	if !ok {
		return nil, ErrInvalid
	}
	return &GuideView{Guide: g}, nil
}

func newPipe() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (f *FixService) Apply(id string, opts ApplyOptions) (*Outcome, error) {
	fix, ok := fixes.Get(id)
	if !ok || len(opts.Context) > 40 {
		return nil, ErrInvalid
	}
	switch opts.OnDemand {
	case "", "yes", "no", "unknown":
	default:
		return nil, ErrInvalid
	}
	if !f.st.fixMu.TryLock() {
		return nil, ErrBusy
	}
	defer f.st.fixMu.Unlock()
	out := &Outcome{FixID: id}
	if !contextAllowed(fix, opts.Context) {
		out.ErrorKey = "fix.error.scenario"
		return out, nil
	}
	if id == "F07" && f.onDemandPossible() && opts.OnDemand != "no" {
		out.BlockedOnDemand = true
		return out, nil
	}
	if fix.Local {
		r := worker.ApplyLocal(f.st.Env.Sys, f.st.Env.Act, id, 2*time.Second)
		out.OK, out.Changed, out.ErrorKey = r.OK, r.Changed, r.ErrorKey
		f.st.addHistory(FixRecord{FixID: id, At: time.Now(), OK: out.OK})
		if out.OK && out.Changed {
			f.st.Prof.Bump(profile.CFixes)
		}
		return out, nil
	}
	a := worker.Args{Pipe: newPipe(), Actions: []string{id}}
	if id == "F14" {
		hosts := opts.Hosts
		if len(hosts) == 0 {
			hosts = append([]string{}, probe.CoreHosts...)
		}
		if !validRouteHosts(hosts) {
			return nil, ErrInvalid
		}
		a.RouteHosts = hosts
	}
	if id == "F04" {
		ad, ok := f.dnsAdapter()
		if !ok {
			out.ErrorKey = "fix.error.adapter"
			return out, nil
		}
		a.DNSAdapterLUID = ad.LUID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	resp, err := f.st.Env.Runner.Run(ctx, a, worker.Request{})
	if errors.Is(err, app.ErrDeclined) {
		out.Declined = true
		out.ErrorKey = "fix.error.declined"
		return out, nil
	}
	if err != nil {
		out.ErrorKey = "fix.error.worker"
		return out, nil
	}
	for _, r := range resp.Results {
		if r.FixID != id {
			continue
		}
		out.OK = r.OK
		out.Changed = r.Changed
		out.ErrorKey = r.ErrorKey
		out.RebootRequired = r.RebootRequired
		if r.Undo != nil && undo.Validate(*r.Undo) == nil {
			_ = f.st.Store.Journal.Add(*r.Undo)
		}
	}
	if !out.OK && out.ErrorKey == "" {
		out.ErrorKey = "fix.error.generic"
	}
	out.UndoAvailable = len(f.st.Store.Journal.List()) > 0
	f.st.addHistory(FixRecord{FixID: id, At: time.Now(), OK: out.OK})
	if out.OK && out.Changed {
		f.st.Prof.Bump(profile.CFixes)
	}
	return out, nil
}

func (f *FixService) UndoList() []UndoItem {
	items := []UndoItem{}
	for _, e := range f.st.Store.Journal.List() {
		items = append(items, UndoItem{ID: e.ID, FixID: e.FixID, At: e.At})
	}
	return items
}

func (f *FixService) UndoAll() (*UndoOutcome, error) {
	if !f.st.fixMu.TryLock() {
		return nil, ErrBusy
	}
	defer f.st.fixMu.Unlock()
	entries := f.st.Store.Journal.List()
	out := &UndoOutcome{Failed: []string{}}
	if len(entries) == 0 {
		out.OK = true
		return out, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	resp, err := f.st.Env.Runner.Run(ctx, worker.Args{Pipe: newPipe(), Actions: []string{worker.ActionUndo}}, worker.Request{Undo: entries})
	if errors.Is(err, app.ErrDeclined) {
		out.Declined = true
		out.ErrorKey = "fix.error.declined"
		return out, nil
	}
	if err != nil {
		out.ErrorKey = "fix.error.worker"
		return out, nil
	}
	_ = f.st.Store.Journal.Remove(resp.UndoneIDs)
	out.Undone = len(resp.UndoneIDs)
	if out.Undone > 0 {
		f.st.Prof.Bump(profile.CUndos)
	}
	for _, r := range resp.Results {
		if !r.OK {
			out.Failed = append(out.Failed, r.FixID)
		}
	}
	out.OK = len(out.Failed) == 0
	f.st.mu.Lock()
	for i := range f.st.history {
		f.st.history[i].Undone = true
	}
	f.st.mu.Unlock()
	return out, nil
}

func (f *FixService) LaunchGame() (*Outcome, error) {
	return f.Apply("F01", ApplyOptions{})
}

type RebootOutcome struct {
	OK        bool `json:"ok"`
	Simulated bool `json:"simulated"`
}

func (f *FixService) Reboot() RebootOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := f.st.Env.Act.ScheduleReboot(ctx)
	f.st.addHistory(FixRecord{FixID: "reboot", At: time.Now(), OK: err == nil})
	return RebootOutcome{OK: err == nil, Simulated: f.st.Env.Demo}
}

func (f *FixService) CancelReboot() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return f.st.Env.Act.CancelReboot(ctx) == nil
}

type NodesView struct {
	Plans     []worker.RoutePlan `json:"plans"`
	Needed    int                `json:"needed"`
	Pinned    int                `json:"pinned"`
	AllDown   int                `json:"allDown"`
	Cancelled bool               `json:"cancelled"`
}

func (f *FixService) Nodes() (*NodesView, error) {
	if !f.st.nodesMu.TryLock() {
		return nil, ErrBusy
	}
	defer f.st.nodesMu.Unlock()
	hosts := append([]string{}, probe.CoreHosts...)
	if r, ok := f.st.Results()["C20"]; ok {
		if sum, ok := r.Data.(logscan.Summary); ok {
			for _, h := range core.SortedKeys(sum.Hosts) {
				if !probe.IsCoreHost(h) && len(hosts) < worker.MaxRouteHosts {
					hosts = append(hosts, h)
				}
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	plans := worker.PlanRoutes(ctx, f.st.Env.Sys, hosts)
	v := &NodesView{Plans: plans}
	for _, p := range plans {
		if p.Needed() {
			v.Needed++
		}
		if p.Pinned != "" {
			v.Pinned++
		}
		if p.Good == 0 {
			v.AllDown++
		}
	}
	v.Cancelled = ctx.Err() != nil
	f.st.Prof.Bump(profile.CNodes)
	return v, nil
}

func (f *FixService) CompleteGuide(id string) bool {
	if _, ok := fixes.GetGuide(id); !ok {
		if _, ok := fixes.InfoSteps[id]; !ok {
			return false
		}
	}
	if f.st.Env.Demo && f.st.Env.Simulate != nil {
		f.st.Env.Simulate(id)
		return true
	}
	return false
}

func (f *FixService) Open(target string) error {
	if !fixes.ValidOpener(target) {
		return ErrInvalid
	}
	if f.st.Env.OpenSystem == nil {
		return errors.New("unavailable")
	}
	return f.st.Env.OpenSystem(fixes.Opener(target))
}

type LocalRunner struct {
	Exec *worker.Executor
}

func (r LocalRunner) Run(ctx context.Context, a worker.Args, req worker.Request) (worker.Response, error) {
	return r.Exec.Run(ctx, a, req), nil
}
