package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kaoriel-studio/netwick/internal/app"
	"github.com/kaoriel-studio/netwick/internal/appcfg"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/kb"
	"github.com/kaoriel-studio/netwick/internal/report"
	"github.com/kaoriel-studio/netwick/internal/undo"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

type AppInfo struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Repository string   `json:"repository"`
	Demo       bool     `json:"demo"`
	Scenario   string   `json:"scenario"`
	Scenarios  []string `json:"scenarios"`
	Platform   string   `json:"platform"`
	License    string   `json:"license"`
}

type AppService struct {
	st        *State
	Scenarios []string
	SetTray   func(bool)
	SetWatch  func(bool)
}

func NewAppService(st *State) *AppService { return &AppService{st: st} }

func (a *AppService) Info() AppInfo {
	return AppInfo{
		Name:       app.Name,
		Version:    app.Version,
		Repository: app.Repository,
		Demo:       a.st.Env.Demo,
		Scenario:   a.st.Env.Scenario,
		Scenarios:  append([]string{}, a.Scenarios...),
		Platform:   runtime.GOOS,
		License:    "MIT",
	}
}

func (a *AppService) OpenURL(raw string) error {
	if !app.AllowedURL(raw) {
		return ErrInvalid
	}
	if a.st.Env.OpenURL == nil {
		return errors.New("unavailable")
	}
	return a.st.Env.OpenURL(raw)
}

func (a *AppService) Settings() app.Settings { return a.st.Store.Settings() }

func (a *AppService) SaveSettings(s app.Settings) (app.Settings, error) {
	if err := a.st.Store.SaveSettings(s); err != nil {
		return a.st.Store.Settings(), ErrInvalid
	}
	if a.SetTray != nil {
		a.SetTray(s.Tray)
	}
	if a.SetWatch != nil {
		a.SetWatch(s.WatchClient)
	}
	return s, nil
}

type UpdateInfo struct {
	Checked  bool   `json:"checked"`
	Newer    bool   `json:"newer"`
	Latest   string `json:"latest"`
	URL      string `json:"url"`
	ErrorKey string `json:"errorKey"`
}

var tagRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

