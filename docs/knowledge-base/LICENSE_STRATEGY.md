# Licensing Strategy

**Status:** Product model accepted; legal terms still require review
**Last updated:** 2026-10-25

## Product and business model

PhotoBooth is software-only. The project does not sell cameras, computers, printers, cabinets, or bundled booth hardware.

The intended offer:
- The official application is free to use for commercial photobooth operations.
- Operators may self-install and self-operate without purchasing support.
- Users may customize the application only through supported application-level controls: settings/configuration, frame/theme packages, and documented content formats.
- The standard distribution does not grant the right to modify the core source code or distribute a modified core build.
- Core source modification, derivative/modified builds, rebranding or redistribution of modified core require prior written permission or a separate license from the rights holder, subject to the final reviewed terms.
- Optional paid services include setup, troubleshooting, training, camera/printer integration, migration, consulting, and authorized custom core work.

This file records product intent. It is not the final end-user license agreement.

## What users may customize

The player should expose appropriate settings and documented extension formats for capabilities such as:
- Countdown duration and number of shots.
- Delays and state timing within the supported workflow.
- Frame/theme selection and visual assets.
- Photo-slot layout, output dimensions, and supported print/export settings.
- Store-specific branding and other settings the product deliberately exposes.

These are supported application-level configuration/content changes. They do not grant permission to edit, patch, reverse engineer, or redistribute the core software. A setting can only change behavior the application has implemented and documented.

Users may create their own frame/theme content for the supported format. Separate permissions for distributing/selling that content, and creator-submitted asset licensing, must be defined in the content terms.

## Core and distribution rights

- End-user distribution is intended to consist of official application builds and documentation; core source code is not part of the ordinary end-user customization workflow.
- Access to core source, modification rights, derivative builds, rebranding, and redistribution are not granted by a public repository or by permission to use the official application.
- If the rights holder later grants source access, it must be under explicit written terms that define the permitted changes and whether/how modified builds may be used or distributed.
- The precise legal effect and enforceability of modification/redistribution restrictions, exceptions, and anti-circumvention wording must be assessed by qualified counsel.

## Terminology

Do not advertise the core as **Open Source** under the intended controlled-core model. The Open Source Initiative states that open-source licenses permit modification and derived works and do not restrict commercial use:
- [Open Source Definition](https://opensource.org/osd)
- [OSI FAQ](https://opensource.org/faq)

Accurate positioning, subject to legal review:
- "Free to use for commercial photobooth operations."
- "Configurable photobooth software with supported frame and theme customization."
- "Optional paid support and authorized custom engineering."

"Free to use" describes price and grant of use; it does not automatically mean open-source, public-domain, or unrestricted redistribution.

## Support and revenue

- Support is optional; self-installation and normal supported configuration must remain viable.
- Primary planned revenue is support and authorized custom engineering, not software usage fees or hardware sales.
- Do not deliberately cripple normal configuration to force support purchases.
- Cloud delivery, paid content, and creator marketplace revenue are optional future models, not prerequisites for local operation.

## Decisions required before public software release

- Have qualified counsel review and finalize the user-facing license/terms.
- Define permitted commercial use, copying/installation, reverse engineering, core modification, modified-build redistribution, trademarks, warranties, liability, and termination.
- Define the support service terms separately from the free-use grant.
- Define content creator terms, third-party assets, and rights to redistribute frames/themes.
- Confirm that every third-party dependency allows the intended distribution model.
- Do not publish a generic or improvised license file before those rights are reviewed and confirmed.
