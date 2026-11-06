# Local Session and Output Retention

**Status:** Implemented in the root Go core, the separate retention CLI, and the Windows Preview operator Settings UI\
**Default:** Disabled; UI requires preview + explicit confirmation, CLI requires a reviewed plan digest and `--apply`\
**Date:** 2026-10-29

## Policy

Schema-v1 station storage supports two independent values:

- `storage.session_retention_days`: age limit for complete local work sessions under `sessions_dir`.
- `storage.output_retention_days`: age limit for finalized local copies under `output_dir`.

A value of `0` disables that cleanup category. Valid enabled values are 1–3650 days. Missing fields in older schema-v1 profiles default to zero, so adding this capability does not silently enable deletion. The running booth player does not invoke cleanup automatically.

## Two-step operator workflow

First inspect the exact configuration that points at the intended booth directories. A Wails profile on Windows is normally stored under `%LOCALAPPDATA%\PhotoBooth\store-config-v1.json`. Run a read-only plan:

~~~powershell
$config = Join-Path $env:LOCALAPPDATA 'PhotoBooth\store-config-v1.json'
go run ./apps/retention-cleanup --config $config --session-days 30 --output-days 30
~~~

Review every candidate path, age, file count and byte count. The report includes `PLAN_DIGEST=<64-hex>`, which covers the normalized roots, retention policy and exact candidate metadata. No files are changed during this run.

Only after reviewing the paths, rerun the same command with the digest copied from the dry-run:

~~~powershell
$digest = 'PASTE_THE_REVIEWED_PLAN_DIGEST_HERE'
go run ./apps/retention-cleanup --config $config --session-days 30 --output-days 30 --apply --confirm-plan $digest
~~~

Do not literally use the placeholder digest. Keep roots, configuration and retention overrides identical between runs. If the eligible candidate set or its observed metadata changes, the digest will not match and the CLI refuses to apply; run and review a new dry-run instead. `--apply` by itself is not enough to delete anything.

To use the retention values saved in the station config, omit `--session-days` and `--output-days`. To override the directory roots for an isolated/manual run, supply `--sessions-dir` and/or `--output-dir`. Avoid applying retention to a profile whose paths you have not verified.

## Candidate allowlist

The planner only considers a lowercase 32-character hexadecimal session ID and the existing PhotoBooth path layout.

A complete session candidate must contain `captures/capture-01.jpg` through a contiguous numbered sequence (1–12) and a non-empty `rendered.jpg`. It must not contain other entries. A local output candidate must contain exactly one non-empty `photo.jpg`. Temporary output files, unknown files, incomplete sessions, symlinks, non-regular files and unexpected directory layouts are skipped rather than recursively deleted.

Age is based on the newest modification time among a candidate's recognized files and directories. Future-dated data is not eligible. Session and output roots may not overlap when both policies are enabled.

## Apply safeguards

- The planner is read-only. The CLI requires `--apply` **and** a matching `--confirm-plan` digest; the core `retention.Apply` API independently requires the matching digest too.
- On apply, the tool rebuilds the candidate set and compares it with the reviewed digest.
- The core checks root identity, candidate identity, recognized layout, metadata fingerprint and age again immediately before removal.
- Filesystem operations after opening the root use Go `os.Root` with root-relative paths, reject symlinks/unknown entries, and remove only recognized entries. Unexpected entries or changed metadata cause a fail-closed error.
- It refuses overlapping active roots and rejects symlinked parent components.
- No cleanup runs on player startup, no scheduled task is installed, and no user profile is modified by the tests.
- The CLI only writes to the two configured local roots; it does not upload or log photo bytes.

This is a conservative filesystem cleanup, not secure erasure. Deleted files may remain recoverable from filesystem snapshots, backups or storage media. Operators should retain independent backups according to booth/business needs. Unrecognized or incomplete directories may require manual inspection and are intentionally not auto-cleaned.

## Validation

The automated tests use temporary directories and synthetic files. They cover dry-run non-destructiveness, confirmed apply, wrong/missing digest rejection, candidate-set changes after review, recent data retention, incomplete/unknown layout skips, symlink skips, overlapping roots, disabled defaults, range validation and cancellation. No real photo/session directory is a test fixture.

## Out of scope

There is no automatic schedule, Wails settings UI for retention, quarantine/undo UI, configurable per-file exceptions, database of deletion audit records, cloud retention, printer spool cleanup or secure wiping. Any scheduler or UI integration must preserve the opt-in and reviewed-plan behavior.

## Windows Preview operator Settings integration (2026-11-06)

The Wails Windows Preview Settings screen now exposes both retention values (0–3650 days; 0 remains disabled). Saving either value only updates the station config file; it never runs cleanup.

The Review old local data panel uses a separate read-only preview call. It displays candidate type, session ID, exact local path, last-modified time, file count, total bytes, scan/skip counts, retention values and a plan digest. The UI lists at most 100 paths. When the plan contains more than 100 candidates, Apply is disabled so an operator cannot approve a truncated list; use the CLI to review the complete candidate set instead.

Apply requires the latest non-empty in-memory preview, matching digest and unchanged configured roots/policy, followed by a native confirmation dialog. The core retention.Apply rebuilds and verifies the candidate set and checks every target immediately before removal. The plan is one-use and is discarded on policy change, preview discard, success or failure. Session start/continue/restart, output retry, settings save and frame-package mutations are blocked during an active preview/apply operation to avoid races.

Validation on 2026-11-06: Wails Go tests verify default-disabled behavior, dry-run non-destructiveness, wrong-digest rejection, exact apply to synthetic temp folders, one-use token behavior, policy-change invalidation and operation guards. Windows UI persistence smoke verified 0/0 defaults, saving 30/45 days, read-only preview with an empty profile, disabled Apply for zero candidates, restart persistence, graceful process close and temporary-profile cleanup. No real photo/session path was used and no cleanup was applied to user data.

This is still the Windows Preview prototype, not production UI certification. Cleanup remains manual and is not scheduled. It is not secure erasure.
