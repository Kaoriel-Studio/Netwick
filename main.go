package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/kaoriel-studio/netwick/internal/app"
	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/services"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed frontend/src/i18n/en.json frontend/src/i18n/ru.json
var i18nFS embed.FS

var version = "2.0.0-dev"

func init() {
	application.RegisterEvent[checks.ProgressEvent](services.EventProgress)
	application.RegisterEvent[string](services.EventNavigate)
}

func main() {
	hardenProcess()
	app.Version = version
	if len(os.Args) > 1 && os.Args[1] == "--worker" {
		os.Exit(workerMain(os.Args[1:]))
	}
	env, scenarios, cleanup, err := buildEnv(os.Args[1:])
	if err != nil {
		fatal(err)
		return
	}
	defer cleanup()
	if err := runUI(env, scenarios); err != nil {
		log.Fatal(err)
	}
}

func trayLabels(lang string) map[string]string {
	if lang != "ru" && lang != "en" {
		lang = systemLanguage()
	}
	b, err := i18nFS.ReadFile("frontend/src/i18n/" + lang + ".json")
	if err != nil {
		b, _ = i18nFS.ReadFile("frontend/src/i18n/en.json")
	}
	var m map[string]string
	_ = json.Unmarshal(b, &m)
	return m
}

func runUI(env *app.Env, scenarios []string) error {
	store := app.NewStore(env)
	st := services.NewState(env, store)
	appSvc := services.NewAppService(st)
	appSvc.Scenarios = scenarios
	checkSvc := services.NewCheckService(st)
	fixSvc := services.NewFixService(st)
	reportSvc := services.NewReportService(st)
	toolsSvc := services.NewToolsService(st)

	wapp := application.New(application.Options{
		Name:        app.Name,
		Description: "Checks why the game client does not start or connect and fixes it with official steps",
		Services: []application.Service{
			application.NewService(appSvc),
			application.NewService(checkSvc),
			application.NewService(fixSvc),
			application.NewService(&services.KBService{}),
			application.NewService(reportSvc),
			application.NewService(toolsSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{},
	})

	st.Emit = func(name string, data any) { wapp.Event.Emit(name, data) }
	if env.OpenURL == nil {
		env.OpenURL = func(u string) error { return wapp.Browser.OpenURL(u) }
	}
	reportSvc.SetClip = func(text string) bool { return wapp.Clipboard.SetText(text) }
	toolsSvc.SetClip = reportSvc.SetClip

	win := wapp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            app.Name,
		Width:            960,
		Height:           680,
		MinWidth:         760,
		MinHeight:        560,
		BackgroundColour: application.NewRGB(246, 247, 249),
		URL:              "/",
	})
	reportSvc.SaveFile = func(name string) (string, error) {
		return wapp.Dialog.SaveFile().SetFilename(name).AttachToWindow(win).PromptForSingleSelection()
	}

	tray := newTrayController(wapp, win, st, store)
	watch := services.NewWatcher(st, func(issue bool) {
		if issue {
			win.Show()
			win.Focus()
			wapp.Event.Emit(services.EventNavigate, "watch-issue")
		}
	})
	appSvc.SetTray = tray.Set
	appSvc.SetWatch = watch.Set
	s := store.Settings()
	tray.Set(s.Tray)
	watch.Set(s.WatchClient)
	defer watch.Set(false)

	return wapp.Run()
}

type trayController struct {
	app   *application.App
	win   *application.WebviewWindow
	st    *services.State
	store *app.Store
	tray  *application.SystemTray
}

func newTrayController(a *application.App, w *application.WebviewWindow, st *services.State, store *app.Store) *trayController {
	return &trayController{app: a, win: w, st: st, store: store}
}

func (t *trayController) Set(on bool) {
	if !on {
		if t.tray != nil {
			t.tray.Destroy()
			t.tray = nil
		}
		return
	}
	if t.tray != nil {
		return
	}
	l := trayLabels(t.store.Settings().Language)
	label := func(k string) string {
		if v, ok := l[k]; ok {
			return v
		}
		return k
	}
	menu := application.NewMenu()
	menu.Add(label("tray.check")).OnClick(func(*application.Context) {
		t.win.Show()
		t.win.Focus()
		t.app.Event.Emit(services.EventNavigate, "check")
	})
	menu.Add(label("tray.quick")).OnClick(func(*application.Context) {
		t.win.Show()
		t.win.Focus()
		t.app.Event.Emit(services.EventNavigate, "quick")
	})
	menu.Add(label("tray.open")).OnClick(func(*application.Context) {
		t.win.Show()
		t.win.Focus()
	})
	menu.AddSeparator()
	menu.Add(label("tray.quit")).OnClick(func(*application.Context) { t.app.Quit() })
	t.tray = t.app.SystemTray.New()
	t.tray.SetIcon(trayIcon)
	t.tray.SetTooltip(app.Name)
	t.tray.SetMenu(menu)
	t.tray.OnClick(func() {
		t.win.Show()
		t.win.Focus()
	})
}

func fatal(err error) {
	showFatal(fmt.Sprintf("Netwick: %v", err))
	os.Exit(1)
}
