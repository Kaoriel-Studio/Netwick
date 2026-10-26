package checks

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/appcfg"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/hostsfile"
	"github.com/kaoriel-studio/netwick/internal/logscan"
	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/probe"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

const MaxSecondaryHosts = 6

type ServersData struct {
	Reports []probe.HostReport    `json:"reports"`
	Summary verdict.ServerSummary `json:"summary"`
}

type ServersCheck struct {
	base
	OnProgress probe.ProgressFunc
}

func (c *ServersCheck) Timeout() time.Duration { return 9500 * time.Millisecond }
func (c *ServersCheck) DependsOn() []string    { return []string{"C19", "C20"} }
func (c *ServersCheck) Run(ctx context.Context, sys platform.System) core.Result {
	return c.RunWith(ctx, sys, nil)
}

func (c *ServersCheck) RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result {
	var pinned map[string]string
	if r, ok := deps["C19"]; ok {
		if p, ok := r.Data.(hostsfile.Parsed); ok {
			pinned = p.Pinned
		}
	}
	logHosts := map[string]int{}
	latestHosts := map[string]int{}
	if r, ok := deps["C20"]; ok {
		if s, ok := r.Data.(logscan.Summary); ok {
			logHosts = s.Hosts
			if s.Latest != nil {
				latestHosts = s.Latest
			}
		}
	}
	hosts := append([]string{}, probe.CoreHosts...)
	var extra []string
	for h := range logHosts {
		if !probe.IsCoreHost(h) {
			extra = append(extra, h)
		}
	}
	sort.Slice(extra, func(i, j int) bool {
		if logHosts[extra[i]] != logHosts[extra[j]] {
			return logHosts[extra[i]] > logHosts[extra[j]]
		}
		return extra[i] < extra[j]
	})
	if len(extra) > MaxSecondaryHosts {
		extra = extra[:MaxSecondaryHosts]
	}
	hosts = append(hosts, extra...)

	reports := probe.ProbeHosts(ctx, sys.Network(), hosts, pinned, 8, c.OnProgress)
	r := verdict.Servers(c.id, c.group, reports, latestHosts)
	r.Data = ServersData{Reports: reports, Summary: verdict.Summarize(reports, latestHosts)}
	return r
}

type ProxyVPNCheck struct{ base }

func IsVPNAdapter(a platform.Adapter) bool {
	d := strings.ToLower(a.Description + " " + a.Name)
	for _, p := range appcfg.Get().VPNPatterns {
		if strings.Contains(d, p) {
			return true
		}
	}
	return a.IfType == platform.IfTypePPP
}

func (c *ProxyVPNCheck) Run(ctx context.Context, sys platform.System) core.Result {
	px, errP := sys.Proxy()
	ads, errA := sys.Adapters()
	if errP != nil && errA != nil {
		return c.unknown(errP)
	}
	var vpns []string
	for _, a := range ads {
		if a.Up && IsVPNAdapter(a) {
			vpns = append(vpns, a.Description)
		}
	}
	r := c.result(core.StatusOK, "proxyvpn.ok")
	if px.Enabled && px.Server != "" {
		r.Status = core.StatusWarn
		r.MessageKey = "proxyvpn.proxy"
		r.AddDetail(core.StatusWarn, "proxyvpn.proxy", nil)
	}
	if len(vpns) > 0 {
		r.Status = core.StatusWarn
		if r.MessageKey == "proxyvpn.proxy" {
			r.MessageKey = "proxyvpn.both"
		} else {
			r.MessageKey = "proxyvpn.vpn"
		}
		r.Params["adapters"] = strings.Join(vpns, ", ")
		r.AddDetail(core.StatusWarn, "proxyvpn.vpn", map[string]string{"adapters": strings.Join(vpns, ", ")})
	}
	if r.Status == core.StatusWarn {
		r.GuideIDs = []string{"G08"}
	}
	r.SourceURL = core.SrcConnection
	return r
}

type FirewallCheck struct{ base }

