package verdict_test

import (
	"context"
	"testing"

	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/platform/fake"
	"github.com/kaoriel-studio/netwick/internal/verdict"
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
	if c10.Status != core.StatusOK || c10.MessageKey != "servers.ok" {
		t.Fatalf("C10 = %s %s", c10.Status, c10.MessageKey)
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
