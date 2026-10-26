# Privacy

**Short version: Netwick sends nothing about you anywhere.**

- No telemetry, no analytics, no crash reporting, no accounts, no ads.
- Netwick never asks for or stores your Riot login or password.
- When you press "Check computer", Netwick connects to public Riot servers (the same addresses the Riot Client uses) and to public DNS servers (1.1.1.1, 8.8.8.8, 77.88.8.8). It sends one ordinary web request (`GET /`) with the header `User-Agent: Netwick/<version> (+repository link)`. Nothing else is sent.
- If you turn on "Check for Netwick updates", Netwick asks GitHub for the latest release number. Nothing about you or your computer is sent.
- Riot Client logs are read only on your computer, and only server names and error types are taken from them. Log lines are never copied.
- Settings, the list of changes (for Undo) and crosshair notes stay on this computer:
  - `%APPDATA%\Netwick\settings.json`, `%APPDATA%\Netwick\crosshairs.json`
  - `%LOCALAPPDATA%\Netwick\undo.json`
  - `%ProgramData%\Netwick\backups\` (hosts backups, readable only by administrators)
- The support report is created only when you open it, and copied or saved only when you press the button. Your user name, computer name, home folder path, e-mail addresses, tokens, Riot IDs and IP addresses (except Riot server addresses) are removed automatically.

Because no personal data is transferred, Russian Federal Law 152-FZ requirements on personal data transfer do not apply. If this ever changes, consent and anonymization come first.
