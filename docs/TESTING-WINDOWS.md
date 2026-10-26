# Manual test on the owner's Windows laptop

Run on a **physical** PC (the game and Vanguard do not run in virtual machines — VAN 138).
Write the result of every step (OK / problem + screenshot) next to it.

**Before starting:** make sure System Protection is on (Start → "Create a restore point" → drive C: → Protection: On),
so Windows can roll back if something goes wrong. **Do not run F05 (network reset) on your main PC without a restore point.**

## 1. First launch

1. Download `netwick.exe` and `SHA256SUMS` from the release (or copy `bin/netwick.exe` built on the Mac).
2. Optional: check the hash in PowerShell: `Get-FileHash .\netwick.exe -Algorithm SHA256` — it must match the line in `SHA256SUMS`.
3. Double-click `netwick.exe`.
   - **Expected:** Windows SmartScreen shows "Windows protected your PC" → click "More info" → "Run anyway".
   - **Expected:** the Netwick window opens. **No UAC prompt** at start.
4. Start `netwick.exe` a second time. **Expected:** message "Netwick is already running", no second window.

## 2. Check computer

1. Make sure the game works normally today.
2. Press "Check computer". Time it.
   - **Expected:** ≤ 10 seconds; the five groups turn to statuses.
   - **Expected:** green "All good" (or yellow with an honest reason). No red if the game works.
   - **Expected:** Vanguard line says "Vanguard protection is currently off — this is normal…" if `vgc` is Stopped/Manual.
   - **Expected:** if old CypherLink entries are in `hosts`, a recommendation "remove old entries" (not red).
3. Open "Details" and compare with reality: Windows build, TPM, Secure Boot, free space, connection type.

## 3. Permission request (UAC)

1. Search `VAL 29` → "Run suitable fixes" → "Next step" → "Allow the game in Windows Firewall" → "Do it for me".
2. **Expected:** the UAC window appears and names `netwick.exe`.
3. Click **No**. **Expected:** message "You declined the Windows permission request. Nothing was changed." The app keeps working.

## 4. F02 — clear DNS cache

1. Search `ping` → "Connection problems or high ping" → "Run suitable fixes" → skip the first two steps → "Clear DNS cache" → "Do it for me".
2. Accept UAC. **Expected:** "Done". The "Undo Netwick changes" button does **not** appear for this step (nothing to undo).

## 5. F06 — firewall rules

1. Search `VAL 29` → run fixes → "Allow the game in Windows Firewall" → Do it for me → accept UAC.
2. Open "Windows Defender Firewall with Advanced Security" → Inbound and Outbound rules.
   - **Expected:** rules named `Netwick — VALORANT.exe`, `Netwick — vgc.exe`, `Netwick — RiotClientServices.exe`.
3. Press "Undo Netwick changes" → Undo → accept UAC.
   - **Expected:** only the `Netwick — …` rules disappear. Your other rules are untouched.

## 6. F04 — DNS

1. Note the current DNS: Settings → Network → your connection → DNS server assignment (Automatic/DHCP or manual addresses).
2. Search `VAN: Incompatible Software` → run fixes → skip until "Use public DNS 8.8.8.8 and 8.8.4.4" → Do it for me.
   - **Expected:** a restore point is created (or the "Could not create a restore point" question appears — answer and write down what happened).
   - **Expected:** DNS shows 8.8.8.8 / 8.8.4.4.
3. "Undo Netwick changes". **Expected:** DNS returns exactly to step 1 (Automatic, or your old addresses).

## 7. F07 — Vanguard start type (PC without On-Demand)

1. Press Win+R → `services.msc` → note `vgc` Startup type (usually Manual).
2. Search `VAN 57` → run fixes → "Start Vanguard together with Windows" → Do it for me.
   - If asked about On-Demand: answer **No** (only if you really did not enable it).
   - **Expected:** `vgc` Startup type = Automatic.
3. "Undo Netwick changes". **Expected:** `vgc` back to the value from step 1.

## 8. C19/F10 — old CypherLink entries

1. Open `C:\Windows\System32\drivers\etc\hosts` in Notepad, copy its content to a text file on the desktop (your own backup).
2. If the check shows "Found … old entries", press "Go through recommendations" → "Remove old Netwick and CypherLink entries" → Do it for me.
   - **Expected:** CypherLink/Netwick lines are gone, other lines unchanged.
   - **Expected:** a backup file exists in `C:\ProgramData\Netwick\backups\` (open as administrator).
3. "Undo Netwick changes". **Expected:** the removed lines come back at the end of the file.

## 9. Error code search

Type each and check the result:

| Input | Expected |
| --- | --- |
| `VAN 68` | VAN 68, "Vanguard lost its connection", 8 steps in Riot order |
| `van-81` | VAN -81 |
| `43` | VAL 43 "A system has timed out" |
| `VAN: RESTRICTION: 1` | VAN: RESTRICTION: 1, Secure Boot |
| `68` | both VAN 68 and VAL 68 to choose from |
| `qwerty` | "Nothing found" |

## 10. Report

1. Open "Copy report for support" → Copy → paste into Notepad.
   - **Expected:** no Windows user name, no PC name, no `C:\Users\<name>` (shown as `%USERPROFILE%`), no home IP.

## 11. Launch the game

1. On a green result press "Launch game".
   - **Expected:** the Riot Client opens (the game starts as with Riot's own shortcut). Vanguard tray icon stays.

## 12. Items marked [VERIFY]

Write down what you see for each (they are listed in `docs/DECISIONS.md`):

1. TPM / Secure Boot / Memory Integrity / IOMMU results match `msinfo32` ("System Summary": Secure Boot State, Virtualization-based security, Kernel DMA Protection).
2. "Open Core isolation" button opens Windows Security → Core isolation. "Open Exploit protection" opens App & browser control.
3. Restore point: created on the first change; on a second change within 24 hours Netwick asks whether to continue.
4. If you use Vanguard On-Demand: does `vgc`/`vgk` Startup type differ from a normal install? (Write both values.)
5. If you have Killer networking software: does `services.msc` show "Killer Network Service" exactly?
6. With "Check when the Riot Client starts" on: start the Riot Client and wait 10 s. Netwick shows itself only if it finds a problem.
7. With "Icon next to the clock" on: the tray icon appears; menu items work.
8. Old CypherLink marker lines in your `hosts` look like: (paste them here).
