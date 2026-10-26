package profile

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/undo"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

const (
	MaxRuns    = 60
	ViewRuns   = 20
	CounterMax = 1_000_000
)

const (
	CChecks  = "checks"
	CFixes   = "fixes"
	CUndos   = "undos"
	CQuality = "quality"
	CNodes   = "nodes"
	CLaunch  = "launch"
)

var counterNames = []string{CChecks, CFixes, CUndos, CQuality, CNodes, CLaunch}

type Run struct {
	At       time.Time `json:"at"`
	Level    string    `json:"level"`
	Score    int       `json:"score"`
	Problems []string  `json:"problems"`
}

type data struct {
	Runs     []Run          `json:"runs"`
	Counters map[string]int `json:"counters"`
	Seen     []string       `json:"seen"`
}

type Store struct {
	mu   sync.Mutex
	path string
	d    data
	now  func() time.Time
}

func Open(path string) *Store {
	s := &Store{path: path, d: data{Runs: []Run{}, Counters: map[string]int{}, Seen: []string{}}, now: time.Now}
	b, err := os.ReadFile(path)
	if err != nil || len(b) > 1<<20 {
		return s
	}
	var d data
	if json.Unmarshal(b, &d) != nil {
		return s
	}
	for _, r := range d.Runs {
		if validRun(r) {
			s.d.Runs = append(s.d.Runs, r)
		}
	}
	for _, k := range counterNames {
		if v := d.Counters[k]; v > 0 && v <= CounterMax {
			s.d.Counters[k] = v
		}
	}
	for _, id := range d.Seen {
		if knownMedal(id) {
			s.d.Seen = append(s.d.Seen, id)
		}
	}
	s.trim()
	return s
}

func validRun(r Run) bool {
	switch r.Level {
	case "green", "yellow", "red", "unknown":
	default:
		return false
	}
	if r.Score < 0 || r.Score > 100 || r.At.IsZero() || len(r.Problems) > 64 {
		return false
	}
	for _, p := range r.Problems {
		if len(p) != 3 || p[0] != 'C' {
			return false
		}
	}
	return true
}

func (s *Store) trim() {
	if len(s.d.Runs) > MaxRuns {
		s.d.Runs = s.d.Runs[len(s.d.Runs)-MaxRuns:]
	}
}

func (s *Store) saveLocked() {
	if s.path == "" {
		return
	}
	b, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return
	}
	_ = undo.WriteAtomic(s.path, b)
}

