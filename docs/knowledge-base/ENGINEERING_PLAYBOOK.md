# Engineering Playbook

**Status:** Proposed working agreement

## Development workflow
1. Read repository instructions and relevant knowledge-base documents.
2. Clarify user-visible behavior and acceptance criteria.
3. Inspect the implementation and dependencies.
4. Define boundaries and failure behavior before implementation.
5. Implement a small vertical slice.
6. Add tests for normal, boundary, failure, and recovery paths.
7. Run relevant formatters, tests, static checks, and builds available in the repo.
8. Update docs and ADRs when behavior or important design decisions change.
9. Report changes, checks actually run, results, and what remains unverified.

## Module boundaries
Use explicit interfaces where useful for:
- Camera device provider.
- Capture session orchestration.
- Image rendering.
- Printer/export output.
- Local configuration and session persistence.
- Package validation and installation.
- Cloud API and synchronization.

Avoid abstraction without a practical substitution/testing reason. Do not leak vendor SDK types into product/domain logic.

## Testing strategy

### Unit tests
Prioritize session transitions, cancellation/timeouts, geometry/layout validation, deterministic rendering, package schema validation, entitlement policy, and sync retries.

### Integration tests
Exercise local persistence, package staging/activation/rollback, render-to-file, output adapters, and API contracts.

### Hardware tests
Maintain a repeatable matrix for camera model, firmware, connection mode, Windows version, SDK/driver, and printer/output device. Mark physical-device tests explicitly; never claim they passed unless run.

### Reliability tests
Test offline mode, network loss during sync, camera disconnect, capture timeout, printer unavailable, low disk space, app restart, corrupt/unsupported packages, and rollback.

## Code quality
- Use readable control flow for stateful/hardware operations.
- Keep long-running I/O off the UI thread and use cancellation/timeouts.
- Make retries bounded and idempotent where possible.
- Use structured diagnostics and stable error codes.
- Avoid unbounded background tasks and silent exception swallowing.
- Version configuration and schemas.
- Verify dependency licenses and vendor SDK distribution rules.

## Documentation
- README.md: project orientation and high-level status.
- docs/knowledge-base/: current product and engineering context.
- docs/adr/: important decisions and consequences.
- Comments explain non-obvious rationale, limitations, and safety assumptions.
- Label examples that are conceptual or not yet a final contract.

## Change report template
- Summary
- Files changed
- Tests/checks run and observed results
- Compatibility/security implications
- Remaining risks or questions
- Knowledge-base/ADR updates

