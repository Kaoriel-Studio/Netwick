package services

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/kaoriel-studio/netwick/internal/app"
	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/profile"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

const EventProgress = "check:progress"

type Emitter func(name string, data any)

type State struct {
	mu      sync.Mutex
	nodesMu sync.Mutex
	fixMu   sync.Mutex
	qualMu  sync.Mutex
	Prof    *profile.Store
	Env     *app.Env
	Store   *app.Store
	Emit    Emitter
	results map[string]core.Result
	verdict verdict.Verdict
	ranAt   time.Time
	running bool
	cancel  context.CancelFunc
	history []FixRecord
}

type FixRecord struct {
	FixID  string    `json:"fixId"`
	At     time.Time `json:"at"`
	OK     bool      `json:"ok"`
	Undone bool      `json:"undone"`
}

func NewState(env *app.Env, store *app.Store) *State {
	profilePath := ""
	if env.DataDir != "" {
		profilePath = filepath.Join(env.DataDir, "profile.json")
	}
	return &State{Env: env, Store: store, Prof: profile.Open(profilePath), results: map[string]core.Result{}, Emit: func(string, any) {}}
}

func (s *State) Results() map[string]core.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]core.Result, len(s.results))
	for k, v := range s.results {
		out[k] = v
	}
	return out
}

func (s *State) addHistory(r FixRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, r)
}

func (s *State) History() []FixRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]FixRecord{}, s.history...)
}

type GroupStatus struct {
	Group  string      `json:"group"`
	Status core.Status `json:"status"`
	Count  int         `json:"count"`
}

type RunSummary struct {
	Mode       string          `json:"mode"`
	Focus      string          `json:"focus"`
	Verdict    verdict.Verdict `json:"verdict"`
	Results    []core.Result   `json:"results"`
	Groups     []GroupStatus   `json:"groups"`
	DurationMs int64           `json:"durationMs"`
	RanAt      time.Time       `json:"ranAt"`
	Cancelled  bool            `json:"cancelled"`
}

type CheckService struct{ st *State }

func NewCheckService(st *State) *CheckService { return &CheckService{st: st} }

var ErrBusy = errors.New("check already running")
var ErrInvalid = errors.New("invalid input")

const (
	ModeFull  = "full"
	ModeQuick = "quick"
)

func (c *CheckService) Run(mode string, focus string) (*RunSummary, error) {
	if mode != ModeFull && mode != ModeQuick {
		return nil, ErrInvalid
	}
	if !verdict.ValidFocus(focus) {
		return nil, ErrInvalid
	}
	return c.st.run(mode, focus, nil)
}

func (c *CheckService) RunOnly(ids []string) (*RunSummary, error) {
	if len(ids) == 0 || len(ids) > len(checks.All()) {
		return nil, ErrInvalid
	}
	for _, id := range ids {
		if checks.ByID(id) == nil {
			return nil, ErrInvalid
		}
	}
	return c.st.run("partial", "", ids)
}

func (c *CheckService) Cancel() bool {
	c.st.mu.Lock()
	defer c.st.mu.Unlock()
	if c.st.cancel != nil {
		c.st.cancel()
		return true
	}
	return false
}

func (c *CheckService) Last() *RunSummary {
	c.st.mu.Lock()
	defer c.st.mu.Unlock()
	if c.st.ranAt.IsZero() {
		return nil
	}
	return summarize("last", "", c.st.results, c.st.verdict, 0, c.st.ranAt, false)
}

func (s *State) run(mode, focus string, only []string) (*RunSummary, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	s.running = true
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()
	}()

	var list []checks.Check
	switch {
	case only != nil:
		list = checks.WithDeps(only)
	case mode == ModeQuick:
		list = checks.Quick()
	default:
		list = checks.All()
	}
	runner := checks.NewRunner(list)
	runner.OnProgress = func(ev checks.ProgressEvent) { s.Emit(EventProgress, ev) }
	start := time.Now()
	res := runner.Run(ctx, s.Env.Sys)
	cancelled := ctx.Err() == context.Canceled

	s.mu.Lock()
	if only != nil || mode == ModeQuick {
		for k, v := range res {
			s.results[k] = v
		}
	} else {
		s.results = res
	}
	merged := make(map[string]core.Result, len(s.results))
	for k, v := range s.results {
		merged[k] = v
	}
	s.verdict = verdict.Overall(merged, focus)
	s.ranAt = time.Now()
	v := s.verdict
	s.mu.Unlock()
	if only == nil && mode == ModeFull && !cancelled {
		s.Prof.RecordRun(merged, s.verdictOf(merged, focus))
	}

	shown := res
	if only == nil && mode == ModeFull {
		shown = merged
	}
	if mode == ModeQuick {
		v = verdict.Overall(res, focus)
	}
	return summarize(mode, focus, shown, v, time.Since(start).Milliseconds(), time.Now(), cancelled), nil
}

func summarize(mode, focus string, res map[string]core.Result, v verdict.Verdict, dur int64, at time.Time, cancelled bool) *RunSummary {
	sum := &RunSummary{Mode: mode, Focus: focus, Verdict: v, DurationMs: dur, RanAt: at, Cancelled: cancelled, Results: []core.Result{}, Groups: []GroupStatus{}}
	for _, id := range core.SortedKeys(res) {
		sum.Results = append(sum.Results, res[id].Normalized())
	}
	sort.SliceStable(sum.Results, func(i, j int) bool { return sum.Results[i].CheckID < sum.Results[j].CheckID })
	for _, g := range core.Groups {
		gs := GroupStatus{Group: g, Status: core.StatusOK}
		for _, r := range sum.Results {
			if r.Group == g {
				gs.Count++
				if r.Status != core.StatusInfo {
					gs.Status = core.Worst(gs.Status, r.Status)
				}
			}
		}
		if gs.Count > 0 {
			sum.Groups = append(sum.Groups, gs)
		}
	}
	return sum
}

func (s *State) verdictOf(res map[string]core.Result, focus string) verdict.Verdict {
	return verdict.Overall(res, focus)
}
