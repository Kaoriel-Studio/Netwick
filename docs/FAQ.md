# FAQ

[English](FAQ.md) · [Русский](FAQ.ru.md)

## Is Netwick allowed by Riot?

Netwick never touches the game, its files or memory, the anti-cheat or the Riot API. It does not show other players' data, press keys or click for you. It only reads and changes Windows settings and Riot Client's own cache files, with your permission. See [Riot's rules on third-party applications](https://support.riotgames.com/en-us/valorant/account/third-party-applications).

## Is it safe to press "Do it for me"?

Every step first tells you what will change, why, whether a restart is needed and whether it can be undone. Nothing happens until you press the button. Windows asks for permission (UAC) when a step needs administrator rights. Steps that are not described in Riot articles are marked.

## What does each repair do?

| Scenario | What happens | Restart | Undo |
| --- | --- | --- | --- |
| Auto-repair | Goes through the safe steps from simple to serious and asks at every step | no | where possible |
| Quick repair | Closes Riot, clears the client cache and DNS, restarts the Vanguard service | no | not needed |
| Full network reset | Cache, sign-in session, server addresses, Vanguard service, Windows network reset | yes | no |
| Servers and routes | Tests every Riot server address; optionally pins the working one in the `hosts` file | no | yes |
| Bonjour service | Stops Apple Bonjour; it is not uninstalled | no | yes |
| Reinstall Vanguard | Restarts the service first, then shows the standard Windows uninstall | yes | — |
| Remove routes | Deletes Riot addresses from the `hosts` file | no | yes |

## I pressed "Reset sign-in session". Do I lose my account?

No. You only have to sign in again. Netwick never sees or stores your password.

## The check says "Problem on Riot's side"

Riot reports an outage or maintenance. Local repairs will not help — wait and look at [Riot server status](https://status.riotgames.com/).

## What is the health score?

100 minus points for problems found by the last full check: 22 per red result, 9 per yellow one, 2 per check that could not run. Informational hints do not lower it. The history and medals live only on your computer and can be cleared in Settings.

## What do the connection-quality numbers mean?

Netwick opens a connection to a Riot server 12 times in a row. *Delay* is the average time, *jitter* is how much it jumps between measurements, *loss* is the share of failed attempts. Below 80 ms with no loss is good; high jitter or loss usually means Wi-Fi trouble or a busy router.

## Windows says "Windows protected your PC"

Netwick is not code-signed yet. Click **More info → Run anyway**. You can check the file with the hash from the release page, see the [README](../README.md#verify-the-download).

## Where do I report a problem?

Open an issue in the repository. For a security problem use private vulnerability reporting, see [SECURITY.md](../SECURITY.md). Do not post your support report publicly without reading it first.
