# ADR 0004: Free Commercial Use and Paid Support

- **Status:** Product direction accepted; exact software license is not finalized
- **Date:** 2026-10-24
- **Decision owner:** Product owner

## Context

The product is intended to lower the initial cost for photobooth operators by letting them use hardware they already own. The creator will not sell camera/computer/printer bundles. The app itself is intended to be free for commercial booth use, while technical support and authorized custom core changes can be paid services.

The product should be flexible, but flexibility should primarily come through configuration, theme/frame packages, and documented extension contracts.

## Decision

1. Basic official player use for commercial photobooth operation will have no software usage fee under the eventual license terms.
2. The standard product should support self-installation and self-service without purchasing support.
3. Optional paid services may include installation, troubleshooting, training, camera/printer integration, migration, consulting, and authorized core modifications.
4. Store customization through documented settings and content packages is a supported product capability and should not require core source modifications.
5. Under the intended controlled-core model, modifying the core or distributing derivative/modified builds requires prior authorization or a separate license, subject to final legal review.
6. The product must not be advertised as OSI open source if the chosen license forbids modifications or derivative works without permission.
7. The exact licensing model (freeware/closed source, source-available custom license, or genuine open source) must be chosen and reviewed before software source code is released or outside code contributions are accepted.

## Consequences

### Positive
- Operators can self-install and earn revenue using their existing hardware.
- Revenue can come from optional expertise rather than hardware sales or mandatory app fees.
- Supported configuration and frame formats provide controlled customization without fragmenting the core.
- Core changes remain governed by an explicit authorization/licensing process if a controlled-core license is selected.

### Trade-offs
- A custom/controlled core license is not OSI open source, even if the software is free and the repository is public.
- Restricting core modifications reduces community contribution and independent forks.
- Terms must clearly distinguish configuration/content changes from source-code modifications.
- Legal review is needed to define redistribution, derivative works, warranties, third-party contributions, and jurisdiction-specific terms.

## Alternatives considered

### Genuine open-source core
Would provide modification and redistribution freedoms under an OSI-approved license, but conflicts with a requirement that core changes need prior permission. It remains an alternative if community code contributions become the top priority.

### Mandatory paid software license
Rejected as the intended default because the product's positioning is to reduce initial software cost.

### Hardware-bundled product
Rejected because the creator does not plan to sell cameras, computers, printers, or full booth kits.