func (c *FirewallCheck) DependsOn() []string { return []string{"C01"} }
func (c *FirewallCheck) Run(ctx context.Context, sys platform.System) core.Result {
	return c.RunWith(ctx, sys, nil)
}
func (c *FirewallCheck) RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result {
	inst := installFrom(sys, deps)
	bins := inst.Binaries()
	if len(bins) == 0 {
		r := c.result(core.StatusUnknown, "firewall.no_install")
		r.SourceURL = core.SrcFirewall
		return r
	}
	rules, err := sys.FirewallRules()
	if err != nil {
		r := c.unknown(err)
		r.SourceURL = core.SrcFirewall
		return r
	}
	var blocked []string
	for _, rule := range rules {
		if !rule.Enabled || !rule.Block || rule.Application == "" {
			continue
		}
		app := expandEnv(sys, rule.Application)
		for _, b := range bins {
			if strings.EqualFold(app, b) {
				blocked = append(blocked, WinBase(b))
			}
		}
	}
	var r core.Result
	if len(blocked) > 0 {
		r = c.result(core.StatusFail, "firewall.blocked")
		r.Params["apps"] = strings.Join(uniq(blocked), ", ")
		r.Steps = []core.Step{core.Guide("G14"), core.Fix("F06")}
	} else {
		r = c.result(core.StatusOK, "firewall.ok")
	}
	r.SourceURL = core.SrcFirewall
	return r
}

func expandEnv(sys platform.System, p string) string {
	for _, v := range []string{"ProgramFiles", "ProgramData", "LOCALAPPDATA", "SystemRoot", "SystemDrive"} {
		token := "%" + v + "%"
		if i := strings.Index(strings.ToLower(p), strings.ToLower(token)); i >= 0 {
			p = p[:i] + sys.Env(v) + p[i+len(token):]
		}
	}
	return WinJoin(p)
}

