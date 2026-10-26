package profile

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

func res(statuses map[string]core.Status) map[string]core.Result {
	out := map[string]core.Result{}
	for id, st := range statuses {
		out[id] = core.Result{CheckID: id, Status: st}
	}
	return out
}

func TestScoreAndDiff(t *testing.T) {
	if got := Score(res(map[string]core.Status{"C01": core.StatusOK, "C10": core.StatusFail, "C20": core.StatusWarn, "C03": core.StatusUnknown})); got != 100-22-9-2 {
		t.Fatalf("score %d", got)
	}
	s := Open("")
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.Local)
	s.now = func() time.Time { return now }
	s.RecordRun(res(map[string]core.Status{"C10": core.StatusFail, "C20": core.StatusWarn}), verdict.Verdict{Level: verdict.Red})
	s.now = func() time.Time { return now.Add(time.Hour) }
	s.RecordRun(res(map[string]core.Status{"C10": core.StatusOK, "C12": core.StatusWarn}), verdict.Verdict{Level: verdict.Yellow})
	v := s.View()
	if v.Score != 91 || v.Delta != 91-69 || len(v.Fixed) != 2 || v.Fixed[0] != "C10" || len(v.Added) != 1 || v.Added[0] != "C12" {
		t.Fatalf("view %+v", v)
	}
}

func TestStreakMedalsPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	s := Open(path)
	base := time.Date(2026, 10, 20, 10, 0, 0, 0, time.Local)
	for i := 0; i < 3; i++ {
		s.now = func() time.Time { return base.AddDate(0, 0, i) }
		s.RecordRun(res(map[string]core.Status{"C01": core.StatusOK}), verdict.Verdict{Level: verdict.Green})
	}
	s.now = func() time.Time { return base.AddDate(0, 0, 2).Add(2 * time.Hour) }
	s.Bump(CFixes)
	s.Bump("nonsense")
	v := s.View()
	got := map[string]Medal{}
	for _, m := range v.Medals {
		got[m.ID] = m
	}
	for _, id := range []string{"first_check", "clean_bill", "fixer", "streak3"} {
		if !got[id].Earned || !got[id].Fresh {
			t.Errorf("%s should be fresh and earned: %+v", id, got[id])
		}
	}
	if got["streak7"].Earned || got["veteran"].Earned || v.Streak != 3 {
		t.Fatalf("streak %d, medals %+v", v.Streak, got)
	}
	s.MarkSeen()
	if s2 := Open(path); s2.View().Medals[0].Fresh || len(s2.View().Points) != 3 {
		t.Fatal("state must persist and medals must be marked seen")
	}
	s.Reset()
	if Open(path).View().HasRuns {
		t.Fatal("reset must delete history")
	}
}

func TestOpenRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	if err := writeFile(path, `{"runs":[{"at":"2026-01-01T00:00:00Z","level":"<script>","score":500,"problems":["x"]}],"counters":{"checks":-4,"evil":9},"seen":["../x"]}`); err != nil {
		t.Fatal(err)
	}
	v := Open(path).View()
	if v.HasRuns || len(v.Counters) != 0 {
		t.Fatalf("garbage accepted: %+v", v)
	}
}

func writeFile(path, content string) error { return osWrite(path, []byte(content)) }
