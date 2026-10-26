package verdict_test

import (
	"context"
	"testing"

	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/platform/fake"
	"github.com/kaoriel-studio/netwick/internal/probe"
	"github.com/kaoriel-studio/netwick/internal/verdict"
	"github.com/kaoriel-studio/netwick/internal/worker"
)

func run(t *testing.T, scenario string) (map[string]core.Result, verdict.Verdict) {
	t.Helper()
	sys := fake.New(scenario)
	res := checks.NewRunner(checks.All()).Run(context.Background(), sys)
	return res, verdict.Overall(res, "")
}

func hasDetail(r core.Result, key string) bool {
	for _, d := range r.Details {
		if d.Key == key {
			return true
		}
	}
	return false
}

func TestOwnerRealRun(t *testing.T) {
	res, v := run(t, fake.OwnerRealRun)
	if v.Level != verdict.Green {
		for _, p := range v.Problems {
			t.Logf("problem %s %s %s", p.CheckID, p.Status, p.MessageKey)
		}
		t.Fatalf("level %s, want green", v.Level)
	}
	for id, r := range res {
		if r.Status == core.StatusFail {
			t.Errorf("%s is red: %s", id, r.MessageKey)
		}
	}
	c10 := res["C10"]
	if c10.Status != core.StatusInfo || c10.MessageKey != "servers.mixed" || len(c10.FixIDs) == 0 || c10.FixIDs[0] != "F14" {
		t.Fatalf("C10 = %s %s %v", c10.Status, c10.MessageKey, c10.FixIDs)
	}
	if !hasDetail(c10, "servers.mixed") {
		t.Error("expected info about individual addresses")
	}
	if !hasDetail(c10, "servers.extra_down") {
		t.Error("expected info about data.riotgames.com")
	}
	if c19 := res["C19"]; c19.Status != core.StatusInfo || len(c19.FixIDs) == 0 || c19.FixIDs[0] != "F10" {
		t.Errorf("C19 = %s %v", c19.Status, c19.FixIDs)
	}
	if c09 := res["C09"]; c09.MessageKey != "vanguard.normal_off" || c09.Status != core.StatusOK {
		t.Errorf("C09 = %s %s", c09.Status, c09.MessageKey)
	}
	if c03 := res["C03"]; c03.Status != core.StatusOK {
		t.Errorf("C03 = %s", c03.Status)
	}
	if len(v.Recommendations) == 0 {
		t.Error("expected F10 recommendation")
	}
}

func TestVAL43Steps(t *testing.T) {
	res, v := run(t, fake.VAL43)
	if res["C10"].MessageKey != "servers.client_errors" || res["C20"].MessageKey != "logs.core_errors" {
		t.Fatalf("C10 = %s, C20 = %s", res["C10"].MessageKey, res["C20"].MessageKey)
	}
	if res["C23"].MessageKey != "bonjour.running" {
		t.Errorf("C23 = %s", res["C23"].MessageKey)
	}
	want := map[string]bool{"F14": false, "F01": false, "F11": false}
	for _, s := range v.Steps {
		if _, ok := want[s.ID]; ok {
			want[s.ID] = true
		}
	}
	for id, ok := range want {
		if !ok {
			t.Errorf("verdict steps %v miss %s", v.Steps, id)
		}
	}
}

func TestRiotIncident(t *testing.T) {
	res, v := run(t, fake.RiotIncident)
	r := res["C22"]
	if r.Status != core.StatusWarn || r.MessageKey != "riotstatus.incident" || r.Params["title_ru"] == "" {
		t.Fatalf("C22 = %s %s %v", r.Status, r.MessageKey, r.Params)
	}
	if v.Problems[0].CheckID != "C22" {
		t.Errorf("top problem %s", v.Problems[0].CheckID)
	}
}

func TestScenarioLevels(t *testing.T) {
	cases := map[string]verdict.Level{
		fake.AllOK:          verdict.Green,
		fake.NoInternet:     verdict.Red,
		fake.ISPCutsSomeIPs: verdict.Red,
		fake.TPMOffWin11:    verdict.Red,
		fake.SecureBootOff:  verdict.Red,
		fake.VGCDisabled:    verdict.Red,
		fake.FirewallBlocks: verdict.Red,
		fake.VPNOn:          verdict.Yellow,
		fake.VAN68:          verdict.Red,
		fake.VAN128:         verdict.Yellow,
		fake.OldHosts:       verdict.Red,
		fake.VAL43:          verdict.Yellow,
		fake.RiotIncident:   verdict.Yellow,
		fake.LaptopSaver:    verdict.Green,
		fake.WifiNoise:      verdict.Green,
	}
	for sc, want := range cases {
		t.Run(sc, func(t *testing.T) {
			_, v := run(t, sc)
			if v.Level != want {
				for _, p := range v.Problems {
					t.Logf("problem %s %s %s", p.CheckID, p.Status, p.MessageKey)
				}
				t.Fatalf("level %s, want %s", v.Level, want)
			}
		})
	}
}

func TestVAL43HealsAfterRoutesAndRestart(t *testing.T) {
	sys := fake.New(fake.VAL43)
	x := &worker.Executor{Sys: sys, Act: sys}
	resp := x.Run(context.Background(), worker.Args{Actions: []string{"F14"}, RouteHosts: probe.CoreHosts}, worker.Request{})
	if !resp.Results[0].OK || !resp.Results[0].Changed {
		t.Fatalf("F14 %+v", resp.Results[0])
	}
	if err := worker.RestartClient(sys, sys, 0); err != nil {
		t.Fatal(err)
	}
	res := checks.NewRunner(checks.All()).Run(context.Background(), sys)
	v := verdict.Overall(res, "")
	if v.Level != verdict.Green {
		for _, p := range v.Problems {
			t.Logf("problem %s %s %s", p.CheckID, p.Status, p.MessageKey)
		}
		t.Fatalf("level %s", v.Level)
	}
	if !hasDetail(res["C10"], "servers.routed") || res["C19"].MessageKey != "hosts.managed" {
		t.Fatalf("C10 %v, C19 %s", res["C10"].Details, res["C19"].MessageKey)
	}
}

func TestLaptopSaverPowerHints(t *testing.T) {
	res, _ := run(t, fake.LaptopSaver)
	r := res["C24"]
	if r.Status != core.StatusInfo || r.MessageKey != "power.issues" || len(r.GuideIDs) != 1 || r.GuideIDs[0] != "G15" || len(r.Details) != 3 {
		t.Fatalf("C24 = %s %s %v %d", r.Status, r.MessageKey, r.GuideIDs, len(r.Details))
	}
	if ok, _ := run(t, fake.AllOK); ok["C24"].MessageKey != "power.ok" {
		t.Fatalf("all-ok C24 = %s", ok["C24"].MessageKey)
	}
}
