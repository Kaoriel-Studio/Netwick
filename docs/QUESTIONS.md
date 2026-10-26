# Questions for the owner

None of these block the work; each has a safe default already applied.

1. **Wails v3.0.0 stable is published.** The project stays on `v3.0.0-beta.34` as the spec requires. Upgrade to `v3.0.0`? (Needs a full test run and a Windows check.)
2. ~~Repository URL~~ — resolved: https://github.com/Kaoriel-Studio/Netwick
3. **"Submit a ticket" link** points to `https://support.riotgames.com/en-us/valorant/submit-a-ticket`. Please confirm the exact URL in a browser; if it differs, change `SrcSubmitTicket` in `internal/core/sources.go`.
4. **Demo build for testers.** Should releases also include a separate `netwick-demo.exe` (built with `-tags demo`) so non-technical testers can see the interface safely? Right now only macOS runs demo mode.
