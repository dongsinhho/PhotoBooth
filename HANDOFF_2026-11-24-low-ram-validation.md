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


## Independent Windows packaging work - 2026-11-24

Per-user NSIS template prototype is compiled but not installed:

- Source: `tools/ui-spike/wails-v2/build/windows/installer/project.nsi`.
- Wails command: `wails build -clean -nsis -installscope user -o PhotoBooth -platform windows/amd64`.
- Verified with Wails CLI v2.17.0 and NSIS 3.13: successful compile.
- Negative NSIS compile with `WAILS_INSTALL_SCOPE=machine`: rejected as expected.
- Negative NSIS compile with `REQUEST_EXECUTION_LEVEL=admin`: rejected as expected.
- Preview installer: `tools/ui-spike/wails-v2/build/bin/PhotoBoothNSISPreview-amd64-installer.exe` (7,189,122 bytes, SHA-256 `C751B5ABD63F0C8AAE6041FD03821B96090598D5B684A4F3E44B451F42FFDEC5`, Authenticode `NotSigned`).
- The installer is isolated to `%LOCALAPPDATA%\Programs\PhotoBooth NSIS Preview` and the separate HKCU Run value `PhotoBoothNSISPreview`; machine scope and admin execution level are rejected.
- The installer was not executed. No actual profile registry entry or install directory was created. The main versioned `PhotoBooth` lifecycle remains untouched.
- The generated `wails_tools.nsh` changes from Wails' build generator were reverted; only the authored `project.nsi` changes are intended source modifications.

Do not mistake this prototype for the production installer. It is a direct-binary Wails/NSIS build and does not yet integrate immutable versioned release payloads, hash verification, side-by-side update/rollback or first-install SMB default-off provisioning. Publisher signing/trust is absent. The next hardware-independent packaging step is an NSIS bootstrapper around the existing PowerShell versioned package install/uninstall scripts, with an explicit and safe design for the machine-wide SMB provisioning step. No installer runtime test or public distribution is approved.


## Versioned Windows NSIS bootstrapper - 2026-11-24

The repository's project-owned NSIS bootstrapper is now reviewed and compiled as an internal unsigned Preview:

- Builder: `tools/release/windows/New-PhotoBoothWindowsInstaller.ps1`.
- NSIS source: `tools/release/windows/PhotoBoothInstaller.nsi`.
- It embeds the exact versioned package ZIP and `.sha256`, plus `Install-PhotoBoothWindowsPackage.ps1`, `Uninstall-PhotoBoothWindowsPackage.ps1` and `Release.Common.ps1`.
- NSIS uses `RequestExecutionLevel user`; application install root is `%LOCALAPPDATA%\Programs\PhotoBooth` and setup/uninstaller files are kept in the per-user setup root.
- The existing PowerShell install lifecycle remains authoritative for checksum/manifest/binary integrity, immutable releases, active pointer, rollback, HKCU Run and data preservation.
- First install runs the packaged SMB helper with `-Action Disable -WaitForElevation`; UAC is required for the machine-wide default-off baseline. A cancellation/failure must prevent activation. Upgrades preserve operator SMB configuration. Do not bypass this in the end-user bootstrapper.

Build command used successfully (NSIS 3.13):

    .\tools\release\windows\New-PhotoBoothWindowsInstaller.ps1 -Version 0.1.0-nsis-preview -PackagePath .\artifacts\windows\PhotoBooth-internal-alpha-0.1.0-nsis-preview-windows-amd64.zip -OutputDirectory $env:TEMP\photobooth-nsis-bootstrapper-audit -MakensisPath "$env:LOCALAPPDATA\Tools\NSIS-3.13\nsis-3.13\makensis.exe" -AllowUnsignedPreview

Build result: 5,513,522 bytes, SHA-256 `58289056F126FA094EA1A79223F664EA7ED086A6C6833AC98870EC9BFDF81496`, Authenticode `NotSigned`. Build was emitted under TEMP. Source ZIP sidecar verified and package contained the expected four files. PowerShell parser checks passed for builder, install, uninstall and Release.Common. Builder rejected unsigned package when `-AllowUnsignedPreview` was omitted.

The synthetic `release-lifecycle-smoke.ps1` passed all checks. Its harness was corrected to pass `-SkipAutoStart` for first install, update and rollback. During the first run before that fix, it wrote a temporary lifecycle-test path to the real HKCU Run value; that exact value was removed immediately. The rerun passed and confirmed the real PhotoBooth Run value remained absent/unchanged. No PhotoBooth process or installed Preview was launched/created by the NSIS build.

`artifacts/` is now ignored by Git; do not commit generated ZIP/EXE files.

**Still pending:** actual NSIS setup install/uninstall on a disposable Windows profile, first-install UAC accept/cancel paths and SMB default-off verification, upgrade/rollback via the wrapper, logon and data-preservation acceptance. This must be planned carefully because first install deliberately changes machine-wide SMB policy. The installer is unsigned and internal-only. Genuine publisher signing/trust and low-RAM target hardware acceptance remain separate pending gates.

Current Git intent: commit only source, tests and documentation; exclude all generated artifacts. Keep the Wails direct-binary NSIS Preview spike as an isolated experiment, not the distribution installer.
