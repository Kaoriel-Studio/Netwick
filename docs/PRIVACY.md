# Privacy

[English](PRIVACY.md) · [Русский](PRIVACY.ru.md)

**Short version: Netwick sends nothing about you anywhere.** No telemetry, no analytics, no crash reports, no accounts, no ads.

## What Netwick connects to

Only when you press a button, and never with your personal data:

| What | Why | What is sent |
| --- | --- | --- |
| Public Riot servers (the addresses the Riot Client uses) | "Check computer", "Servers and routes" | One ordinary `GET /` per address with `User-Agent: Netwick/<version>` |
| Public DNS servers 1.1.1.1, 8.8.8.8, 77.88.8.8 | Find the addresses of Riot servers | A normal DNS question for a Riot server name |
| Riot's public server status (JSON, region from Settings) | "Riot server status" | A plain request, nothing about you |
| One Riot server, port 443 | "Connection quality" test | Only opening and closing a connection, no data |
| GitHub (only if you turn on update checks) | Find the latest release number | A plain request, nothing about you |

## What stays on your computer

| File | Contents |
| --- | --- |
| `%APPDATA%\Netwick\settings.json` | Your settings |
| `%APPDATA%\Netwick\crosshairs.json` | Your crosshair notes |
| `%LOCALAPPDATA%\Netwick\undo.json` | What Netwick changed, so it can be undone |
| `%LOCALAPPDATA%\Netwick\profile.json` | Check history, health scores, counters and medals (no names, no addresses) |
| `%ProgramData%\Netwick\backups\` | Backups of the `hosts` file (readable only by administrators) |

You can erase the history and medals any time in **Settings → Clear history and medals**.

## What Netwick reads and changes

- It reads Windows settings and Riot installation files. From Riot Client logs it takes only server names and error types, never log lines.
- It never asks for or stores your Riot login or password.
- "Clear client cache" and "Reset sign-in session" delete Riot Client files on this computer only when you press the button. Session files are deleted, never copied.
- Every change is explained first and asked for first. Reversible changes can be undone with one button.

## Support report

The report is created only when you open it, and copied or saved only when you press the button. Your user name, computer name, home folder path, e-mail addresses, tokens, Riot IDs and IP addresses (except Riot server addresses) are removed automatically. Read it before you send it.
