package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/hostsfile"
	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/platform/fake"
	"github.com/kaoriel-studio/netwick/internal/undo"
)

const pipe = "0123456789abcdef0123456789abcdef"

func TestParseArgs(t *testing.T) {
	good := [][]string{
		{"--worker", "--pipe=" + pipe, "--actions=F02"},
		{"--worker", "--pipe=" + pipe, "--actions=F02,F05,F10"},
		{"--worker", "--pipe=" + pipe, "--actions=F04", "--dns-adapter-luid=123"},
		{"--worker", "--pipe=" + pipe, "--actions=UNDO"},
		{"--worker", "--pipe=" + pipe, "--actions=F05"},
		{"--worker", "--pipe=" + pipe, "--actions=F14", "--route-hosts=auth.riotgames.com,valorant.secure.dyn.riotcdn.net"},
		{"--worker", "--pipe=" + pipe, "--actions=F13,F15"},
	}
	for _, g := range good {
		a, err := ParseArgs(g)
		if err != nil {
			t.Errorf("%v: %v", g, err)
			continue
		}
		if strings.Join(a.Build(), " ") != strings.Join(g, " ") {
			t.Errorf("roundtrip %v != %v", a.Build(), g)
		}
	}
	bad := [][]string{
		{},
		{"--pipe=" + pipe, "--actions=F02"},
		{"--worker", "--actions=F02"},
		{"--worker", "--pipe=" + pipe},
		{"--worker", "--pipe=XYZ", "--actions=F02"},
		{"--worker", "--pipe=" + pipe, "--actions=F01"},
		{"--worker", "--pipe=" + pipe, "--actions=F99"},
		{"--worker", "--pipe=" + pipe, "--actions=F02,F02"},
		{"--worker", "--pipe=" + pipe, "--actions=UNDO,F02"},
		{"--worker", "--pipe=" + pipe, "--actions=F04"},
		{"--worker", "--pipe=" + pipe, "--actions=F02", "--dns-adapter-luid=5"},
		{"--worker", "--pipe=" + pipe, "--actions=F04", "--dns-adapter-luid=0"},
		{"--worker", "--pipe=" + pipe, "--actions=F04", "--dns-adapter-luid=-1"},
		{"--worker", "--pipe=" + pipe, "--actions=F02;calc.exe"},
		{"--worker", "--pipe=" + pipe, "--actions=F02", "--path=C:\\Windows"},
		{"--worker", "--pipe=" + pipe, "--actions=F02", "--skip-restore-point"},
		{"--worker", "--pipe=" + pipe, "--pipe=" + pipe, "--actions=F02"},
		{"--worker", "--pipe=" + pipe, "--actions=f02"},
		{"--worker", "--pipe=" + pipe, "--actions=F02", "extra"},
		{"--worker", "--pipe=" + pipe, "--actions=F14"},
		{"--worker", "--pipe=" + pipe, "--actions=F02", "--route-hosts=auth.riotgames.com"},
		{"--worker", "--pipe=" + pipe, "--actions=F14", "--route-hosts=evil.com"},
		{"--worker", "--pipe=" + pipe, "--actions=F14", "--route-hosts=Auth.RiotGames.com"},
		{"--worker", "--pipe=" + pipe, "--actions=F14", "--route-hosts=auth.riotgames.com,auth.riotgames.com"},
		{"--worker", "--pipe=" + pipe, "--actions=F14", "--route-hosts=riotgames.com.evil.com"},
		{"--worker", "--pipe=" + pipe, "--actions=F11"},
		{"--worker", "--pipe=" + pipe, "--actions=F12"},
	}
	for _, b := range bad {
		if _, err := ParseArgs(b); err == nil {
			t.Errorf("accepted %v", b)
		}
	}
}

func FuzzParseArgs(f *testing.F) {
	f.Add("--pipe="+pipe, "--actions=F02,F05", "--dns-adapter-luid=1")
	f.Add("--pipe=../../x", "--actions=UNDO", "")
	f.Fuzz(func(t *testing.T, a, b, c string) {
		args, err := ParseArgs([]string{"--worker", a, b, c})
		if err != nil {
			return
		}
		for _, act := range args.Actions {
			if act != ActionUndo && !privileged(act) {
				t.Fatalf("unknown action accepted: %q", act)
			}
		}
		if len(args.Pipe) != 32 {
			t.Fatalf("bad pipe accepted: %q", args.Pipe)
		}
	})
}

