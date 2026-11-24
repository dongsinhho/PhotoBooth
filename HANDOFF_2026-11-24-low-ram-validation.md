# PhotoBooth Handoff - 2026-11-24 - Low-RAM target validation

## Repository state

- Repository: `C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth`
- Branch: `main`
- Starting HEAD: `2168f34 docs: record Windows logon acceptance and cleanup`
- At handoff start, branch was ahead of `origin/main` by two commits; inspect final status before continuing.
- Only connected Desktop Commander device: ShinPc.

## Closed gate: Windows logon startup and uninstall lifecycle

The user confirmed all Windows logon acceptance checks and the post-uninstall negative startup check passed under `SHINPC\sinhv`.

The positive report `%TEMP%\photobooth-logon-acceptance-20261123-210410.json` records:
- `status=PASS`
- user `SHINPC\sinhv`, session 3, parent process `explorer.exe`
- HKCU Run matches the active immutable release and `--kiosk`
- process exists in the interactive session
- Customer start action is visible
- no public Operator access entry
- Customer window fills the primary display

The Preview uninstall/cleanup was independently verified:
- no PhotoBooth.exe process running
- no `HKCU:\Software\Microsoft\Windows\CurrentVersion\Run\PhotoBooth` value
- `%LOCALAPPDATA%\Programs\PhotoBooth` install root absent
- `%LOCALAPPDATA%\PhotoBooth` user data preserved (`frames`, `photos`, `sessions`, `store-config-v1.json`)
- acceptance kit `C:\Users\Public\Documents\PhotoBoothLogonAcceptance-078355aa51364814a682b2a97e1f66c8` removed

The user confirmed that after uninstall and another sign-out/sign-in, PhotoBooth did not start. The positive logon and negative uninstall lifecycle gate is closed. This is an unsigned internal Preview test, not a signed production installer acceptance.

## Current target-host facts

- Host: ASRock X600-ITX
- CPU: AMD Ryzen 5 8600G
- GPU: AMD Radeon 760M
- OS: Windows 11
- Visible physical RAM: 15.12 GiB
- WSL distro: Ubuntu 26.04.1 LTS / WSL2 kernel 6.18.58.1
- This host is above the 8 GiB low-RAM candidate ceiling. WSL Ubuntu 26.04.1 is not native Ubuntu 24.04. Neither can close the target-class acceptance gate.

## Next roadmap gate: ADR 0025 / ADR 0034 low-RAM validation

Required:
1. Windows 11 x64 machine reporting <= 8,192 MiB visible RAM; use the B525 camera if available.
2. Native Ubuntu 24.04 LTS x86_64 machine with approximately 4 GiB RAM.
3. Build current Wails candidate on the qualifying host.
4. On Windows, run the read-only preflight first:

   `powershell -ExecutionPolicy Bypass -File .\tools\ui-spike\wails-v2\ui-preview-kiosk-soak.ps1 -PreflightOnly -RequireLowRamHost`

5. If preflight passes, run the full preview soak:

   `powershell -ExecutionPolicy Bypass -File .\tools\ui-spike\wails-v2\ui-preview-kiosk-soak.ps1 -DurationMinutes 10 -RequireLowRamHost`

6. Collect process-tree and host available-memory, commit and paging telemetry. Then run separate capture/render/local-output acceptance; the preview soak alone does not cover the complete booth workflow.
7. On native Ubuntu 24.04 / 4 GiB, benchmark UI, camera, capture/render/output and host headroom. WSL2 is not a substitute.
8. Establish numeric process/headroom budgets from the target-class evidence. If Wails misses the memory/headroom or touch interaction gates, evaluate an OS-specific native UI while preserving the shared Go core.

Do not create artificial memory pressure, alter pagefile settings or disable services to qualify ShinPc. Do not mark low-RAM PASS from the 15.12 GiB host.

## Other separate pending gates

- Genuine publisher-owned Code Signing certificate, approved timestamp authority, trust pin and signed per-user Windows installer.
- Ubuntu 24.04 .deb integration and install/update/rollback/uninstall tests.
- Native Ubuntu 24.04 direct USB camera and Linux printer/runtime acceptance.
- Physical printer/media acceptance remains pending/deferred until actual hardware is available.
- OS-level kiosk/desktop lockdown is distinct from app fullscreen behavior.
- Payment and remote operator configuration remain future work; SMB Remote administration stays paused.

## Current next action

Obtain/connect a real Windows 11 <=8 GiB target machine and a native Ubuntu 24.04 / 4 GiB target machine. The current connected ShinPc cannot satisfy either hardware target. Do not begin signing/production installer certification before the target-hardware UI/runtime gates are resolved.
