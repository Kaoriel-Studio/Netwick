package checks_test

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/fixes"
	"github.com/kaoriel-studio/netwick/internal/kb"
	"github.com/kaoriel-studio/netwick/internal/platform/fake"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

func load(t *testing.T, lang string) map[string]string {
	t.Helper()
	b, err := os.ReadFile("../../frontend/src/i18n/" + lang + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func stepKeys(s core.Step) []string {
	switch s.Kind {
	case core.StepFix:
		return []string{"fix." + s.ID + ".title", "fix." + s.ID + ".what", "fix." + s.ID + ".why", "fix." + s.ID + ".know"}
	case core.StepGuide:
		return []string{"guide." + s.ID + ".title", "guide." + s.ID + ".intro"}
	case core.StepInfo:
		return []string{"info." + s.ID + ".title", "info." + s.ID + ".text"}
	case core.StepCheck:
		return []string{"check." + s.ID}
	}
	return nil
}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

func TestAllKeysTranslated(t *testing.T) {
	need := map[string]bool{"verdict.green": true, "verdict.yellow": true, "verdict.red": true, "verdict.green_no_servers": true, "check.timeout": true, "check.crashed": true, "check.unknown": true}
	for _, sc := range fake.Scenarios {
		res := checks.NewRunner(checks.All()).Run(context.Background(), fake.New(sc))
		v := verdict.Overall(res, "")
		need[v.MessageKey] = true
		for _, r := range res {
			need[r.MessageKey] = true
			need["check."+r.CheckID] = true
			for _, d := range r.Details {
				need[d.Key] = true
			}
			for _, s := range r.Steps {
				for _, k := range stepKeys(s) {
					need[k] = true
				}
			}
		}
	}
	for _, id := range fixes.IDs() {
		for _, k := range stepKeys(core.Fix(id)) {
			need[k] = true
		}
	}
	for id, g := range fixes.Guides {
		for _, k := range stepKeys(core.Guide(id)) {
			need[k] = true
		}
		for i := 1; i <= g.Steps; i++ {
			need["guide."+id+".s"+string(rune('0'+i))] = true
		}
		if g.Warn {
			need["guide."+id+".warn"] = true
		}
		for _, o := range g.Opens {
			need["open."+string(o)] = true
		}
	}
	for id := range fixes.InfoSteps {
		for _, k := range stepKeys(core.Step{Kind: core.StepInfo, ID: id}) {
			need[k] = true
		}
	}
	db, _ := kb.Default()
	for _, e := range db.Entries {
		for _, s := range e.Steps {
			for _, k := range stepKeys(s) {
				need[k] = true
			}
		}
	}
	en, ru := load(t, "en"), load(t, "ru")
	for k := range need {
		if _, ok := en[k]; !ok {
			t.Errorf("en.json missing %q", k)
		}
		if _, ok := ru[k]; !ok {
			t.Errorf("ru.json missing %q", k)
		}
	}
	for k, v := range en {
		a := placeholder.FindAllString(v, -1)
		b := placeholder.FindAllString(ru[k], -1)
		if len(a) != len(b) {
			t.Errorf("%s: placeholders differ en=%v ru=%v", k, a, b)
		}
	}
}
