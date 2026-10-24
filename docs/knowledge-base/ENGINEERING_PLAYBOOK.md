# Engineering Playbook

**Status:** Proposed working agreement

## Development workflow
1. Read repository instructions and relevant knowledge-base documents.
2. Clarify user-visible behavior and acceptance criteria.
3. Inspect current implementation and dependencies.
4. Define module boundaries and failure behavior before coding.
5. Implement a small vertical slice.
6. Add tests for normal, boundary, failure, and recovery paths.
7. Run relevant formatters, tests, static checks, and builds available.
8. Update docs and ADRs when behavior or important decisions change.
9. Report changed files, checks actually run, results, and remaining uncertainty.

## Module boundaries
Use explicit interfaces where useful for:
- Camera provider.
- Capture session orchestration.
- Image rendering.
- Printer/export output.
- Local configuration and session persistence.
- Package validation and installation.
- Payment provider and payment policy.
- Media-delivery policy/provider.
- Cloud API and synchronization.
- Platform-specific secure storage and kiosk integration.

Avoid abstraction without a substitution/testing reason. Do not leak vendor SDK types into product/domain logic.

## Cross-platform development
- Declare the supported OS distribution/version/architecture instead of claiming generic Linux support.
- Keep OS-specific code in adapters and isolate native libraries.
- Test installation, kiosk startup, touch/keyboard input, auto-start, updates, camera, and printer on each supported platform.
- A successful build is not proof of hardware compatibility.
- Prefer reproducible release artifacts and signed updates.

## Testing strategy

### Unit tests
Prioritize session transitions, cancellation/timeouts, geometry/layout validation, deterministic rendering, package schema validation, payment policy, media-delivery policy, and retry logic.

### Integration tests
Exercise local persistence, package staging/activation/rollback, render-to-file, output adapters, API contracts, payment callbacks, idempotency, and media access-token expiry.

### Hardware tests
Maintain a matrix for camera model, firmware, connection mode, OS, SDK/driver, printer/output device, and result. Mark tests requiring physical devices and never claim they passed unless actually run.

### Reliability tests
Test offline mode, network loss during sync/upload/payment, camera disconnect, capture timeout, printer unavailable, low disk space, app restart, corrupt packages, unsupported schema, payment duplicate/late webhook, and rollback.

## Code quality
- Use readable control flow for stateful/hardware operations.
- Keep long-running I/O off the UI thread and use cancellation/timeouts.
- Make retries bounded and idempotent where possible.
- Use structured diagnostics and stable error codes.
- Avoid unbounded background tasks and silent exception swallowing.
- Version configuration and schemas.
- Verify dependency licenses and vendor SDK distribution rules.

## Documentation
- README.md: orientation and high-level status.
- docs/knowledge-base/: current product/engineering context.
- docs/adr/: important decisions and trade-offs.
- Code comments explain non-obvious rationale and safety assumptions.
- Label conceptual examples and incomplete contracts.

## Change report template
- Summary
- Files changed
- Tests/checks run and observed results
- Compatibility/security implications
- Remaining risks or questions
- Knowledge-base/ADR updates