func run(t *testing.T, x *Executor, actions []string, luid uint64, req Request) Response {
	t.Helper()
	a := Args{Pipe: pipe, Actions: actions, DNSAdapterLUID: luid}
	return x.Run(context.Background(), a, req)
}

func TestApplyAndUndo(t *testing.T) {
	sys := fake.New(fake.OldHosts)
	x := &Executor{Sys: sys, Act: sys}
	ads, _ := sys.Adapters()
	luid := checks.ActiveAdapters(ads)[0].LUID
	beforeDNS, _ := sys.GetDNS(luid)
	beforeVGC, _ := sys.Service("vgc")
	beforeHosts, _ := sys.ReadFile(sys.HostsPath(), 0)

	resp := run(t, x, []string{"F04", "F06", "F07", "F08", "F10"}, luid, Request{})
	var entries []undo.Entry
	for _, r := range resp.Results {
		if !r.OK {
			t.Fatalf("%s failed: %s", r.FixID, r.ErrorKey)
		}
		if r.Undo != nil {
			if err := undo.Validate(*r.Undo); err != nil {
				t.Fatalf("%s produced invalid undo: %v", r.FixID, err)
			}
			entries = append(entries, *r.Undo)
		}
	}
	if len(entries) != 5 {
		t.Fatalf("got %d undo entries", len(entries))
	}
	if s, _ := sys.Service("vgc"); s.StartType != platform.StartAutomatic {
		t.Fatal("vgc not automatic")
	}
	h, _ := sys.ReadFile(sys.HostsPath(), 0)
	if hostsfile.Parse(h).HasLegacy() {
		t.Fatal("hosts not cleaned")
	}
	if ok, _ := sys.FirewallRuleExists(undo.RuleName("VALORANT.exe")); !ok {
		t.Fatal("firewall rule missing")
	}

	again := run(t, x, []string{"F06", "F07", "F10"}, 0, Request{})
	for _, r := range again.Results {
		if r.Changed || r.Undo != nil {
			t.Errorf("%s not idempotent", r.FixID)
		}
	}

	u := run(t, x, []string{ActionUndo}, 0, Request{Undo: entries})
	if len(u.UndoneIDs) != 5 {
		t.Fatalf("undone %d: %+v", len(u.UndoneIDs), u.Results)
	}
	if d, _ := sys.GetDNS(luid); d.DHCP != beforeDNS.DHCP || strings.Join(d.Servers, ",") != strings.Join(beforeDNS.Servers, ",") {
		t.Errorf("dns not restored: %+v vs %+v", d, beforeDNS)
	}
	if s, _ := sys.Service("vgc"); s.StartType != beforeVGC.StartType {
		t.Error("vgc not restored")
	}
	if ok, _ := sys.FirewallRuleExists(undo.RuleName("VALORANT.exe")); ok {
		t.Error("own rule not removed")
	}
	after, _ := sys.ReadFile(sys.HostsPath(), 0)
	if len(hostsfile.Parse(after).RiotEntries) != len(hostsfile.Parse(beforeHosts).RiotEntries) {
		t.Error("hosts not restored")
	}
}

func TestUndoRejectsForeign(t *testing.T) {
	sys := fake.New(fake.FirewallBlocks)
	x := &Executor{Sys: sys, Act: sys}
	bad := []undo.Entry{
		{ID: "0123456789abcdef", FixID: "F06", FirewallRules: []string{"Block VALORANT"}},
		{ID: "0123456789abcdef", FixID: "F07", Service: &platform.ServicePrev{Name: "WinDefend", StartType: platform.StartDisabled}},
		{ID: "0123456789abcdef", FixID: "F04", DNS: &platform.DNSState{LUID: 1, Servers: []string{"evil; rm"}}},
		{ID: "0123456789abcdef", FixID: "F10", HostsBackupID: "../../etc/passwd"},
		{ID: "0123456789abcdef", FixID: "F05"},
	}
	u := run(t, x, []string{ActionUndo}, 0, Request{Undo: bad})
	if len(u.UndoneIDs) != 0 {
		t.Fatal("foreign undo executed")
	}
	if rules, _ := sys.FirewallRules(); len(rules) != 1 {
		t.Fatal("foreign firewall rule removed")
	}
}

