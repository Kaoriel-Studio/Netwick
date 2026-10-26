package verdict

import (
	"sort"

	"github.com/kaoriel-studio/netwick/internal/core"
)

type Level string

const (
	Green   Level = "green"
	Yellow  Level = "yellow"
	Red     Level = "red"
	Unknown Level = "unknown"
)

const (
	FocusNone       = ""
	FocusWontLaunch = "wont_launch"
	FocusWontUpdate = "wont_update"
	FocusCrashes    = "crashes"
	FocusHighPing   = "high_ping"
)

var focusGroups = map[string][]string{
	FocusWontLaunch: {core.GroupInstall, core.GroupVanguard, core.GroupWindows},
	FocusWontUpdate: {core.GroupRiot, core.GroupNetwork, core.GroupInstall},
	FocusCrashes:    {core.GroupVanguard, core.GroupWindows},
	FocusHighPing:   {core.GroupNetwork, core.GroupRiot},
}

func ValidFocus(f string) bool {
	if f == FocusNone {
		return true
	}
	_, ok := focusGroups[f]
	return ok
}

func FocusGroups(f string) []string {
	return append([]string{}, focusGroups[f]...)
}

type Verdict struct {
	Level           Level             `json:"level"`
	MessageKey      string            `json:"messageKey"`
	Params          map[string]string `json:"params"`
	PrimaryAction   string            `json:"primaryAction"`
	Problems        []core.Result     `json:"problems"`
	Recommendations []core.Result     `json:"recommendations"`
	Steps           []core.Step       `json:"steps"`
	Total           int               `json:"total"`
	Unknown         int               `json:"unknown"`
}

var checkOrder = map[string]int{}

func init() {
	order := []string{"C01", "C17", "C02", "C22", "C10", "C12", "C11", "C03", "C09", "C04", "C05", "C19", "C18", "C21", "C20", "C06", "C07", "C08", "C13", "C14", "C15", "C16", "C23"}
	for i, id := range order {
		checkOrder[id] = i
	}
}

func rank(r core.Result, focus string) int {
	base := checkOrder[r.CheckID]
	for i, g := range focusGroups[focus] {
		if r.Group == g {
			return i*100 + base - 1000
		}
	}
	return base
}

func Overall(results map[string]core.Result, focus string) Verdict {
	v := Verdict{Level: Green, MessageKey: "verdict.green", PrimaryAction: "launch", Params: map[string]string{}, Problems: []core.Result{}, Recommendations: []core.Result{}, Steps: []core.Step{}}
	all := make([]core.Result, 0, len(results))
	for _, r := range results {
		all = append(all, r)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Status.Severity() != all[j].Status.Severity() {
			return all[i].Status.Severity() > all[j].Status.Severity()
		}
		return rank(all[i], focus) < rank(all[j], focus)
	})
	v.Total = len(all)
	worst := core.StatusOK
	for _, r := range all {
		switch r.Status {
		case core.StatusFail, core.StatusWarn:
			v.Problems = append(v.Problems, r)
			worst = core.Worst(worst, r.Status)
		case core.StatusInfo:
			if len(r.FixIDs) > 0 {
				v.Recommendations = append(v.Recommendations, r)
			}
		case core.StatusUnknown:
			v.Unknown++
		}
	}
	for _, r := range v.Problems {
		v.Steps = core.AppendSteps(v.Steps, r.Steps...)
	}
	switch worst {
	case core.StatusFail:
		v.Level = Red
		v.MessageKey = "verdict.red"
		v.PrimaryAction = "fix"
	case core.StatusWarn:
		v.Level = Yellow
		v.MessageKey = "verdict.yellow"
		v.PrimaryAction = "show"
	}
	if len(v.Problems) > 0 {
		v.Params["top"] = v.Problems[0].MessageKey
	}
	if v.Level == Green {
		if c10, ok := results["C10"]; ok && c10.Status == core.StatusUnknown {
			v.MessageKey = "verdict.green_no_servers"
		}
	}
	if len(v.Steps) == 0 && v.Level != Green {
		v.PrimaryAction = "show"
	}
	return v
}