func uniq(l []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range l {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

type ConnectionTypeCheck struct{ base }

func ActiveAdapters(ads []platform.Adapter) []platform.Adapter {
	var out []platform.Adapter
	for _, a := range ads {
		if a.Up && a.HasGateway && !IsVPNAdapter(a) && a.IfType != 24 {
			out = append(out, a)
		}
	}
	return out
}

func (c *ConnectionTypeCheck) Run(ctx context.Context, sys platform.System) core.Result {
	ads, err := sys.Adapters()
	if err != nil {
		return c.unknown(err)
	}
	act := ActiveAdapters(ads)
	var r core.Result
	wired, wifi := false, false
	for _, a := range act {
		switch a.IfType {
		case platform.IfTypeEthernet:
			wired = true
		case platform.IfTypeWiFi:
			wifi = true
		}
	}
	switch {
	case len(act) == 0:
		r = c.result(core.StatusFail, "conn.offline")
		r.Steps = []core.Step{core.Guide("G10")}
	case wired:
		r = c.result(core.StatusOK, "conn.wired")
	case wifi:
		r = c.result(core.StatusInfo, "conn.wifi")
		r.GuideIDs = []string{"G10"}
	default:
		r = c.result(core.StatusOK, "conn.other")
	}
	r.SourceURL = core.SrcConnection
	r.Data = act
	return r
}

type NetworkHogsCheck struct{ base }

func (c *NetworkHogsCheck) Run(ctx context.Context, sys platform.System) core.Result {
	procs, err := sys.Processes()
	if err != nil {
		return c.unknown(err)
	}
	running := map[string]bool{}
	for _, p := range procs {
		running[strings.ToLower(p.Name)] = true
	}
	var found []string
	for _, h := range appcfg.Get().NetworkHogs {
		if running[strings.ToLower(h.Process)] {
			found = append(found, h.Name)
		}
	}
	var r core.Result
	if len(found) > 0 {
		r = c.result(core.StatusInfo, "hogs.found")
		r.Params["apps"] = strings.Join(found, ", ")
		r.GuideIDs = []string{"G10"}
	} else {
		r = c.result(core.StatusOK, "hogs.none")
	}
	r.SourceURL = core.SrcConnection
	r.Data = found
	return r
}

const KillerDisplayName = "Killer Network Service"

type KillerCheck struct{ base }

func (c *KillerCheck) Run(ctx context.Context, sys platform.System) core.Result {
	s, err := sys.ServiceByDisplayName(KillerDisplayName)
	var r core.Result
	switch {
	case err == nil && s.Exists && s.StartType != platform.StartDisabled:
		r = c.result(core.StatusInfo, "killer.found")
		r.Params["state"] = string(s.State)
	default:
		r = c.result(core.StatusOK, "killer.none")
	}
	r.SourceURL = core.SrcKiller
	r.Data = s
	return r
}

type HostsCheck struct{ base }

func (c *HostsCheck) Run(ctx context.Context, sys platform.System) core.Result {
	b, err := sys.ReadFile(sys.HostsPath(), 4<<20)
	if err != nil {
		r := c.unknown(err)
		r.Data = hostsfile.Parsed{Pinned: map[string]string{}}
		return r
	}
	p := hostsfile.Parse(b)
	var legacy, managed []hostsfile.Entry
	for _, e := range p.RiotEntries {
		if e.Section == hostsfile.ManagedMarker {
			managed = append(managed, e)
		} else {
			legacy = append(legacy, e)
		}
	}
	var sections []string
	for _, sct := range p.Sections {
		if sct != hostsfile.ManagedMarker {
			sections = append(sections, sct)
		}
	}
	var r core.Result
	switch {
	case len(legacy) > 0 || len(sections) > 0:
		r = c.result(core.StatusInfo, "hosts.legacy")
		r.Params["count"] = strconv.Itoa(len(legacy))
		r.Params["sections"] = strings.Join(sections, ", ")
		r.FixIDs = []string{"F10"}
		for _, e := range legacy {
			r.AddDetail(core.StatusInfo, "hosts.entry", map[string]string{"ip": e.IP, "host": e.Host})
		}
	case len(managed) > 0:
		r = c.result(core.StatusOK, "hosts.managed")
		r.Params["count"] = strconv.Itoa(len(managed))
	default:
		r = c.result(core.StatusOK, "hosts.clean")
	}
	for _, e := range managed {
		r.AddDetail(core.StatusOK, "hosts.route", map[string]string{"ip": e.IP, "host": e.Host})
	}
	r.Data = p
	return r
}

type LogsCheck struct{ base }

func (c *LogsCheck) DependsOn() []string { return []string{"C01"} }
func (c *LogsCheck) Run(ctx context.Context, sys platform.System) core.Result {
	return c.RunWith(ctx, sys, nil)
}

func LogRoots(sys platform.System, inst Install) []string {
	var roots []string
	if la := sys.Env("LOCALAPPDATA"); la != "" {
		roots = append(roots, WinJoin(la, "Riot Games", "Riot Client", "Logs"))
	}
	if pd := sys.Env("ProgramData"); pd != "" {
		roots = append(roots, WinJoin(pd, "Riot Games"))
	}
	if inst.ClientExe != "" {
		roots = append(roots, WinJoin(WinDir(inst.ClientExe), "Logs"))
	}
	return uniq(roots)
}

func (c *LogsCheck) RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result {
	inst := installFrom(sys, deps)
	var files []platform.FileInfo
	seen := map[string]bool{}
	for _, root := range LogRoots(sys, inst) {
		fs, err := sys.FindFiles(root, "*.log", 4)
		if err != nil {
			continue
		}
		for _, f := range fs {
			k := strings.ToLower(f.Path)
			if !seen[k] {
				seen[k] = true
				files = append(files, f)
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	if len(files) > 6 {
		files = files[:6]
	}
	sum := logscan.NewSummary()
	latest := logscan.NewSummary()
	latestPath := ""
	for _, f := range files {
		if strings.Contains(strings.ToLower(f.Path), "riot client logs") {
			latestPath = f.Path
			break
		}
	}
	if latestPath == "" && len(files) > 0 {
		latestPath = files[0].Path
	}
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		b, err := sys.ReadFileTail(f.Path, logscan.MaxBytes)
		if err != nil {
			continue
		}
		sum.Files++
		sum.Scan(b)
		if f.Path == latestPath {
			latest.Scan(b)
		}
	}
	var coreErr, extraErr []string
	for _, h := range core.SortedKeys(latest.Hosts) {
		if probe.IsCoreHost(h) {
			coreErr = append(coreErr, h)
		} else {
			extraErr = append(extraErr, h)
		}
	}
	var r core.Result
	switch {
	case len(files) == 0:
		r = c.result(core.StatusInfo, "logs.none")
	case len(coreErr) > 0:
		r = c.result(core.StatusWarn, "logs.core_errors")
		r.Params["hosts"] = strings.Join(coreErr, ", ")
		r.Steps = []core.Step{core.Fix("F01"), core.Fix("F11"), core.Fix("F02"), core.Fix("F14")}
	case len(extraErr) > 0:
		r = c.result(core.StatusInfo, "logs.extra_errors")
		r.Params["hosts"] = strings.Join(extraErr, ", ")
	case len(sum.Hosts) > 0:
		r = c.result(core.StatusOK, "logs.recovered")
	default:
		r = c.result(core.StatusOK, "logs.clean")
	}
	for _, h := range core.SortedKeys(sum.Hosts) {
		r.AddDetail(core.StatusInfo, "logs.host", map[string]string{"host": h, "count": strconv.Itoa(sum.Hosts[h])})
	}
	sum.Latest = latest.Hosts
	r.Params["files"] = strconv.Itoa(sum.Files)
	r.Params["incidents"] = strconv.Itoa(sum.Incidents)
	r.Data = sum
	return r
}

func (d ServersData) ReportIPs() []string {
	var out []string
	for _, r := range d.Reports {
		out = append(out, r.Pool...)
	}
	return out
}