func Score(results map[string]core.Result) int {
	score := 100
	for _, r := range results {
		switch r.Status {
		case core.StatusFail:
			score -= 22
		case core.StatusWarn:
			score -= 9
		case core.StatusUnknown:
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func (s *Store) RecordRun(results map[string]core.Result, v verdict.Verdict) {
	run := Run{At: s.now().UTC(), Level: string(v.Level), Score: Score(results), Problems: []string{}}
	for _, id := range core.SortedKeys(results) {
		if st := results[id].Status; st == core.StatusFail || st == core.StatusWarn {
			run.Problems = append(run.Problems, id)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.Runs = append(s.d.Runs, run)
	s.trim()
	s.bumpLocked(CChecks)
	s.saveLocked()
}

func (s *Store) bumpLocked(name string) {
	if s.d.Counters[name] < CounterMax {
		s.d.Counters[name]++
	}
}

func (s *Store) Bump(name string) {
	ok := false
	for _, n := range counterNames {
		if n == name {
			ok = true
		}
	}
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bumpLocked(name)
	s.saveLocked()
}

func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d = data{Runs: []Run{}, Counters: map[string]int{}, Seen: []string{}}
	if s.path != "" {
		_ = os.Remove(s.path)
	}
}

type Medal struct {
	ID     string `json:"id"`
	Earned bool   `json:"earned"`
	Fresh  bool   `json:"fresh"`
}

var medalIDs = []string{"first_check", "clean_bill", "fixer", "safe_hands", "scout", "tester", "streak3", "streak7", "veteran"}

func knownMedal(id string) bool {
	for _, m := range medalIDs {
		if m == id {
			return true
		}
	}
	return false
}

type Point struct {
	At    time.Time `json:"at"`
	Score int       `json:"score"`
	Level string    `json:"level"`
}

type View struct {
	HasRuns  bool           `json:"hasRuns"`
	Score    int            `json:"score"`
	Delta    int            `json:"delta"`
	Level    string         `json:"level"`
	LastAt   time.Time      `json:"lastAt"`
	Streak   int            `json:"streak"`
	Points   []Point        `json:"points"`
	Fixed    []string       `json:"fixed"`
	Added    []string       `json:"added"`
	Counters map[string]int `json:"counters"`
	Medals   []Medal        `json:"medals"`
}

func day(t time.Time) time.Time {
	y, m, d := t.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func streak(runs []Run, now time.Time) int {
	days := map[time.Time]bool{}
	for _, r := range runs {
		days[day(r.At)] = true
	}
	n := 0
	cur := day(now)
	if !days[cur] {
		cur = cur.AddDate(0, 0, -1)
	}
	for days[cur] {
		n++
		cur = cur.AddDate(0, 0, -1)
	}
	return n
}

func diff(prev, cur []string) (fixed, added []string) {
	in := func(l []string, v string) bool {
		for _, x := range l {
			if x == v {
				return true
			}
		}
		return false
	}
	fixed, added = []string{}, []string{}
	for _, p := range prev {
		if !in(cur, p) {
			fixed = append(fixed, p)
		}
	}
	for _, c := range cur {
		if !in(prev, c) {
			added = append(added, c)
		}
	}
	return
}

func (s *Store) View() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	v := View{Points: []Point{}, Fixed: []string{}, Added: []string{}, Counters: map[string]int{}, Medals: []Medal{}}
	for k, n := range s.d.Counters {
		v.Counters[k] = n
	}
	runs := s.d.Runs
	greens := 0
	for _, r := range runs {
		if r.Level == "green" {
			greens++
		}
	}
	if n := len(runs); n > 0 {
		last := runs[n-1]
		v.HasRuns, v.Score, v.Level, v.LastAt = true, last.Score, last.Level, last.At
		if n > 1 {
			v.Delta = last.Score - runs[n-2].Score
			v.Fixed, v.Added = diff(runs[n-2].Problems, last.Problems)
		}
		from := 0
		if n > ViewRuns {
			from = n - ViewRuns
		}
		for _, r := range runs[from:] {
			v.Points = append(v.Points, Point{At: r.At, Score: r.Score, Level: r.Level})
		}
	}
	v.Streak = streak(runs, now)
	c := s.d.Counters
	earned := map[string]bool{
		"first_check": c[CChecks] >= 1,
		"clean_bill":  greens >= 1,
		"fixer":       c[CFixes] >= 1,
		"safe_hands":  c[CUndos] >= 1,
		"scout":       c[CNodes] >= 1,
		"tester":      c[CQuality] >= 1,
		"streak3":     v.Streak >= 3,
		"streak7":     v.Streak >= 7,
		"veteran":     c[CChecks] >= 25,
	}
	seen := map[string]bool{}
	for _, id := range s.d.Seen {
		seen[id] = true
	}
	for _, id := range medalIDs {
		v.Medals = append(v.Medals, Medal{ID: id, Earned: earned[id], Fresh: earned[id] && !seen[id]})
	}
	return v
}

func (s *Store) MarkSeen() {
	v := s.View()
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, id := range s.d.Seen {
		seen[id] = true
	}
	for _, m := range v.Medals {
		if m.Earned {
			seen[m.ID] = true
		}
	}
	s.d.Seen = s.d.Seen[:0]
	for id := range seen {
		s.d.Seen = append(s.d.Seen, id)
	}
	sort.Strings(s.d.Seen)
	s.saveLocked()
}