func TestRestartClient(t *testing.T) {
	sys := fake.New(fake.AllOK)
	if err := RestartClient(sys, sys, 0); err != nil {
		t.Fatal(err)
	}
	procs, _ := sys.Processes()
	found := false
	for _, p := range procs {
		if p.Name == "RiotClientServices.exe" {
			found = true
		}
		if p.Name == "vgtray.exe" {
			continue
		}
	}
	if !found {
		t.Fatal("client not started")
	}
	for _, p := range procs {
		if strings.EqualFold(p.Name, "vgtray.exe") {
			return
		}
	}
	t.Fatal("vanguard tray must never be terminated")
}

func TestVAL43Repair(t *testing.T) {
	sys := fake.New(fake.VAL43)
	x := &Executor{Sys: sys, Act: sys}
	if r := ApplyLocal(sys, sys, "F11", 0); !r.OK || !r.Changed {
		t.Fatalf("F11 %+v", r)
	}
	if _, err := sys.Stat(`C:\Users\Player\AppData\Local\Riot Games\Riot Client\Cache`); err == nil {
		t.Fatal("cache not removed")
	}
	if r := ApplyLocal(sys, sys, "F11", 0); !r.OK || r.Changed {
		t.Fatalf("second F11 %+v", r)
	}
	if r := ApplyLocal(sys, sys, "F12", 0); !r.OK || !r.Changed {
		t.Fatalf("F12 %+v", r)
	}
	if err := sys.RemoveTree(`C:\Windows\System32`); err == nil {
		t.Fatal("RemoveTree outside Riot folders must fail")
	}
	plans := PlanRoutes(context.Background(), sys, []string{"valorant.secure.dyn.riotcdn.net", "auth.riotgames.com"})
	if !plans[0].Needed() || plans[0].IP != "65.9.46.10" || plans[1].Needed() {
		t.Fatalf("plans %+v", plans)
	}
	resp := x.Run(context.Background(), Args{Actions: []string{"F14", "F15", "F13"}, RouteHosts: []string{"valorant.secure.dyn.riotcdn.net", "clientconfig.rpg.riotgames.com"}}, Request{})
	for _, r := range resp.Results {
		if !r.OK || !r.Changed {
			t.Fatalf("%s %+v", r.FixID, r)
		}
	}
	b, _ := sys.ReadFile(sys.HostsPath(), 0)
	p := hostsfile.Parse(b)
	if p.Pinned["valorant.secure.dyn.riotcdn.net"] != "65.9.46.10" || p.Pinned["clientconfig.rpg.riotgames.com"] != "104.18.156.37" {
		t.Fatalf("hosts:\n%s", b)
	}
	if bj, _ := sys.Service(checks.BonjourService); bj.State != platform.StateStopped {
		t.Fatalf("bonjour %+v", bj)
	}
	if err := RestartClient(sys, sys, 0); err != nil {
		t.Fatal(err)
	}
	var entries []undo.Entry
	for _, r := range resp.Results {
		if r.Undo != nil {
			if err := undo.Validate(*r.Undo); err != nil {
				t.Fatalf("%s undo invalid: %v", r.FixID, err)
			}
			entries = append(entries, *r.Undo)
		}
	}
	if len(entries) != 2 {
		t.Fatalf("undo entries %d", len(entries))
	}
	done, _ := x.undoAll(context.Background(), entries)
	if len(done) != 2 {
		t.Fatalf("undone %v", done)
	}
	b, _ = sys.ReadFile(sys.HostsPath(), 0)
	if len(hostsfile.Parse(b).RiotEntries) != 0 {
		t.Fatalf("routes left after undo:\n%s", b)
	}
	if bj, _ := sys.Service(checks.BonjourService); bj.State != platform.StateRunning {
		t.Fatalf("bonjour not restarted %+v", bj)
	}
}
