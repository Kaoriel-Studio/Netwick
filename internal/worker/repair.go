package worker

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/hostsfile"
	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/probe"
	"github.com/kaoriel-studio/netwick/internal/undo"
)

func CacheDirs(sys platform.System) []string {
	la := sys.Env("LOCALAPPDATA")
	if la == "" {
		return nil
	}
	return []string{
		checks.WinJoin(la, "Riot Games", "Riot Client", "Cache"),
		checks.WinJoin(la, "Riot Games", "Riot Client", "Crashpad"),
		checks.WinJoin(la, "VALORANT", "Saved", "Crashes"),
	}
}

func SessionFiles(sys platform.System) []string {
	la := sys.Env("LOCALAPPDATA")
	if la == "" {
		return nil
	}
	root := checks.WinJoin(la, "Riot Games", "Riot Client")
	return []string{
		checks.WinJoin(root, "Data", "RiotClientPrivateSettings.yaml"),
		checks.WinJoin(root, "Config", "RiotClientSettings.yaml"),
		checks.WinJoin(root, "Data", "RiotGamesPrivateSettings.yaml"),
	}
}

func exists(sys platform.System, p string) bool {
	_, err := sys.Stat(p)
	return err == nil
}

func closeClient(act platform.Actions, wait time.Duration) {
	if n, _ := act.TerminateProcesses(RiotProcesses); n > 0 && wait > 0 {
		time.Sleep(wait)
	}
}

func ApplyLocal(sys platform.System, act platform.Actions, id string, wait time.Duration) ActionResult {
	switch id {
	case "F01":
		if err := RestartClient(sys, act, wait); err != nil {
			return fail(id, "fix.error.no_client")
		}
		return ActionResult{FixID: id, OK: true, Changed: true}
	case "F11":
		closeClient(act, wait)
		changed := false
		for _, d := range CacheDirs(sys) {
			if !exists(sys, d) {
				continue
			}
			if err := act.RemoveTree(d); err != nil {
				return fail(id, "fix.error.files_locked")
			}
			changed = true
		}
		return ActionResult{FixID: id, OK: true, Changed: changed}
	case "F12":
		closeClient(act, wait)
		changed := false
		for _, f := range SessionFiles(sys) {
			if !exists(sys, f) {
				continue
			}
			if err := act.RemoveFile(f); err != nil {
				return fail(id, "fix.error.files_locked")
			}
			changed = true
		}
		return ActionResult{FixID: id, OK: true, Changed: changed}
	}
	return fail(id, "fix.error.unknown")
}

type RoutePlan struct {
	Host      string         `json:"host"`
	Core      bool           `json:"core"`
	IP        string         `json:"ip"`
	LatencyMs int            `json:"latencyMs"`
	Good      int            `json:"good"`
	Total     int            `json:"total"`
	Pinned    string         `json:"pinned"`
	Probes    []probe.Result `json:"probes"`
}

func (p RoutePlan) Needed() bool { return p.IP != "" && p.Pinned != p.IP }

func PlanRoutes(ctx context.Context, sys platform.System, hosts []string) []RoutePlan {
	pinned := map[string]string{}
	if b, err := sys.ReadFile(sys.HostsPath(), 4<<20); err == nil {
		pinned = hostsfile.Parse(b).Pinned
	}
	reports := probe.ProbeHosts(ctx, sys.Network(), hosts, pinned, 8, nil)
	plans := make([]RoutePlan, 0, len(reports))
	for _, r := range reports {
		p := RoutePlan{Host: r.Host, Core: r.Core, Total: len(r.Probes), Pinned: pinned[r.Host], Probes: r.Probes}
		var good []probe.Result
		for _, pr := range r.Probes {
			if pr.OK() {
				good = append(good, pr)
			}
		}
		sort.SliceStable(good, func(i, j int) bool { return good[i].LatencyMs < good[j].LatencyMs })
		p.Good = len(good)
		pinnedDead := p.Pinned != "" && !probeOK(r.Probes, p.Pinned)
		if len(good) > 0 && (len(good) < len(r.Probes) || pinnedDead) {
			p.IP = good[0].IP
			p.LatencyMs = good[0].LatencyMs
		}
		if p.Pinned != "" && !pinnedDead {
			p.IP = p.Pinned
		}
		plans = append(plans, p)
	}
	return plans
}

func probeOK(list []probe.Result, ip string) bool {
	for _, p := range list {
		if p.IP == ip {
			return p.OK()
		}
	}
	return false
}

func (x *Executor) applyRoutes(ctx context.Context, id string, hosts []string) ActionResult {
	if len(hosts) == 0 {
		return fail(id, "fix.error.generic")
	}
	routes := map[string]string{}
	for _, p := range PlanRoutes(ctx, x.Sys, hosts) {
		if p.IP != "" {
			routes[p.Host] = p.IP
		}
	}
	orig, err := x.Sys.ReadFile(x.Sys.HostsPath(), 4<<20)
	if err != nil {
		return fail(id, "fix.error.generic")
	}
	cur := hostsfile.Parse(orig).Pinned
	same := len(routes) > 0
	for h, ip := range routes {
		if cur[h] != ip {
			same = false
		}
	}
	if len(routes) == 0 || same {
		return ActionResult{FixID: id, OK: true}
	}
	updated, removed := hostsfile.ApplyRoutes(orig, routes, x.now().Format("2006-01-02"))
	bid := undo.NewBackupID(x.now())
	if err := os.MkdirAll(x.Act.BackupDir(), 0o700); err != nil {
		return fail(id, "fix.error.backup")
	}
	if err := undo.WriteAtomic(x.backupPath(bid), orig); err != nil {
		return fail(id, "fix.error.backup")
	}
	if err := undo.WriteAtomic(x.removedPath(bid), []byte(strings.Join(removed, "\n"))); err != nil {
		return fail(id, "fix.error.backup")
	}
	if err := x.Act.WriteHosts(updated); err != nil {
		return fail(id, "fix.error.hosts_locked")
	}
	_ = x.Act.FlushDNS(ctx)
	e := x.entry(id)
	e.HostsBackupID = bid
	return ActionResult{FixID: id, OK: true, Changed: true, Undo: &e}
}

func (x *Executor) restartVGC(ctx context.Context, id string) ActionResult {
	svc, err := x.Sys.Service("vgc")
	if err != nil || !svc.Exists {
		return fail(id, "fix.error.vgc_missing")
	}
	if svc.StartType == platform.StartDisabled {
		return fail(id, "fix.error.vgc_disabled")
	}
	if svc.State == platform.StateRunning {
		if err := x.Act.StopService(ctx, "vgc"); err != nil {
			return fail(id, "fix.error.generic")
		}
	}
	if err := x.Act.StartService("vgc"); err != nil {
		return fail(id, "fix.error.generic")
	}
	return ActionResult{FixID: id, OK: true, Changed: true}
}

func (x *Executor) stopBonjour(ctx context.Context, id string) ActionResult {
	svc, err := x.Sys.Service(checks.BonjourService)
	if err != nil || !svc.Exists || svc.State != platform.StateRunning {
		return ActionResult{FixID: id, OK: true}
	}
	if err := x.Act.StopService(ctx, checks.BonjourService); err != nil {
		return fail(id, "fix.error.generic")
	}
	e := x.entry(id)
	e.Service = &platform.ServicePrev{Name: checks.BonjourService, StartType: svc.StartType, WasRunning: true}
	return ActionResult{FixID: id, OK: true, Changed: true, Undo: &e}
}

func (x *Executor) removedPath(backupID string) string {
	return x.backupPath(backupID) + ".removed"
}
