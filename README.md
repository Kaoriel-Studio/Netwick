# Netwick

**English** · [Русский](README.ru.md)

A free, open-source Windows app that finds out why the Riot Client or the game won't launch, update or connect —
and fixes it step by step: with steps from official Riot and Microsoft articles and with common safe repairs
(the app marks every step that is not in Riot articles).

> Netwick isn't endorsed by Riot Games and doesn't reflect the views or opinions of Riot Games or anyone
> officially involved in producing or managing Riot Games properties. Riot Games and all associated
> properties are trademarks or registered trademarks of Riot Games, Inc.

## What it does

Press **Check computer** and in about 10 seconds Netwick:

- reads your Windows settings and Riot installation files (TPM, Secure Boot, firewall, services, free disk space, `hosts`, VPN and so on);
- checks that the public Riot servers answer — only when you press the button;
- explains the cause in plain words and suggests a fix;
- applies the fix **only with your permission**, and every change can be undone with one button;
- starts the Riot Client the same way its own shortcut does.

**My progress** keeps a local history of your checks: a health score, a trend line since the last time and medals.
**Tools** has a connection-quality test (delay, jitter, loss to Riot servers), a check of power plan and Game Mode,
a crosshair notebook and a sensitivity calculator. All of it stays on your computer.

The **Repair** section has every scenario on its own: auto-repair, quick repair (client cache, DNS, Vanguard service),
full network reset for VAL 43, a per-address test of Riot servers with optional pinning of working addresses in `hosts`,
Bonjour, Vanguard reinstall, removing routes and undo. Before a scenario starts you see its steps and can untick any of them.

## What it never does

- Never touches the game, its files or its memory. No overlays, no injection, no traffic interception.
- Never disables, stops or bypasses the Vanguard anti-cheat.
- Never asks for or stores your Riot login or password.
- Never presses keys or clicks for you: no macros, no auto-accept.
- Never shows other players' stats or hidden information, and doesn't use the Riot API.
- Never sends data anywhere: no telemetry, no ads. No VPN, proxy or "accelerators".

See also [Riot's rules on third-party applications](https://support.riotgames.com/en-us/valorant/account/third-party-applications).

## Disclaimer

Netwick is provided **"as is", without warranty of any kind**, as stated in the [MIT license](LICENSE).
Netwick changes Windows settings (network, services, the `hosts` file) and deletes Riot Client cache and sign-in files
**only when you confirm a step**, and it explains every step first. Even so, any change to a computer carries some risk.

- You use Netwick **at your own risk**. The authors are not liable for any loss or damage, including data loss,
  a computer that stops working or starting, network or internet problems, lost settings, account problems,
  sanctions from game publishers, or any other direct or indirect damage.
- Not every change can be undone. Steps marked "cannot be undone" (for example the network reset) stay in effect.
- Read what each step does before you press "Do it for me". If you are not sure, choose "Show me how to do it myself"
  or skip the step. Save your work before steps that need a restart.
- Netwick does not guarantee that a problem will be fixed, and it cannot fix problems on Riot's side.
- Netwick is an independent project and is not endorsed by or affiliated with Riot Games, Microsoft or any other company.

## Install

**Requirements:** Windows 10 (build 19041+) or Windows 11, x64, Microsoft Edge WebView2
(already included in Windows 11 and most Windows 10 installs).

1. Download `netwick.exe` and `SHA256SUMS` from the [Releases](https://github.com/Kaoriel-Studio/Netwick/releases) page.
2. Run `netwick.exe`. The app is not code-signed yet, so Windows SmartScreen will say
   **"Windows protected your PC"** — click **More info → Run anyway**.
3. Netwick starts **without** administrator rights. Windows asks for permission (UAC) only when you pick a fix that needs it.

### Verify the download

- Hash: in PowerShell run `Get-FileHash .\netwick.exe -Algorithm SHA256` and compare it with `SHA256SUMS`.
- Build provenance (proves the exe was built by GitHub Actions from this repository):
  `gh attestation verify netwick.exe --repo Kaoriel-Studio/Netwick`
- Every release links to a VirusTotal scan.

### Antivirus false positives

Unsigned Go programs are sometimes flagged by mistake. Netwick opens no hidden PowerShell windows, doesn't add itself
to autostart and never downloads executables. If your antivirus flags it, please report a false positive to the
vendor and open an issue here with the detection name.

## Privacy and security

Nothing about you or your computer leaves it. Details:

- [docs/PRIVACY.md](docs/PRIVACY.md) — what data Netwick touches and where it stays
- [docs/FAQ.md](docs/FAQ.md) — what each repair does, common questions
- [dev/SECURITY.md](dev/SECURITY.md) — threat model and safety checklist
- [SECURITY.md](SECURITY.md) — how to report a vulnerability

## For developers

**Stack:** Go 1.27.3, [Wails v3](https://v3alpha.wails.io/) (3.0.0-beta.34), vanilla TypeScript + Vite. No CGO on Windows.

### Set up

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
cd frontend && npm ci && cd ..
```

### Try it on macOS or Linux (demo mode)

Netwick targets Windows, but on other systems it runs in **demo mode**: Windows is simulated and nothing on your
computer changes. Build the frontend once (Go embeds `frontend/dist`), then start the app:

```bash
(cd frontend && npm run build)
go run .                        # starts with "all-ok"; or --demo=val-43, or NETWICK_DEMO=<scenario>
```

The strip under the top bar switches the simulated situation at any time and has a **Restart PC** button that
simulates a reboot. Fixes really change the simulated computer, so you can go through every scenario like a Windows user.

Scenarios: `all-ok`, `val-43`, `owner-real-run`, `no-internet`, `isp-cuts-some-ips`, `riot-incident`, `tpm-off-win11`,
`secure-boot-off`, `vgc-disabled`, `firewall-blocks`, `vpn-on`, `van-68`, `van-128`, `old-hosts-entries`.

Demo mode is not available in the Windows release build.

### Common commands

| Command | What it does |
| --- | --- |
| `wails3 dev` | Dev mode with hot reload (demo mode off Windows) |
| `go test ./...` | All Go tests, including the TLS testbed |
| `cd frontend && npm run lint && npm run build && npm test` | Frontend lint, build and checks |
| `wails3 build GOOS=windows VERSION=2.0.0` | Builds `bin/netwick.exe` (x64, runs as the invoking user) |

`wails3 task test` runs the Go and frontend checks together.

CI regenerates the Wails bindings and fails if `frontend/bindings` differs from what is committed, so commit
regenerated bindings together with the Go changes that caused them.

### More

- [dev/DECISIONS.md](dev/DECISIONS.md) — design decisions
- [dev/TESTING-WINDOWS.md](dev/TESTING-WINDOWS.md) — manual test checklist for a real Windows PC

## License

[MIT](LICENSE)
