package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/fixes"
	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/undo"
	"github.com/kaoriel-studio/netwick/internal/worker"
)

const (
	Name       = "Netwick"
	Repository = "https://github.com/Kaoriel-Studio/Netwick"
	RepoPath   = "/Kaoriel-Studio/Netwick"
)

var Version = "2.0.0-dev"

func UserAgent() string {
	return Name + "/" + Version + " (+" + Repository + ")"
}

type Runner interface {
	Run(ctx context.Context, a worker.Args, req worker.Request) (worker.Response, error)
}

var ErrDeclined = errors.New("elevation declined")

type Env struct {
	Sys        platform.System
	Act        platform.Actions
	Runner     Runner
	DataDir    string
	ConfigDir  string
	Demo       bool
	Scenario   string
	OpenURL    func(string) error
	OpenSystem func(fixes.Opener) error
	Simulate   func(string)
}

var allowedHosts = map[string]bool{
	"support.riotgames.com":   true,
	"www.riotgames.com":       true,
	"status.riotgames.com":    true,
	"support.microsoft.com":   true,
	"learn.microsoft.com":     true,
	"developer.microsoft.com": true,
}

func AllowedURL(raw string) bool {
	if len(raw) > 512 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if allowedHosts[host] {
		return true
	}
	if host == "github.com" {
		p := strings.ToLower(u.EscapedPath())
		rp := strings.ToLower(RepoPath)
		return p == rp || strings.HasPrefix(p, rp+"/")
	}
	return false
}

type Settings struct {
	Language     string `json:"language"`
	Theme        string `json:"theme"`
	RestorePoint bool   `json:"restorePoint"`
	UpdateCheck  bool   `json:"updateCheck"`
	Tray         bool   `json:"tray"`
	WatchClient  bool   `json:"watchClient"`
}

func DefaultSettings() Settings {
	return Settings{Language: "", Theme: "system", RestorePoint: true}
}

func (s Settings) Valid() bool {
	switch s.Language {
	case "", "ru", "en":
	default:
		return false
	}
	switch s.Theme {
	case "system", "light", "dark":
	default:
		return false
	}
	return true
}

type Store struct {
	mu       sync.Mutex
	env      *Env
	settings Settings
	Journal  *undo.Journal
}

func NewStore(env *Env) *Store {
	s := &Store{env: env, settings: DefaultSettings()}
	if b, err := os.ReadFile(s.settingsPath()); err == nil {
		var st Settings
		if json.Unmarshal(b, &st) == nil && st.Valid() {
			s.settings = st
		}
	}
	j, _ := undo.Open(filepath.Join(env.DataDir, "undo.json"))
	s.Journal = j
	return s
}

func (s *Store) settingsPath() string { return filepath.Join(s.env.ConfigDir, "settings.json") }

func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Store) SaveSettings(st Settings) error {
	if !st.Valid() {
		return errors.New("invalid settings")
	}
	s.mu.Lock()
	s.settings = st
	s.mu.Unlock()
	b, _ := json.MarshalIndent(st, "", "  ")
	return undo.WriteAtomic(s.settingsPath(), b)
}

func SourceFor(step core.Step) string {
	switch step.Kind {
	case core.StepFix:
		if f, ok := fixes.Get(step.ID); ok {
			return f.SourceURL
		}
	case core.StepGuide:
		if g, ok := fixes.GetGuide(step.ID); ok {
			return g.SourceURL
		}
	case core.StepInfo:
		return fixes.InfoSteps[step.ID]
	}
	return ""
}
