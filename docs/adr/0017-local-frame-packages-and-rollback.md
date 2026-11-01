# ADR 0017: Local declarative frame packages and rollback

- **Status:** Accepted for the current Go core and Wails prototype; not a marketplace/production release decision
- **Date:** 2026-10-29
- **Decision owners:** PhotoBooth engineering

## Context

The root renderer already accepts a validated, non-executable schema-v1 JSON frame. The roadmap calls for installing and rolling back local assets/packages before online catalog/payment work. The broader CONTENT_PACKAGE_SPEC.md describes a proposed future shape (themes, assets, signatures and a marketplace); treating it as complete would silently widen scope and create unimplemented claims.

## Decision

Implement only a versioned local .pbframe package containing a manifest, one existing schema-v1 frame definition, and optional preview PNG/JPEG plus optional UTF-8 LICENSE.txt. The complete on-disk contract and CLI workflow are documented in FRAME_PACKAGE_FORMAT.md.

The implementation has a bounded ZIP reader, strict JSON, allowlisted entry names, path/symlink checks, file count/size limits, SHA-256 verification for every payload, preview content/dimension checks, and frame-schema validation through the production renderer. Package creation does not modify source files and refuses to overwrite an existing archive. Installed versions live under frames/packages/<package-id>/<package-version> and are immutable. Installing never changes the active frame.

The Wails operator screen exposes Browse, Validate, Install locally, Activate, Roll back frame and Remove for installed versions that are eligible for safe uninstall. It does not fetch content from a network. Activation stages the currently active frame as a rollback backup and persists a pending activation record before the station profile is changed. After the config save, the activation is committed. Recovery compares the stored active-frame path with the pending target after an interrupted process. Rollback validates the backup digest and frame schema, writes a separate restored frame file, persists the restored path, and only then deletes rollback metadata. Rollback itself keeps installed package files; a separate explicit uninstall can remove an inactive version after active/pending/rollback references are checked.

Activation aligns capture shot count to the selected definition's verified slot count. Saving a managed package path through the ordinary frame-path field is rejected so package activation cannot bypass rollback preparation. Package/settings operations are blocked during a running capture session.

## Consequences

- Operators can create, validate, install and use frame packages completely offline.
- A bad package is rejected before activation; an install does not auto-switch the active configuration.
- SHA-256 supplies integrity checking, not authorship/authenticity. There are no signatures, publisher identity, licensing verification, trust store or marketplace moderation in schema v1.
- Uninstall refuses active versions and any package referenced by pending activation or committed rollback state; it re-verifies the package before deleting only its immutable version directory. Automatic retention/garbage collection, full theme packages, arbitrary asset folders, animation, custom fonts, remote sync, paid listings and creator payouts remain out of scope.
- The package manager and UI are tested on Windows in the isolated Wails prototype; the Wails framework is still not the production UI selection. Linux runtime/hardware and physical camera output were not exercised by package tests.

## Validation

- Unit tests cover package creation, validation, no-overwrite, source immutability, install/list/idempotence, immutable-version conflict, digest tampering, malformed previews, unsafe ZIP paths, activation commit, rollback restore, abort/interruption recovery, tampered rollback backups, active/pending/rollback-reference uninstall guards, symlinked package/rollback parents, bounded regular-file reads, and successful safe uninstall.
- CLI tests exercise create → validate → install → list.
- Wails integration test exercises validate → install (not active) → activate a two-slot frame → capture settings update to two shots → rollback to original three-slot frame while keeping package installed.
- The Wails finalization test prevents a terminal session state from becoming visible before final preview/output metadata is ready; the demo capture-render-save test passed 50 consecutive runs after this race fix.
- UI Automation passed settings-only smoke and a full isolated profile flow: validate → install → activate → rollback → uninstall. It confirmed shot-count restoration, retained the package through rollback, then removed the version only after rollback metadata was clear. The smoke changed no user profile and did not open the camera.
- Root tests/vet, Windows build, Linux/amd64 compile-only check, 15 Python camera POC tests, Wails tests/vet, Vite production build and Wails Windows build passed during this implementation session.
