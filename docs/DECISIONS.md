# Decisions

Each entry: what was decided, why, and whether it still needs checking on real Windows (`[VERIFY]`).

## Toolchain and structure

1. **Wails template is `vanilla`**, not `vanilla-ts`. Verified with `wails3 init -l` (v3.0.0-beta.34): `vanilla` = "Vanilla + TypeScript + Vite".
2. **Wails pinned to `v3.0.0-beta.34`** (Go module, CLI and `@wailsio/runtime`). Wails `v3.0.0` (stable) is already published; `go mod tidy` picked it up once by itself. Reverted to the pinned version because the spec says to update only intentionally. See QUESTIONS.md.
3. **Go**: `go 1.27` with `toolchain go1.27.3`.
4. **Repository root = project root.** The spec shows a `netwick/` folder; the repository itself plays that role. The PowerShell v1.3 reference lives in `reference/` and is not part of the build.
5. **`Check` interface lives in `internal/checks`**, not `internal/core`, because it needs `platform.System`; `core` stays free of OS types. Checks that need other checks implement `Dependent` (`DependsOn()` + `RunWith`). Data passed between checks travels in `Result.Data` (never serialized to the UI).
6. **Fake platform package has no build tag**, so the same tests run on macOS and on Windows CI. Demo mode is excluded from the release Windows binary by `env_windows.go` (`windows && !demo`) vs `env_demo.go` (`!windows || demo`). A Windows demo build is possible only with `-tags demo`.
7. **No comments in application code** (instruction 0.8 of the spec). Documentation lives in `docs/`.
8. **Windows build is always x64.** The Wails CLI defaults `ARCH` to the host (arm64 on Apple Silicon), so `build/windows/Taskfile.yml` hard-codes `amd64`.

## Riot rules and sources

9. **Vanguard Pre-Check URL**: the official page is now `support.riotgames.com/en-us/riot/performance/vanguard-pre-check` (the spec had `/support-tools/`). The page wins.
10. **F05 source** is Riot's "High Latency and Disconnections" guide (`troubleshoot-pc-connection`), which contains the Reset TCP/IP step. No standalone "Reset TCP/IP" Riot article was found by search on 26.10.2026. `[VERIFY]` the exact article on 14.10.2026 mentioned in the spec and update `core/sources.go` if it exists.
11. **G14 "Remove the blocking firewall rule"** was added as an instructions-only guide. C12 must show "red, with instructions"; block rules override allow rules, so F06 alone cannot help. Netwick never deletes foreign firewall rules.
12. **Recommendations on a green result** show only informational results that have a Netwick fix (for example C19 → F10). Memory Integrity, IOMMU and Wi-Fi tips stay in "Details" so a working game is not reported as "bad" (UX principle 5).
13. **C13 Wi-Fi, C06, C07** are informational, not yellow, for the same reason.
14. **C18**: Windows Update `RebootRequired` and CBS `RebootPending` → yellow; `PendingFileRenameOperations` alone → info (many programs set it).
15. **Sensitivity calculator** includes only well-known coefficients (VALORANT 0.07, CS2 0.022, Apex 0.022, Overwatch 2 0.0066), labeled "approximate, community data" in the UI.

## Fixes and worker

