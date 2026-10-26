# Netwick

> Netwick isn't endorsed by Riot Games and doesn't reflect the views or opinions of Riot Games or anyone
> officially involved in producing or managing Riot Games properties. Riot Games, and all associated
> properties are trademarks or registered trademarks of Riot Games, Inc.

**Netwick не связан с Riot Games. Программа не трогает игру и античит, не просит пароль и работает только с настройками Windows.**

Netwick is a free, open-source Windows program. In about 10 seconds it checks why the Riot Client or the game
does not launch, does not update or does not connect, explains the cause in plain words and fixes it
**only with steps from official Riot or Microsoft articles**. Every change can be undone with one button.

Netwick — бесплатная программа с открытым кодом для Windows. Примерно за 10 секунд она проверяет, почему
Riot Client или игра не запускается, не обновляется или не подключается, объясняет причину простыми словами
и исправляет её **только способами из официальных статей Riot и Microsoft**. Любое изменение отменяется одной кнопкой.

## What Netwick does / Что делает

- Reads Windows settings and Riot installation files.
- Checks that public Riot servers answer (only when you press the button).
- Changes only Windows settings named in official Riot or Microsoft articles — and only with your permission.
- Lets you undo every change with one button.
- Starts the Riot Client the same way its own shortcut does.

## What Netwick never does / Чего никогда не делает

- Does not touch the game, its files or its memory; no overlays, no injection, no traffic interception.
- Does not disable, stop or bypass Vanguard anti-cheat.
- Does not ask for or store your login and password.
- Does not press keys or click for you; no macros, no auto-accept.
- Does not show other players' stats or hidden information; does not use the Riot API.
- Does not send data anywhere: no telemetry, no ads. No VPN, proxy or "accelerators".

See [Riot's rules on third-party applications](https://support.riotgames.com/en-us/valorant/account/third-party-applications).

## Download and first launch

1. Download `netwick.exe` and `SHA256SUMS` from the [Releases](https://github.com/Kaoriel-Studio/Netwick/releases) page.
2. Netwick is not code-signed yet, so Windows SmartScreen shows **"Windows protected your PC"**.
   Click **More info** → **Run anyway**. (Окно «Windows защитила ваш компьютер» → «Подробнее» → «Выполнить в любом случае».)
3. Netwick starts **without** administrator rights. Windows asks for permission (UAC) only when you choose a fix that needs it.

Requirements: Windows 10 (build 19041+) or Windows 11, x64, Microsoft Edge WebView2 (built into Windows 11 and most Windows 10).

### Check that the file is genuine

- Hash: in PowerShell run `Get-FileHash .\netwick.exe -Algorithm SHA256` and compare with `SHA256SUMS`.
- Build provenance (proves the exe was built by GitHub Actions from this repository):
  `gh attestation verify netwick.exe --repo Kaoriel-Studio/Netwick`
- Each release links to a VirusTotal scan.

### Antivirus false positives

Unsigned Go programs are sometimes flagged by mistake. Netwick has no hidden PowerShell windows, does not add itself
to autostart and never downloads executables. If your antivirus flags it, please report a false positive to the
antivirus vendor and open an issue here with the detection name.

## Privacy and security

- [docs/PRIVACY.md](docs/PRIVACY.md) — what data Netwick touches (nothing leaves your computer).
- [docs/SECURITY.md](docs/SECURITY.md) — threat model and safety checklist.
- [SECURITY.md](SECURITY.md) — how to report a vulnerability.

## For developers

Stack: Go 1.27.3, Wails v3.0.0-beta.34, vanilla TypeScript + Vite. No CGO for Windows.

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
cd frontend && npm ci && cd ..

wails3 dev                                   # interface on macOS, demo mode
go run . --demo=owner-real-run               # or NETWICK_DEMO=<scenario>
go test ./...                                # all unit tests incl. TLS testbed
cd frontend && npm run lint && npm run build && npm test
wails3 build GOOS=windows VERSION=2.0.0      # bin/netwick.exe (x64, asInvoker)
```

Demo scenarios (Windows is simulated, nothing on your computer changes):
`all-ok`, `owner-real-run`, `no-internet`, `isp-cuts-some-ips`, `tpm-off-win11`, `secure-boot-off`,
`vgc-disabled`, `firewall-blocks`, `vpn-on`, `van-68`, `van-128`, `old-hosts-entries`.
Demo mode is not available in the Windows release build.

Project layout, decisions and the manual Windows checklist: [docs/DECISIONS.md](docs/DECISIONS.md),
[docs/TESTING-WINDOWS.md](docs/TESTING-WINDOWS.md). The previous PowerShell version is in [reference/](reference/).

## License

[MIT](LICENSE).
