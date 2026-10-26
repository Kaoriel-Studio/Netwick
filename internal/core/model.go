package core

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Status int

const (
	StatusOK Status = iota
	StatusInfo
	StatusWarn
	StatusFail
	StatusUnknown
)

var statusNames = map[Status]string{
	StatusOK:      "ok",
	StatusInfo:    "info",
	StatusWarn:    "warn",
	StatusFail:    "fail",
	StatusUnknown: "unknown",
}

func (s Status) String() string {
	if n, ok := statusNames[s]; ok {
		return n
	}
	return "unknown"
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Status) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		return err
	}
	for k, v := range statusNames {
		if v == name {
			*s = k
			return nil
		}
	}
	return fmt.Errorf("unknown status %q", name)
}

func (s Status) Severity() int {
	switch s {
	case StatusFail:
		return 4
	case StatusWarn:
		return 3
	case StatusUnknown:
		return 2
	case StatusInfo:
		return 1
	default:
		return 0
	}
}

func Worst(a, b Status) Status {
	if b.Severity() > a.Severity() {
		return b
	}
	return a
}

const (
	GroupInstall  = "install"
	GroupWindows  = "windows"
	GroupVanguard = "vanguard"
	GroupNetwork  = "network"
	GroupRiot     = "riot"
)

var Groups = []string{GroupInstall, GroupWindows, GroupVanguard, GroupNetwork, GroupRiot}

type Detail struct {
	Status Status            `json:"status"`
	Key    string            `json:"key"`
	Params map[string]string `json:"params"`
}

type Step struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

const (
	StepFix     = "fix"
	StepGuide   = "guide"
	StepCheck   = "check"
	StepInfo    = "info"
	StepArticle = "article"
)

func AppendSteps(dst []Step, src ...Step) []Step {
	for _, s := range src {
		dup := false
		for _, d := range dst {
			if d == s {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, s)
		}
	}
	return dst
}

func Fix(id string) Step   { return Step{Kind: StepFix, ID: id} }
func Guide(id string) Step { return Step{Kind: StepGuide, ID: id} }

type Result struct {
	CheckID    string            `json:"checkId"`
	Group      string            `json:"group"`
	Status     Status            `json:"status"`
	MessageKey string            `json:"messageKey"`
	Params     map[string]string `json:"params"`
	Details    []Detail          `json:"details"`
	FixIDs     []string          `json:"fixIds"`
	GuideIDs   []string          `json:"guideIds"`
	Steps      []Step            `json:"steps"`
	SourceURL  string            `json:"sourceUrl"`
	DurationMs int64             `json:"durationMs"`
	Data       any               `json:"-"`
}

func NewResult(id, group string, status Status, key string) Result {
	return Result{CheckID: id, Group: group, Status: status, MessageKey: key}.Normalized()
}

func (r Result) Normalized() Result {
	if r.Params == nil {
		r.Params = map[string]string{}
	}
	if r.Details == nil {
		r.Details = []Detail{}
	}
	for i := range r.Details {
		if r.Details[i].Params == nil {
			r.Details[i].Params = map[string]string{}
		}
	}
	if r.FixIDs == nil {
		r.FixIDs = []string{}
	}
	if r.GuideIDs == nil {
		r.GuideIDs = []string{}
	}
	if r.Steps == nil {
		r.Steps = []Step{}
	}
	for _, f := range r.FixIDs {
		r.Steps = AppendSteps(r.Steps, Fix(f))
	}
	for _, g := range r.GuideIDs {
		r.Steps = AppendSteps(r.Steps, Guide(g))
	}
	return r
}

func (r *Result) With(k, v string) *Result {
	if r.Params == nil {
		r.Params = map[string]string{}
	}
	r.Params[k] = v
	return r
}

func (r *Result) AddDetail(status Status, key string, params map[string]string) {
	if params == nil {
		params = map[string]string{}
	}
	r.Details = append(r.Details, Detail{Status: status, Key: key, Params: params})
}

func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
