package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/fixes"
	"github.com/kaoriel-studio/netwick/internal/hostsfile"
	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/undo"
)

var PublicDNS = []string{"8.8.8.8", "8.8.4.4"}

var RiotProcesses = []string{"RiotClientServices", "RiotClientUx", "RiotClientUxRender", "RiotClientCrashHandler", "VALORANT", "VALORANT-Win64-Shipping"}

const (
	RPCreated  = "created"
	RPFailed   = "failed"
	RPSkipped  = "skipped"
	RPNotUsed  = "not_needed"
	MaxMessage = 64 * 1024
)

type Request struct {
	Undo []undo.Entry `json:"undo"`
}

type ActionResult struct {
	FixID          string      `json:"fixId"`
	OK             bool        `json:"ok"`
	ErrorKey       string      `json:"errorKey"`
	Changed        bool        `json:"changed"`
	RebootRequired bool        `json:"rebootRequired"`
	Undo           *undo.Entry `json:"undo,omitempty"`
}

type Response struct {
	RestorePoint string         `json:"restorePoint"`
	Results      []ActionResult `json:"results"`
	UndoneIDs    []string       `json:"undoneIds"`
}

type Executor struct {
	Sys platform.System
	Act platform.Actions
	Now func() time.Time
}

func (x *Executor) now() time.Time {
	if x.Now != nil {
		return x.Now()
	}
	return time.Now()
}

func needsRestorePoint(actions []string) bool {
	for _, a := range actions {
		if f, ok := fixes.Get(a); ok && f.RestorePoint {
			return true
		}
	}
	return false
}

func (x *Executor) Run(ctx context.Context, a Args, req Request) Response {
	resp := Response{RestorePoint: RPNotUsed, Results: []ActionResult{}, UndoneIDs: []string{}}
	if len(a.Actions) == 1 && a.Actions[0] == ActionUndo {
		resp.UndoneIDs, resp.Results = x.undoAll(ctx, req.Undo)
		return resp
	}
	if needsRestorePoint(a.Actions) {
		if a.SkipRestorePoint {
			resp.RestorePoint = RPSkipped
		} else if err := x.Act.CreateRestorePoint("Netwick: before changes"); err != nil {
			resp.RestorePoint = RPFailed
			return resp
		} else {
			resp.RestorePoint = RPCreated
		}
	}
	for _, id := range a.Actions {
		if ctx.Err() != nil {
			resp.Results = append(resp.Results, ActionResult{FixID: id, ErrorKey: "fix.error.cancelled"})
			continue
		}
		resp.Results = append(resp.Results, x.apply(ctx, id, a))
	}
	return resp
}

func fail(id, key string) ActionResult {
	return ActionResult{FixID: id, ErrorKey: key}
}

func (x *Executor) entry(id string) undo.Entry {
	return undo.Entry{ID: undo.NewID(), FixID: id, At: x.now().UTC()}
}

func (x *Executor) apply(ctx context.Context, id string, a Args) ActionResult {
	switch id {
	case "F02":
		if err := x.Act.FlushDNS(ctx); err != nil {
			return fail(id, "fix.error.generic")
		}
		return ActionResult{FixID: id, OK: true, Changed: true}
	case "F03":
		if err := x.Act.RenewIP(ctx); err != nil {
			return fail(id, "fix.error.generic")
		}
		_ = x.Act.FlushDNS(ctx)
		return ActionResult{FixID: id, OK: true, Changed: true}
	case "F04":
		return x.applyDNS(id, a.DNSAdapterLUID)
	case "F05":
		if err := x.Act.ResetNetworkStack(ctx); err != nil {
			return fail(id, "fix.error.generic")
		}
		return ActionResult{FixID: id, OK: true, Changed: true, RebootRequired: true}
	case "F06":
		return x.applyFirewall(id)
	case "F07":
		return x.applyVGC(id)
	case "F08":
		prev, err := x.Act.GetCrashDumpEnabled()
		if err != nil {
			return fail(id, "fix.error.generic")
		}
		if prev.Exists && prev.Value == 0 {
			return ActionResult{FixID: id, OK: true}
		}
		if err := x.Act.SetCrashDumpEnabled(platform.RegDWORD{Exists: true, Value: 0}); err != nil {
			return fail(id, "fix.error.generic")
		}
		e := x.entry(id)
		e.CrashDump = &prev
		return ActionResult{FixID: id, OK: true, Changed: true, RebootRequired: true, Undo: &e}
	case "F09":
		return x.applyKiller(ctx, id)
	case "F10":
		return x.applyHosts(ctx, id)
	}
	return fail(id, "fix.error.unknown")
}