func (a *AppService) CheckUpdate() UpdateInfo {
	if !a.st.Store.Settings().UpdateCheck {
		return UpdateInfo{ErrorKey: "update.disabled"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos"+app.RepoPath+"/releases/latest", nil)
	req.Header.Set("User-Agent", app.UserAgent())
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UpdateInfo{ErrorKey: "update.failed"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return UpdateInfo{ErrorKey: "update.failed"}
	}
	var rel struct {
		Tag string `json:"tag_name"`
		URL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&rel); err != nil || !tagRe.MatchString(rel.Tag) || !app.AllowedURL(rel.URL) {
		return UpdateInfo{ErrorKey: "update.failed"}
	}
	cur := strings.SplitN(app.Version, "-", 2)[0]
	newer := false
	if m := tagRe.FindStringSubmatch(rel.Tag); m != nil {
		newer = compareSemver(strings.TrimPrefix(rel.Tag, "v"), cur) > 0
	}
	return UpdateInfo{Checked: true, Newer: newer, Latest: rel.Tag, URL: rel.URL}
}

func compareSemver(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			x = atoi(pa[i])
		}
		if i < len(pb) {
			y = atoi(pb[i])
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

type KBService struct{}

func (KBService) Search(q string) []kb.Match {
	if utf8.RuneCountInString(q) > kb.MaxQuery {
		return []kb.Match{}
	}
	db, err := kb.Default()
	if err != nil {
		return []kb.Match{}
	}
	return db.Search(q)
}

func (KBService) Get(code string) (*kb.Entry, error) {
	if len(code) > kb.MaxQuery {
		return nil, ErrInvalid
	}
	db, err := kb.Default()
	if err != nil {
		return nil, err
	}
	e, ok := db.Get(code)
	if !ok {
		return nil, ErrInvalid
	}
	return &e, nil
}

type ReportService struct {
	st       *State
	SaveFile func(defaultName string) (string, error)
	SetClip  func(text string) bool
}

func NewReportService(st *State) *ReportService { return &ReportService{st: st} }

func (r *ReportService) sanitizer() report.Sanitizer {
	sys := r.st.Env.Sys
	return report.Sanitizer{
		UserProfile: sys.Env("USERPROFILE"),
		UserName:    sys.Env("USERNAME"),
		Computer:    sys.Env("COMPUTERNAME"),
		AllowIPs:    report.AllowIPsFrom(r.st.Results()),
	}
}

func (r *ReportService) Build() report.Report {
	res := r.st.Results()
	v := verdict.Overall(res, "")
	var recs []report.FixRecord
	for _, h := range r.st.History() {
		recs = append(recs, report.FixRecord{FixID: h.FixID, At: h.At, OK: h.OK, Undone: h.Undone})
	}
	mode := "windows"
	if r.st.Env.Demo {
		mode = "demo:" + r.st.Env.Scenario
	}
	return report.Build(app.Name, app.Version, mode, time.Now(), res, v, recs, r.sanitizer())
}

const MaxReportText = 256 << 10

func (r *ReportService) Copy(text string) (bool, error) {
	if len(text) > MaxReportText || !utf8.ValidString(text) {
		return false, ErrInvalid
	}
	if r.SetClip == nil {
		return false, errors.New("unavailable")
	}
	return r.SetClip(r.sanitizer().String(text)), nil
}

func (r *ReportService) Save(text string, format string) (string, error) {
	if len(text) > MaxReportText || !utf8.ValidString(text) {
		return "", ErrInvalid
	}
	var name, body string
	switch format {
	case "txt":
		name = "netwick-report.txt"
		body = r.sanitizer().String(text)
	case "json":
		name = "netwick-report.json"
		b, _ := json.MarshalIndent(r.Build(), "", "  ")
		body = string(b)
	default:
		return "", ErrInvalid
	}
	if r.SaveFile == nil {
		return "", errors.New("unavailable")
	}
	path, err := r.SaveFile(name)
	if err != nil || path == "" {
		return "", err
	}
	if err := undo.WriteAtomic(path, []byte(body)); err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

type Crosshair struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"createdAt"`
}

type ToolsService struct {
	st      *State
	mu      sync.Mutex
	SetClip func(text string) bool
}

func NewToolsService(st *State) *ToolsService { return &ToolsService{st: st} }

var crossCodeRe = regexp.MustCompile(`^[\x21-\x7E]{1,400}$`)

func (t *ToolsService) path() string { return filepath.Join(t.st.Env.ConfigDir, "crosshairs.json") }

func (t *ToolsService) load() []Crosshair {
	list := []Crosshair{}
	b, err := os.ReadFile(t.path())
	if err != nil {
		return list
	}
	_ = json.Unmarshal(b, &list)
	var ok []Crosshair
	for _, c := range list {
		if crossCodeRe.MatchString(c.Code) && utf8.RuneCountInString(c.Name) <= 40 {
			ok = append(ok, c)
		}
	}
	if ok == nil {
		ok = []Crosshair{}
	}
	return ok
}

func (t *ToolsService) Crosshairs() []Crosshair {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.load()
}

func (t *ToolsService) AddCrosshair(name, code string) ([]Crosshair, error) {
	name = strings.TrimSpace(name)
	code = strings.TrimSpace(code)
	if name == "" || utf8.RuneCountInString(name) > 40 || !utf8.ValidString(name) || !crossCodeRe.MatchString(code) {
		return nil, ErrInvalid
	}
	for _, r := range name {
		if r < 0x20 {
			return nil, ErrInvalid
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	list := t.load()
	if len(list) >= 100 {
		return nil, ErrInvalid
	}
	list = append(list, Crosshair{ID: undo.NewID(), Name: name, Code: code, CreatedAt: time.Now().UTC()})
	b, _ := json.MarshalIndent(list, "", "  ")
	if err := undo.WriteAtomic(t.path(), b); err != nil {
		return nil, err
	}
	return list, nil
}

func (t *ToolsService) DeleteCrosshair(id string) ([]Crosshair, error) {
	if len(id) != 16 {
		return nil, ErrInvalid
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	list := t.load()
	kept := []Crosshair{}
	for _, c := range list {
		if c.ID != id {
			kept = append(kept, c)
		}
	}
	b, _ := json.MarshalIndent(kept, "", "  ")
	if err := undo.WriteAtomic(t.path(), b); err != nil {
		return nil, err
	}
	return kept, nil
}

func (t *ToolsService) CopyCrosshair(id string) bool {
	if len(id) != 16 || t.SetClip == nil {
		return false
	}
	for _, c := range t.Crosshairs() {
		if c.ID == id {
			return t.SetClip(c.Code)
		}
	}
	return false
}

type SensGame struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SensResult struct {
	EDPI      float64 `json:"edpi"`
	Cm360     float64 `json:"cm360"`
	Converted float64 `json:"converted"`
	Target    string  `json:"target"`
}

func (t *ToolsService) SensGames() []SensGame {
	out := []SensGame{}
	for _, g := range appcfg.Get().Sensitivity.Games {
		out = append(out, SensGame{ID: g.ID, Name: g.Name})
	}
	return out
}

func gameYaw(id string) (float64, bool) {
	for _, g := range appcfg.Get().Sensitivity.Games {
		if g.ID == id {
			return g.Yaw, true
		}
	}
	return 0, false
}

func round(v float64, p int) float64 {
	m := math.Pow(10, float64(p))
	return math.Round(v*m) / m
}

func (t *ToolsService) Sensitivity(dpi float64, sens float64, from string, to string) (*SensResult, error) {
	if !(dpi >= 50 && dpi <= 64000) || !(sens > 0 && sens <= 1000) || math.IsNaN(dpi) || math.IsNaN(sens) {
		return nil, ErrInvalid
	}
	yf, ok := gameYaw(from)
	if !ok {
		return nil, ErrInvalid
	}
	res := &SensResult{EDPI: round(dpi*sens, 1)}
	degPerInch := yf * sens * dpi
	res.Cm360 = round(360/degPerInch*2.54, 2)
	if to != "" {
		yt, ok := gameYaw(to)
		if !ok {
			return nil, ErrInvalid
		}
		res.Converted = round(sens*yf/yt, 4)
		res.Target = to
	}
	return res, nil
}

type Link struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func (t *ToolsService) Links() []Link {
	return []Link{
		{"status", core.SrcServerStatus},
		{"support", core.SrcRiotSupport},
		{"ticket", core.SrcSubmitTicket},
		{"repair_tool", core.SrcRepairTool},
		{"precheck", core.SrcPreCheck},
		{"specs", core.SrcMinSpecs},
		{"third_party", core.SrcThirdParty},
		{"error_codes", core.SrcVALErrorCodes},
		{"vanguard_codes", core.SrcVANErrorCodes},
	}
}
