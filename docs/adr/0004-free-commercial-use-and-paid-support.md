# ADR 0004: Free Commercial Use, App-Level Customization, and Paid Support

- **Status:** Product model accepted; exact legal terms are not finalized
- **Date:** 2026-10-25
- **Decision owner:** Product owner

## Context

The product aims to reduce the upfront cost for photobooth operators by letting them use equipment they already own. The project will not sell cameras, computers, printers, or bundled booth kits. The official application should be free for commercial booth use, while customization should be available at the application/content layer rather than through editing core code.

## Decision

1. The official player is intended to be free to use for commercial photobooth sessions.
2. Operators may self-install and use it without buying support.
3. Users may change supported settings and create/install supported frame/theme content at the application layer.
4. Ordinary end-user distribution does not grant access to or permission to modify the core source code.
5. Editing core source, creating modified core builds, rebranding or redistributing modified core requires prior written permission or a separate license from the rights holder, subject to legal review.
6. Optional paid services may include setup, troubleshooting, training, device integration, migration, and authorized custom core changes.
7. The project must not be described as OSI open-source software under this controlled-core model because the intended terms restrict core modification/derived-build redistribution.
8. Final license terms must be reviewed before source code is made available, external code contributions are accepted, or commercial distribution begins.

## Consequences

### Positive
- Booth owners can earn money using the official software without a per-session/per-booth usage fee.
- They can adjust exposed settings and visual content without programming.
- Core changes remain governed by an explicit authorization/separate-license process.
- Revenue can come from expertise without selling hardware or requiring a support contract.

### Trade-offs
- This is controlled/free-to-use software, not OSI open source.
- End users cannot independently modify and redistribute the core under the intended model.
- Supported configuration and frame/theme formats must be flexible enough for common store needs.
- Exact rights and restrictions require qualified legal review.

## Alternatives considered

### Genuine open-source core
Would allow users to modify and redistribute derivative works under the chosen open-source license, conflicting with the current requirement that core modifications/redistribution require permission.

### Mandatory paid app license
Rejected as the intended default because the product should be free for commercial booth usage.

### Hardware-bundled product
Rejected because the project does not plan to sell hardware kits.
