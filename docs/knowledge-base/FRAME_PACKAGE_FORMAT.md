# Local Frame Package Format (.pbframe), Schema v1

**Status:** Implemented in the Go core and exposed by the isolated Wails prototype
**Date:** 2026-10-29

This is the deliberately small first package format agreed for local frame installation. It does **not** implement the complete proposed theme/catalog/marketplace format in CONTENT_PACKAGE_SPEC.md.

## Supported package

A .pbframe file is a ZIP archive containing exactly two to four regular files:

- manifest.json — package identity, declared payloads and hashes.
- definition.json — the existing frame JSON schema v1, validated by internal/booth/render.
- Optional one preview: preview.png **or** preview.jpg.
- Optional LICENSE.txt — UTF-8 text supplied by the creator.

Directories, duplicate entries, symlinks, unknown filenames, path traversal, unsupported compression methods, undeclared entries, malformed JSON and trailing JSON documents are rejected. The only accepted ZIP compression methods are Store and Deflate.

A minimal manifest looks like this (hash/size values here are illustrative only):

~~~json
{
  "schema_version": 1,
  "package_type": "frame",
  "package_id": "sample-strip",
  "package_version": "1.0.0",
  "name": "Sample photo strip",
  "description": "Three-photo local frame",
  "author": "Creator name",
  "license": "CC0-1.0",
  "entrypoint": "definition.json",
  "files": [
    {
      "path": "definition.json",
      "size": 512,
      "sha256": "replace-with-64-lowercase-hex-characters",
      "media_type": "application/json"
    }
  ]
}
~~~

Every payload file is declared once with its exact path, uncompressed size, media type and SHA-256 digest. The manifest itself is not hashed, to avoid recursive hashing. IDs use lowercase letters, digits and hyphens (1–63 characters). Versions use canonical numeric major.minor.patch form with no leading zeros (except zero itself). Package versions are immutable once installed.

## Resource limits

| Item | Limit |
|---|---:|
| Compressed archive | 16 MiB |
| Manifest | 64 KiB |
| Frame definition | 1 MiB |
| Optional preview | 4 MiB |
| Optional LICENSE.txt | 64 KiB |
| Archive file count | 2–4 total |
| Preview dimensions | At most 4000 px per side and 16 million pixels |
| Frame slots | Existing renderer schema v1 limit: 1–12 |

The renderer also validates the frame schema, canvas geometry, text, colors, output quality and resource bounds. A frame must have exactly as many slots as the configured capture count before a capture session can start.

## Local workflow

Create an archive from an existing frame without modifying its source:

~~~powershell
go run ./apps/frame-package create --frame ./examples/frames/photo-strip-v1.json --out ./photo-strip-1.0.0.pbframe --id photo-strip --version 1.0.0 --name "Sample photo strip" --author "Your name" --license "CC0-1.0"
~~~

Validate it without installing:

~~~powershell
go run ./apps/frame-package validate --package ./photo-strip-1.0.0.pbframe
~~~

Install into the current user's local managed frame store, without activating:

~~~powershell
go run ./apps/frame-package install --package ./photo-strip-1.0.0.pbframe
go run ./apps/frame-package list
~~~

--frames-dir <path> overrides the local managed root for CLI install/list. The default root is the current user's PhotoBooth data directory under the OS cache location; PHOTOBOOTH_DATA_DIR can override the base directory for isolated setups and tests.

The Wails prototype's **Settings & frame → Frame packages** panel offers the same local workflow: Browse, Validate package, Install locally, Activate, Roll back frame and Remove an installed version. Install does not activate automatically. Activation first stages a verified rollback point, then saves the station profile to the package's version directory and updates shot count to the frame's verified slot count. Rollback verifies the saved backup's SHA-256 and frame schema, writes a new restored frame file, saves the station profile and only then clears rollback metadata. The immutable installed package is kept after rollback.

Uninstall is permitted only for a non-active package version that is not referenced by the pending activation or current rollback transition. The manager refuses deletion when any of those guards fail, re-verifies the installed package immediately before removal, and rejects symlinked package/version parent directories and rollback-state directories. Bounded file reads compare the identity of the opened file with the initially inspected file, and ZIP parsing uses the exact verified bytes rather than reopening the path. If a package is still needed by rollback, activate another frame or complete/clear the rollback transition first. Uninstall removes that version directory only; it does not delete source JSON files, session photos, rendered strips or the station profile.

An interrupted activation is reconciled on the next package-state refresh: if the persisted profile points at the package target, the pending rollback is committed; if the old path remains active, the pending change is discarded. Corrupt/unsupported package versions are reported and are never auto-rewritten.

## Security and trust boundaries

- A package is declarative content; there is no script or plugin execution.
- No network access or remote URL fetching occurs during validate, install, activation or capture.
- SHA-256 verifies payload integrity against the manifest, but **does not authenticate the creator or prove that a claimed license is valid**. Schema v1 has no signature, publisher identity service, trust store or moderation process.
- The license field is creator-declared metadata, not legal advice or a rights verification result. Review redistribution rights before sharing a package publicly.
- Package preview bytes are validated as PNG/JPEG, including content format and dimension limits.
- Installed package versions are immutable; attempting to replace the same ID/version with different manifest data fails.
- These checks protect against malformed packages but do not make an untrusted Windows account or a user-writable profile directory a security sandbox.

## Not in schema v1

Full themes/UI styling, arbitrary assets/ trees, animation, multiple preview images, custom fonts, remote downloads, package signatures, publisher trust, retention policies/automatic garbage collection, in-app frame editor, public catalog, paid content, moderation/takedown workflows and entitlement/payment handling remain future work. They must not be described as implemented by the current package manager.