func (x *Executor) applyDNS(id string, luid uint64) ActionResult {
	ads, err := x.Sys.Adapters()
	if err != nil {
		return fail(id, "fix.error.generic")
	}
	found := false
	for _, ad := range checks.ActiveAdapters(ads) {
		if ad.LUID == luid {
			found = true
		}
	}
	if !found {
		return fail(id, "fix.error.adapter")
	}
	prev, err := x.Act.GetDNS(luid)
	if err != nil {
		return fail(id, "fix.error.generic")
	}
	if !prev.DHCP && strings.Join(prev.Servers, ",") == strings.Join(PublicDNS, ",") {
		return ActionResult{FixID: id, OK: true}
	}
	if err := x.Act.SetDNS(luid, false, PublicDNS); err != nil {
		return fail(id, "fix.error.generic")
	}
	_ = x.Act.FlushDNS(context.Background())
	e := x.entry(id)
	prev.LUID = luid
	e.DNS = &prev
	return ActionResult{FixID: id, OK: true, Changed: true, Undo: &e}
}

func (x *Executor) applyFirewall(id string) ActionResult {
	inst := checks.ResolveInstall(x.Sys)
	var rules []platform.FirewallAllow
	for _, p := range inst.Binaries() {
		base := checks.WinBase(p)
		allowed := false
		for _, a := range undo.AllowedRuleApps {
			if strings.EqualFold(a, base) {
				allowed = true
				base = a
			}
		}
		if !allowed {
			continue
		}
		name := undo.RuleName(base)
		if ok, _ := x.Act.FirewallRuleExists(name); ok {
			continue
		}
		rules = append(rules, platform.FirewallAllow{Name: name, Application: p})
	}
	if len(inst.Binaries()) == 0 {
		return fail(id, "fix.error.no_install")
	}
	if len(rules) == 0 {
		return ActionResult{FixID: id, OK: true}
	}
	if err := x.Act.AddFirewallAllow(rules); err != nil {
		return fail(id, "fix.error.generic")
	}
	e := x.entry(id)
	for _, r := range rules {
		e.FirewallRules = append(e.FirewallRules, r.Name)
	}
	return ActionResult{FixID: id, OK: true, Changed: true, Undo: &e}
}

func (x *Executor) applyVGC(id string) ActionResult {
	svc, err := x.Sys.Service("vgc")
	if err != nil || !svc.Exists {
		return fail(id, "fix.error.vgc_missing")
	}
	if svc.StartType == platform.StartAutomatic {
		return ActionResult{FixID: id, OK: true}
	}
	if err := x.Act.SetServiceStartType("vgc", platform.StartAutomatic); err != nil {
		return fail(id, "fix.error.generic")
	}
	e := x.entry(id)
	e.Service = &platform.ServicePrev{Name: "vgc", StartType: svc.StartType, WasRunning: svc.State == platform.StateRunning}
	return ActionResult{FixID: id, OK: true, Changed: true, RebootRequired: true, Undo: &e}
}

func (x *Executor) applyKiller(ctx context.Context, id string) ActionResult {
	svc, err := x.Sys.ServiceByDisplayName(checks.KillerDisplayName)
	if err != nil || !svc.Exists {
		return ActionResult{FixID: id, OK: true}
	}
	running := svc.State == platform.StateRunning
	if running {
		if err := x.Act.StopService(ctx, svc.Name); err != nil {
			return fail(id, "fix.error.generic")
		}
	}
	if err := x.Act.SetServiceStartType(svc.Name, platform.StartDisabled); err != nil {
		return fail(id, "fix.error.generic")
	}
	e := x.entry(id)
	e.Service = &platform.ServicePrev{StartType: svc.StartType, WasRunning: running}
	return ActionResult{FixID: id, OK: true, Changed: true, Undo: &e}
}

