# ADR 0018: Opt-in local session/output retention

- **Status:** Accepted for the root Go core/CLI
- **Date:** 2026-10-29

## Context

Stage 2 includes configurable local data retention. Session captures and finalized local outputs can contain guest images, so deletion must not be silently enabled, happen during a capture session, or recursively target arbitrary user directories. Existing schema-v1 profiles should remain safe when new settings are introduced.

## Decision

Add optional `storage.session_retention_days` and `storage.output_retention_days` fields to the station config. Both default to zero (disabled), and allowed values are 0–3650 days. Implement cleanup in the separate `internal/booth/retention` package and `apps/retention-cleanup` CLI. The player does not call this package automatically and the Wails settings screen does not expose deletion yet.

Default CLI invocation is a read-only dry-run. The report lists candidate paths, ages, file counts and byte counts and prints a stable plan digest. Destructive CLI apply requires both `--apply` and the digest copied from an operator-reviewed dry-run. The core `retention.Apply` API also requires that exact digest, so programmatic callers cannot bypass the confirmation gate. The CLI rebuilds the current plan and rejects a stale/wrong digest. A second in-memory preflight occurs before filesystem mutation.

Only finalized PhotoBooth layouts under the two configured roots are recognized: work sessions have complete numbered captures and `rendered.jpg`; output folders contain exactly `photo.jpg`. Unknown/incomplete layouts, symlinks, non-regular files, future-dated content and temporary output files are not candidates. Overlapping roots are rejected when both policies are enabled.

Deletion uses an opened Go `os.Root` handle for root-relative operations. The implementation checks root/candidate identity, a metadata fingerprint and retention age before deleting. It revalidates child identities and metadata immediately before each removal, and refuses to remove directories that gained unexpected entries. It is conservative but not a secure-erasure mechanism and does not claim transactional all-or-nothing deletion if an I/O failure occurs during a multi-file removal.

## Consequences

- Old profiles remain unaffected because omitted/zero retention values disable cleanup.
- Operators can inspect the exact current candidate paths before authorizing a deletion.
- If roots, policy or eligible candidate metadata changes after review, the supplied digest does not match and no apply begins.
- Unknown/incomplete entries require manual inspection and can remain on disk indefinitely.
- There is no startup cleanup, scheduled task, Wails delete UI, quarantine/undo workflow, cleanup audit database or secure wipe.
- This step does not alter camera/provider/render/capture code or enable network access.

## Validation

Temporary-fixture tests cover read-only dry-run, successful digest-confirmed apply, missing/wrong/stale digest refusal, candidate-set changes, recently modified data, incomplete sessions, temporary/unknown output entries, symlink skips, root overlap, retention bounds, disabled defaults and cancellation. The retention CLI/core/config targeted tests and `go vet` pass on Windows. Full project regression and Windows/Linux builds are required before this should be described as fully validated.
