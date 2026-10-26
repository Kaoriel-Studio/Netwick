# Netwick security design

This document describes how Netwick protects the user's computer. It is reviewed on every change
to the Wails services or the worker.

## Threat model

| Threat | Mitigation |
| --- | --- |
| Code in the WebView tries to gain administrator rights | The UI process runs `asInvoker` (no admin). The WebView loads only embedded files under a strict CSP. Every service method validates input. Privileged work happens only in a separate worker that accepts a fixed list of action IDs. |
| Another program runs `netwick.exe --worker` with malicious arguments | Arguments are parsed against an allowlist (`internal/worker/args.go`): action IDs from F02–F10 or `UNDO`, a 32-hex-char pipe name, a numeric adapter LUID. No paths, commands or free text. The worker rediscovers all paths itself and re-checks preconditions. UAC still asks the user. Fuzz-tested. |
| Someone intercepts the UI ↔ worker pipe | Random 128-bit pipe name, DACL limited to the current user SID and `BUILTIN\Administrators`, one connection only. The UI checks the client PID equals the worker it started; the worker checks the pipe server is the same `netwick.exe` image. Messages are JSON, ≤ 64 KB, unknown fields rejected. |
| DLL hijacking from the Downloads folder | `SetDefaultDllDirectories(LOAD_LIBRARY_SEARCH_SYSTEM32)` is the first call in `main`. All system DLLs are loaded with `NewLazySystemDLL`. System tools (`ipconfig.exe`, `netsh.exe`) run by absolute path from `%SystemRoot%\System32`, arguments as an array, never through `cmd.exe` or PowerShell. |
| Personal data leaks through the report | Report contains only structured check results. `internal/report` removes profile paths, user and PC names, e-mails, tokens (JWT, long hex/base64, UUID/PUUID), Riot IDs and IP addresses that are not Riot server addresses. Covered by unit tests. Text copied from the UI is sanitized again in Go. |
| Supply chain | Exact versions in `go.mod`/`go.sum` and `package-lock.json`; `govulncheck`, `staticcheck`, `gosec`, `go vet`, `npm audit` in CI; Dependabot. |

## Review checklist (Section 7.2)

| ID | Where |
| --- | --- |
| S1 manifest `asInvoker`, `uiAccess="false"` | `build/windows/wails.exe.manifest` |
| S2 embedded resources only, URL allowlist | `main.go` (`AssetFileServerFS`), `internal/app/env.go` `AllowedURL` |
| S3 CSP | `frontend/index.html`, checked by `frontend/scripts/lint-security.mjs` and `check-dist.mjs` |
| S4 no `eval`/`innerHTML` | `frontend/scripts/lint-security.mjs` (fails the build) |
| S5 devtools off in release | release build uses `-tags production` (Wails disables WebView2 devtools) |
| S6 exported methods validated | list below; `internal/services/exported_test.go` fails if it changes |
| S7 worker allowlist | `internal/worker/args.go`, `internal/undo/undo.go` `Validate` |
| S8 no shell | `internal/platform/winsys/actions.go` `runSystemTool` |
| S9 DLL search order | `env_windows.go` `hardenProcess` |
| S10 TLS validation | `internal/probe/probe.go`: chain verified against system roots; on failure the probe only classifies `CertError` and sends nothing beyond `GET /` |
| S11 network destinations | Riot hosts from `internal/probe/network.go`, DNS 1.1.1.1/8.8.8.8/77.88.8.8, GitHub Releases API only if enabled |
| S12 update check | `AppService.CheckUpdate` shows a link only |
| S13 Riot logs | read-only, last 5 MB of 6 newest files, only whitelisted hostnames and error types are kept |
| S14 sanitization | `internal/report` + tests |
| S15 atomic writes, backups | `internal/undo.WriteAtomic`, hosts backup in `%ProgramData%\Netwick\backups` |
| S16 own folder permissions | `env_windows.go` `SecureDir` (current user + SYSTEM) |
| S17 limits | 4 s per probe stage, 64 KB per probe, 10 s total check budget, context cancellation |
| S18 panic safety | `internal/checks/check.go` `runOne` recovers → `Unknown` |
| S19 no telemetry | there is no telemetry code |
| S20 docs | this file, `docs/PRIVACY.md`, `SECURITY.md` |

## Exported service methods (callable from the WebView)

Every method below validates its input in Go.

- `AppService.Info`
- `AppService.OpenURL`
- `AppService.Settings`
- `AppService.SaveSettings`
- `AppService.CheckUpdate`
- `CheckService.Run`
- `CheckService.RunOnly`
- `CheckService.Cancel`
- `CheckService.Last`
- `FixService.Describe`
- `FixService.Guide`
- `FixService.Apply`
- `FixService.UndoList`
- `FixService.UndoAll`
- `FixService.LaunchGame`
- `FixService.CompleteGuide`
- `FixService.Open`
- `KBService.Search`
- `KBService.Get`
- `ReportService.Build`
- `ReportService.Copy`
- `ReportService.Save`
- `ToolsService.Crosshairs`
- `ToolsService.AddCrosshair`
- `ToolsService.DeleteCrosshair`
- `ToolsService.CopyCrosshair`
- `ToolsService.SensGames`
- `ToolsService.Sensitivity`
- `ToolsService.Links`
