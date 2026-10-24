# Theme and Frame Package Specification

**Status:** Proposed draft; review schema before implementation

## Purpose
Define a versioned, validated content format installable by the Windows player without executing arbitrary code. Themes affect the player UI; frames define photo composition.

## Illustrative package layout

~~~text
example-package/
├── manifest.json
├── definition.json
├── preview.webp
├── assets/
│   ├── background.webp
│   ├── overlay.png
│   └── stickers/
└── LICENSE.txt
~~~

Names are illustrative, not a final compatibility contract.

## Manifest fields to define
- packageId: stable unique ID.
- packageType: theme or frame.
- packageVersion: immutable content version.
- schemaVersion: manifest/definition format version.
- name, description, authorId, license.
- minPlayerVersion and declared capabilities.
- Canvas/output dimensions and supported aspect ratios where relevant.
- Asset file list, media types, byte lengths, and cryptographic hashes.
- Entry definition path and preview paths.
- Optional signature and publishing metadata.

The final schema must define allowed characters, limits, required fields, version syntax, and compatibility rules.

## Frame definition concepts
- Canvas dimensions and coordinate system.
- Photo slots with coordinates, dimensions, crop/fit policy, and rotation.
- Explicit layer order for background, slots, masks, overlay, stickers, and text.
- Font references restricted to packaged or approved fonts, with license checks.
- Static output variants, expected resolution, color handling, and printer safe margins.
- Preview generation rules and a known-good sample package.

## Theme definition concepts
- Supported player screens and components.
- Color tokens, typography, approved assets, spacing, and layout properties.
- State-specific screens for idle, countdown, capture, review, output, and errors.
- Allowed transition/animation properties and performance limits.
- Sound assets only when supported, with mute/configuration behavior.
- No arbitrary shell commands, native code, remote URLs, dynamic script imports, or unrestricted JavaScript.

## Validation and installation
1. Reject unknown or incompatible schemas with an actionable error.
2. Reject archive path traversal and unsafe filenames.
3. Limit compressed/uncompressed size, file count, individual asset size, image dimensions, and decode time.
4. Verify every declared file exists and matches its hash.
5. Inspect media type instead of trusting extension alone.
6. Do not fetch remote URLs embedded in packages during a guest session.
7. Stage package files temporarily; activate only after full validation.
8. Keep the last known-good version for rollback.
9. Treat metadata, previews, fonts, and visual assets as untrusted input.
10. Require provenance, licensing information, moderation, and takedown procedures before launching a public marketplace.

## Animation and output policy
Animation support is unresolved. Distinguish on-screen preview animation, static printer output, and downloadable GIF/video output if supported. For paper output, select an explicit frame/time or documented static composite; printing must be deterministic.

## Compatibility strategy
- Published package versions are immutable.
- schemaVersion is distinct from packageVersion.
- Minimum player version and required capabilities are explicit.
- Add migrations/adapters only when covered by tests.
- Decide whether paid licenses cover a store, account, device count, or other unit before implementing entitlements.

