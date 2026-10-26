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
		{"--worker", "--pipe=" + pipe, "--actions=F05", "--skip-restore-point"},
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
		{"--worker", "--pipe=" + pipe, "--actions=F02", "--skip-restore-point=yes"},
		{"--worker", "--pipe=" + pipe, "--pipe=" + pipe, "--actions=F02"},
		{"--worker", "--pipe=" + pipe, "--actions=f02"},
		{"--worker", "--pipe=" + pipe, "--actions=F02", "extra"},
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
	if resp.RestorePoint != RPCreated {
		t.Fatalf("restore point %s", resp.RestorePoint)
	}
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

func TestRestorePointFailureStops(t *testing.T) {
	sys := fake.New(fake.AllOK)
	x := &Executor{Sys: sys, Act: &noRP{sys}}
	resp := run(t, x, []string{"F07"}, 0, Request{})
	if resp.RestorePoint != RPFailed || len(resp.Results) != 0 {
		t.Fatalf("%+v", resp)
	}
	if s, _ := sys.Service("vgc"); s.StartType != platform.StartManual {
		t.Fatal("changed without restore point")
	}
}

type noRP struct{ *fake.System }

func (n *noRP) CreateRestorePoint(string) error { return platform.ErrUnavailable }

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
