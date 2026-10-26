package services

import (
	"testing"
	"time"

	"github.com/kaoriel-studio/netwick/internal/app"
	"github.com/kaoriel-studio/netwick/internal/platform/fake"
	"github.com/kaoriel-studio/netwick/internal/probe"
	"github.com/kaoriel-studio/netwick/internal/worker"
)

func newTestState(t *testing.T, scenario string) *State {
	t.Helper()
	sys := fake.New(scenario)
	dir := t.TempDir()
	env := &app.Env{Sys: sys, Act: sys, Runner: LocalRunner{Exec: &worker.Executor{Sys: sys, Act: sys}}, DataDir: dir, ConfigDir: dir, Demo: true, Scenario: scenario}
	return NewState(env, app.NewStore(env))
}

func TestQualityScenarios(t *testing.T) {
	old := qualityInterval
	qualityInterval = time.Millisecond
	defer func() { qualityInterval = old }()
	host := probe.CoreHosts[0]

	good, err := NewToolsService(newTestState(t, fake.AllOK)).Quality(host)
	if err != nil || good.Loss != 0 || len(good.Samples) != qualitySamples || good.Avg < 20 || good.Avg > 120 || good.Min > good.Avg || good.Max < good.Avg {
		t.Fatalf("all-ok %+v %v", good, err)
	}
	down, err := NewToolsService(newTestState(t, fake.NoInternet)).Quality(host)
	if err != nil || down.Loss != 100 || len(down.Samples) != 0 {
		t.Fatalf("no-internet %+v %v", down, err)
	}
	noisy, err := NewToolsService(newTestState(t, fake.WifiNoise)).Quality(host)
	if err != nil || noisy.Jitter <= good.Jitter || noisy.Loss == 0 {
		t.Fatalf("wifi-noise %+v vs %+v", noisy, good)
	}
	if _, err := NewToolsService(newTestState(t, fake.AllOK)).Quality("evil.example.com"); err == nil {
		t.Fatal("host outside Riot core hosts must be refused")
	}
}

func TestProfileRecordsFullRunsOnly(t *testing.T) {
	st := newTestState(t, fake.VAL43)
	cs := NewCheckService(st)
	if _, err := cs.Run(ModeQuick, ""); err != nil {
		t.Fatal(err)
	}
	if st.Prof.View().HasRuns {
		t.Fatal("quick runs must not enter the history")
	}
	if _, err := cs.Run(ModeFull, ""); err != nil {
		t.Fatal(err)
	}
	v := st.Prof.View()
	if !v.HasRuns || v.Score >= 100 || v.Counters["checks"] != 1 {
		t.Fatalf("view %+v", v)
	}
}