16. **Hosts cleanup (C19/F10)** removes: sections whose comment marker names `NETWICK`, `CYPHERLINK-CDN`, `CYPHERLINK-ROUTE` or `RIOT-TLS-FIX` together with `>>>`/`BEGIN`/`START` (open) or `<<<`/`END` (close); and active lines that map a Riot domain (Riot never recommends pinning addresses). Comments and non-Riot lines are never touched. `[VERIFY]` the exact CypherLink marker format on the owner's laptop.
17. **Hosts backups are kept by the worker in `%ProgramData%\Netwick\backups`** (DACL: SYSTEM + Administrators), not in `%LOCALAPPDATA%`. The worker may run as a different user (admin password entered), and the unprivileged UI must not be able to feed hosts content to the elevated worker. Undo sends only a validated backup ID; the worker restores only the lines it removed.
18. **F04 uses `SetInterfaceDnsSettings`/`GetInterfaceDnsSettings`** (iphlpapi, Windows 10 2004+ = our minimum build 19041) instead of `netsh`, to avoid parsing localized output. Empty `NameServer` = DHCP. `[VERIFY]` on the laptop: static → 8.8.8.8 → undo restores; DHCP → 8.8.8.8 → undo returns to DHCP.
19. **F04 adapter**: first active wired adapter with a gateway, otherwise the first active non-VPN adapter. The worker re-checks that the LUID belongs to an active adapter.
20. **F06** creates two rules (inbound + outbound) per program, all named `Netwick — <exe>`. Undo removes only names from the fixed list.
21. **Restore point** uses `SRSetRestorePointW` (BEGIN/END_SYSTEM_CHANGE, MODIFY_SETTINGS) and then compares the newest `SystemRestore.SequenceNumber` (WMI `root\default`) before and after. If no new point appeared (System Protection off, or the 24-hour limit), the UI asks whether to continue. `[VERIFY]` behavior and the 24-hour limit on the laptop.
22. **F07 and On-Demand**: if Windows 11 25H2+ (build ≥ 26200 `[VERIFY]`) and none of TPM 2.0 / Secure Boot / IOMMU / VBS / HVCI is known to be missing, the UI asks the On-Demand question; "Yes" / "Don't know" → F07 is not executed, official steps are shown. `[VERIFY]` whether enabling On-Demand changes the `vgc`/`vgk` start type; if it does, add automatic detection.
23. **F08 and F09** are refused by Go unless the wizard was started from the matching KB entry (`VAN 128` / `VAN 68`).
24. **F01** runs in the UI process (no admin). Processes closed: `RiotClientServices`, `RiotClientUx`, `RiotClientUxRender`, `RiotClientCrashHandler`, `VALORANT`, `VALORANT-Win64-Shipping`. Vanguard processes are never in the list (tested).
25. **Killer Network Service** is found by display name through `GetServiceKeyNameW`. `[VERIFY]` display name on a machine with Killer software.

## Platform details (`[VERIFY]` on the laptop)

26. **C04 TPM** via `Tbsi_GetDeviceInfo` (no admin). `TBS_E_TPM_NOT_FOUND` → no TPM; other errors → Unknown.
27. **C05 Secure Boot** via `UEFISecureBootEnabled`; missing key → legacy BIOS → "off".
28. **C06/C07** via WMI `Win32_DeviceGuard`: VBS running = `VirtualizationBasedSecurityStatus == 2`; HVCI = `SecurityServicesRunning` contains 2; IOMMU = `AvailableSecurityProperties` contains 3 (DMA protection). Mapping `[VERIFY]` against msinfo32.
29. **Windows Security deep links**: `windowsdefender://coreisolation` for Memory Integrity; Exploit protection opens `windowsdefender://appbrowser` (App & browser control) because no documented direct link was found.
30. **CSP**: the Wails runtime talks to Go through same-origin `fetch`, so the spec's CSP works unchanged (verified in Wails server mode in Chrome). `[VERIFY]` in WebView2 (`http://wails.localhost` origin).
31. **Pipe client/server verification** relies on the go-winio connection exposing `Fd()`. `[VERIFY]` with a real fix on the laptop (checklist item 3).

## UI

32. **Report text** is rendered by the frontend from sanitized structured data (so it is in the user's language); Go sanitizes it again before clipboard or file.
33. **Watcher notification** (M6): instead of the Wails notification service (which would expose its methods to JavaScript), Netwick brings its window to the front and shows the quick-check result, only when a problem is found.
34. **Update check**: off by default; when on, Netwick checks once at start and from the "Check now" button. It only shows the version and a link to the GitHub release page.

## Hardening found during review

35. **TLS probe never disables verification.** The handshake always verifies the certificate chain against the system roots. A verification failure is classified as `CertError` immediately and no HTTP request is sent over that connection (stricter than S10). The testbed case "untrusted certificate → CertError" still holds.
36. **Elevated code does not trust environment variables for paths.** `System32` comes from `GetSystemDirectory`, `ProgramData`/`LocalAppData`/`RoamingAppData` from `SHGetKnownFolderPath`. Otherwise a modified `%SystemRoot%` could make the elevated worker run a fake `ipconfig.exe`.
37. **gosec rule G115** (integer conversion overflow) is excluded in CI: it flags every fixed-width conversion required by the Win32 ABI (`uintptr`/`uint32` sizes, DWORD values). All other high-severity rules are enforced and pass.
38. **CI scanners** (`staticcheck`, `gosec`, `govulncheck`) are installed at their latest versions in CI so new vulnerability data is always used; application dependencies stay pinned in `go.mod`/`package-lock.json`. GitHub Actions use major version tags; Dependabot keeps them current.
