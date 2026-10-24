# Licensing and Support Strategy

**Status:** Product direction accepted; exact legal terms and license text are not finalized  
**Last updated:** 2026-10-24

## Business intent

PhotoBooth is a software-only project. The creator does not sell cameras, computers, printers, booth cabinets, or bundled hardware.

The intended distribution model is:
- The official player can be used without a software usage fee, including for commercial photobooth operations.
- Operators can self-install and self-support it.
- Optional paid services include setup, troubleshooting, training, camera/printer integration, migration, operational consulting, and authorized custom core changes.
- The standard product must remain useful without buying support.
- The product should make supported configuration and content extension easy so most store-level customization does not require changing the core.

This document records product intent, not a legally reviewed end-user license.

## Distinguish four different things

### 1. Free commercial use
A store may use the official unmodified player to earn money from photobooth sessions without paying a per-booth or per-session software license fee, subject to the eventual terms.

### 2. Supported customization
Operators may use documented settings, frame/theme package formats, and supported extension contracts to change behavior within the product's supported capabilities. Examples include countdown length, number of shots, image timing, output layout, print delay, paper/output settings, and visual content, provided a setting/capability is implemented and documented.

This is product configuration/content, not permission to modify the program's copyrighted core.

### 3. Core software modification
Under the preferred launch direction, editing the core source/binary, creating a derivative product, rebranding/embedding the core, or redistributing modified builds requires prior written authorization or a separate license from the rights holder, where legally enforceable. Paid services may be offered to perform authorized changes.

The exact rights, exceptions, reverse-engineering language, and redistribution terms require legal review. Do not publish a homemade EULA as if it were legal advice.

### 4. Support services
Support is optional and separately priced. It may include installation/setup, remote troubleshooting, device compatibility work, configuration, custom integration, and agreed changes. Support fees must not be confused with a mandatory app license fee.

## Important terminology

**Do not call the core "Open Source"** if the license forbids modification or derived works without permission. Open Source Initiative's definition requires licenses to permit derived works and not discriminate against commercial use:
- [Open Source Definition](https://opensource.org/osd)
- [OSI FAQ: commercial use and modification](https://opensource.org/faq)

Possible accurate descriptions after legal review:
- "Free to use for commercial photobooth operations."
- "Free desktop software with a documented configuration and content-extension system."
- "Source-available under a custom license" only if source is actually distributed and the custom terms are clearly stated.

A public GitHub repository containing documentation does not by itself grant a license to copy, modify, or redistribute software. No source code license should be inferred from repository visibility.

## Options to decide before source-code publication

### Option A — Free commercial-use freeware with controlled core (recommended given current intent)
- Distribute official binaries at no usage fee.
- Explicitly allow use for commercial booth sessions.
- Provide public configuration and theme/frame package contracts.
- Keep core source closed or source-available only under custom terms.
- Require permission/separate license for core modifications and derivative/redistributed builds.
- Charge only for optional support and authorized custom engineering.

**Trade-off:** strongest control over core modifications, but it is not OSI open source. Third-party developers cannot be promised the freedom to modify and redistribute the core.

### Option B — Genuine open-source core plus paid support (alternative if community code contributions matter more)
- Publish source under an OSI-approved license that permits modification and commercial use.
- Users may modify and redistribute according to that license; the creator cannot require every such modification to be separately approved.
- Paid support, setup, hosted services, official integrations, and clearly separate proprietary offerings can still generate revenue.
- Copyleft licenses can impose source-sharing conditions on some redistributed derivatives; exact consequences depend on the selected license and distribution/use model.

**Trade-off:** wider community contribution potential, but less control over third-party core modifications and redistribution.

### Option C — Source-available, custom licensed core
- Publish source but reserve modification/redistribution rights except as explicitly granted.
- Provide an express free commercial-use grant for the official app and a documented extension API.
- Clearly call this source-available/custom-licensed, not open source.

**Trade-off:** transparency without the full freedoms of open source; license clarity and contributor agreements become important.

Given the stated requirement that core edits need permission or a separate paid license, Option A or C is the closest fit. Make the final choice deliberately before accepting outside code contributions or releasing source.

## Extension and content policy

- Supported configuration files, themes, frames, stickers, and content packages should have documented schemas and version compatibility.
- If third-party extensions are supported, specify exactly which hooks are allowed and whether they are free to create, distribute, or sell.
- Do not treat an arbitrary executable/native plugin as equivalent to a declarative frame/theme package.
- A theme/frame package is a separate work from the player. Each package must carry creator attribution and its own license/usage permissions.
- Commercial permission to use the player does not imply permission to use copyrighted characters, logos, fonts, music, or frame assets.
- If the community can submit content, require an explicit license grant and takedown process.

## Support and revenue guardrails

- No hardware sales or bundled-hardware margin is part of the business model.
- No mandatory subscription is required to run the basic player.
- The primary intended revenue is optional paid support and authorized custom work.
- Frame/content payments or a creator marketplace are optional future possibilities, not a dependency for the initial business model.
- Cloud photo storage, if provided, needs a transparent cost/retention policy and should not become a forced fee for local-only users.

## Required decisions before public release

- Select Option A, B, or C.
- Decide whether source code will be public and under which exact license.
- Define commercial-use rights, redistribution, installation limits (if any), modification rights, trademarks/branding, and warranty/liability boundaries.
- Define third-party extension permissions and contributor terms.
- Define frame/theme creator license defaults.
- Have qualified counsel review the actual terms for intended jurisdictions, including Vietnam.
- Do not add a generic LICENSE file until the chosen license and rights grant are confirmed.

