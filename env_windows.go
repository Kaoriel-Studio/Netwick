//go:build windows && !demo

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/app"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/fixes"
	"github.com/kaoriel-studio/netwick/internal/ipc"
	"github.com/kaoriel-studio/netwick/internal/platform/winsys"
	"github.com/kaoriel-studio/netwick/internal/worker"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func hardenProcess() {
	_ = windows.SetDefaultDllDirectories(windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
}

func system32() string { return winsys.System32() }

func messageBox(text string, flags uint32) int32 {
	t, _ := windows.UTF16PtrFromString(text)
	c, _ := windows.UTF16PtrFromString("Netwick")
	r, _ := windows.MessageBox(0, t, c, flags)
	return r
}

func showFatal(msg string) {
	messageBox(msg, windows.MB_OK|windows.MB_ICONERROR)
}

func shellOpen(file, args string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	f, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	var a *uint16
	if args != "" {
		a, _ = windows.UTF16PtrFromString(args)
	}
	return windows.ShellExecute(0, verb, f, a, nil, windows.SW_SHOWNORMAL)
}

func openSystem(o fixes.Opener) error {
	switch o {
	case fixes.OpenWindowsUpdate:
		return shellOpen("ms-settings:windowsupdate", "")
	case fixes.OpenAppsFeatures:
		return shellOpen("ms-settings:appsfeatures", "")
	case fixes.OpenProxySettings:
		return shellOpen("ms-settings:network-proxy", "")
	case fixes.OpenVPNSettings:
		return shellOpen("ms-settings:network-vpn", "")
	case fixes.OpenNetworkStatus:
		return shellOpen("ms-settings:network-status", "")
	case fixes.OpenDateTime:
		return shellOpen("ms-settings:dateandtime", "")
	case fixes.OpenCoreIsolation:
		return shellOpen("windowsdefender://coreisolation", "")
	case fixes.OpenExploitProtection:
		return shellOpen("windowsdefender://appbrowser", "")
	case fixes.OpenMsconfig:
		return shellOpen(filepath.Join(system32(), "msconfig.exe"), "")
	case fixes.OpenTaskManager:
		return shellOpen(filepath.Join(system32(), "Taskmgr.exe"), "/0 /startup")
	case fixes.OpenFirewallAdvanced:
		return shellOpen(filepath.Join(system32(), "WF.msc"), "")
	case fixes.OpenSystemInfo:
		return shellOpen(filepath.Join(system32(), "msinfo32.exe"), "")
	case fixes.OpenPower:
		return shellOpen("ms-settings:powersleep", "")
	case fixes.OpenGameMode:
		return shellOpen("ms-settings:gaming-gamemode", "")
	case fixes.OpenDeviceManager:
		return shellOpen(filepath.Join(system32(), "devmgmt.msc"), "")
	}
	return errors.New("unknown target")
}

const webView2Client = `{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

func webView2Installed() bool {
	check := func(root registry.Key, path string) bool {
		k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
		if err != nil {
			return false
		}
		defer k.Close()
		v, _, err := k.GetStringValue("pv")
		return err == nil && v != "" && v != "0.0.0.0"
	}
	return check(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\`+webView2Client) ||
		check(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\EdgeUpdate\Clients\`+webView2Client) ||
		check(registry.CURRENT_USER, `Software\Microsoft\EdgeUpdate\Clients\`+webView2Client)
}

func systemLanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err == nil && len(langs) > 0 && strings.HasPrefix(strings.ToLower(langs[0]), "ru") {
		return "ru"
	}
	return "en"
}

const idYes = 6

var uiMutex windows.Handle

func singleInstance(name string) bool {
	n, _ := windows.UTF16PtrFromString(name)
	h, err := windows.CreateMutex(nil, false, n)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false
	}
	uiMutex = h
	return h != 0
}

func buildEnv(args []string) (*app.Env, []string, func(), error) {
	for _, a := range args {
		if strings.HasPrefix(a, "--demo") {
			return nil, nil, nil, errors.New("demo mode is not available in this build")
		}
	}
	if !singleInstance(`Local\NetwickUI`) {
		messageBox(trayLabels(systemLanguage())["app.already_running"], windows.MB_OK|windows.MB_ICONINFORMATION)
		os.Exit(0)
	}
	if !webView2Installed() {
		l := trayLabels(systemLanguage())
		if messageBox(l["app.webview2_missing"], windows.MB_YESNO|windows.MB_ICONWARNING) == idYes {
			_ = shellOpen(core.SrcMSWebView2, "")
		}
		os.Exit(1)
	}
	sys := winsys.New(app.UserAgent())
	local, err1 := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	roaming, err2 := windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, 0)
	if err1 != nil || err2 != nil || local == "" || roaming == "" {
		return nil, nil, nil, errors.New("user profile folders are not available")
	}
	dataDir := filepath.Join(local, "Netwick")
	configDir := filepath.Join(roaming, "Netwick")
	if sid, err := winsys.CurrentUserSID(); err == nil {
		sddl := "D:P(A;OICI;FA;;;" + sid + ")(A;OICI;FA;;;SY)"
		_ = winsys.SecureDir(dataDir, sddl)
		_ = winsys.SecureDir(configDir, sddl)
	}
	env := &app.Env{
		Sys:        sys,
		Act:        winsys.NewActions(sys),
		Runner:     ipc.ElevatedRunner{},
		DataDir:    dataDir,
		ConfigDir:  configDir,
		OpenSystem: openSystem,
		OpenURL: func(u string) error {
			if !app.AllowedURL(u) {
				return errors.New("blocked")
			}
			return shellOpen(u, "")
		},
	}
	cleanup := func() {
		if uiMutex != 0 {
			windows.CloseHandle(uiMutex)
		}
	}
	return env, nil, cleanup, nil
}

func workerMain(argv []string) int {
	a, err := worker.ParseArgs(argv)
	if err != nil {
		return 2
	}
	if !singleInstance(`Global\NetwickWorker`) {
		return 3
	}
	conn, err := ipc.Dial(a.Pipe)
	if err != nil {
		return 4
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Minute))
	var req worker.Request
	if err := ipc.Read(conn, &req); err != nil {
		return 5
	}
	if len(req.Undo) > 0 && (len(a.Actions) != 1 || a.Actions[0] != worker.ActionUndo) {
		return 6
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	sys := winsys.New(app.UserAgent())
	x := &worker.Executor{Sys: sys, Act: winsys.NewActions(sys)}
	resp := x.Run(ctx, a, req)
	if err := ipc.Write(conn, resp); err != nil {
		return 7
	}
	return 0
}