func (x *Executor) backupPath(backupID string) string {
	return filepath.Join(x.Act.BackupDir(), "hosts-"+backupID+".bak")
}

func (x *Executor) applyHosts(ctx context.Context, id string) ActionResult {
	orig, err := x.Sys.ReadFile(x.Sys.HostsPath(), 4<<20)
	if err != nil {
		return fail(id, "fix.error.generic")
	}
	cleaned, _, changed := hostsfile.Clean(orig)
	if !changed {
		return ActionResult{FixID: id, OK: true}
	}
	bid := undo.NewBackupID(x.now())
	if err := os.MkdirAll(x.Act.BackupDir(), 0o700); err != nil {
		return fail(id, "fix.error.backup")
	}
	if err := undo.WriteAtomic(x.backupPath(bid), orig); err != nil {
		return fail(id, "fix.error.backup")
	}
	if err := x.Act.WriteHosts(cleaned); err != nil {
		return fail(id, "fix.error.hosts_locked")
	}
	_ = x.Act.FlushDNS(ctx)
	e := x.entry(id)
	e.HostsBackupID = bid
	return ActionResult{FixID: id, OK: true, Changed: true, Undo: &e}
}

func (x *Executor) undoAll(ctx context.Context, entries []undo.Entry) ([]string, []ActionResult) {
	done := []string{}
	results := []ActionResult{}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if err := undo.Validate(e); err != nil {
			results = append(results, fail(e.FixID, "fix.error.invalid_undo"))
			continue
		}
		if err := x.undoOne(ctx, e); err != nil {
			results = append(results, fail(e.FixID, "fix.error.undo_failed"))
			continue
		}
		done = append(done, e.ID)
		results = append(results, ActionResult{FixID: e.FixID, OK: true, Changed: true})
	}
	return done, results
}

var errSkip = errors.New("nothing to undo")

func (x *Executor) undoOne(ctx context.Context, e undo.Entry) error {
	switch e.FixID {
	case "F04":
		return x.Act.SetDNS(e.DNS.LUID, e.DNS.DHCP, e.DNS.Servers)
	case "F06":
		return x.Act.RemoveFirewallRules(e.FirewallRules)
	case "F07":
		if err := x.Act.SetServiceStartType("vgc", e.Service.StartType); err != nil {
			return err
		}
		return nil
	case "F08":
		return x.Act.SetCrashDumpEnabled(*e.CrashDump)
	case "F09":
		svc, err := x.Sys.ServiceByDisplayName(checks.KillerDisplayName)
		if err != nil || !svc.Exists {
			return errSkip
		}
		if err := x.Act.SetServiceStartType(svc.Name, e.Service.StartType); err != nil {
			return err
		}
		if e.Service.WasRunning {
			return x.Act.StartService(svc.Name)
		}
		return nil
	case "F10":
		backup, err := os.ReadFile(x.backupPath(e.HostsBackupID))
		if err != nil {
			return err
		}
		_, removed, changed := hostsfile.Clean(backup)
		if !changed {
			return nil
		}
		cur, err := x.Sys.ReadFile(x.Sys.HostsPath(), 4<<20)
		if err != nil {
			return err
		}
		if err := x.Act.WriteHosts(hostsfile.Restore(cur, removed)); err != nil {
			return err
		}
		_ = x.Act.FlushDNS(ctx)
		return nil
	}
	return errSkip
}

func RestartClient(sys platform.System, act platform.Actions, wait time.Duration) error {
	inst := checks.ResolveInstall(sys)
	if inst.ClientExe == "" || !strings.EqualFold(checks.WinBase(inst.ClientExe), "RiotClientServices.exe") {
		return platform.ErrNotFound
	}
	if n, _ := act.TerminateProcesses(RiotProcesses); n > 0 && wait > 0 {
		time.Sleep(wait)
	}
	return act.StartProcess(inst.ClientExe, []string{"--launch-product=valorant", "--launch-patchline=live"})
}
