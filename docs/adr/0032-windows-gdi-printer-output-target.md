# ADR 0032: Windows GDI printer output target

**Status:** Proposed for production adoption; implemented and exercised as an isolated Windows Preview technical spike
**Date:** 2026-11-19

## Context

PhotoBooth's output coordinator supports independent local-save and printer targets. LocalProvider is implemented, but the repository had no Windows printer adapter. The operator installed a DNP DS-RX1 Windows x64 driver queue on ShinPc. The queue is virtual and routes to a local Multi File Port Monitor and raster-to-JPEG converter; no physical printer is connected.

The printer API must pass the rendered image through the installed driver. Sending a JPEG or PNG byte stream through the RAW spooler path would bypass the driver's normal rasterization and vendor-specific handling.

## Decision for the technical spike

Implement an isolated Windows-only output.Target using the Windows GDI printer path:

1. Open the named local Windows printer queue.
2. Request the driver's complete DEVMODE buffer using DocumentPropertiesW, preserving the vendor-specific private bytes.
3. Change job-scoped orientation fields and merge them through DocumentPropertiesW; do not persist or mutate queue defaults.
4. Create a WINSPOOL device context and send the rendered JPEG through StartDocW, StartPage, StretchDIBits, EndPage and EndDoc.
5. Return TargetQueued with the job ID returned by StartDocW only after EndDoc succeeds.
6. Keep local save as a separate coordinator target. A printer failure must not roll back a successful local save.
7. Keep vertical flip as an explicit per-target option. Never infer a physical printer transform from a virtual port converter.

Implementation: internal/booth/output/printer.go, printer_windows.go and printer_other.go. Smoke tool: tools/printer-poc/dsrx1-virtual-smoke.

## Validation

On 2026-11-19, the ShinPc DS-RX1 virtual queue was tested with synthetic color/alignment images:
- The Windows queue and driver were present, and the port was a local Multi File Port Monitor, not USB hardware.
- The correct transform for this configured virtual converter was Landscape=true and FlipVertical=true.
- Ten consecutive jobs through the printer target were accepted by the Windows spooler and converted to 1200x1800 JPEGs.
- A second ten-job run through output.Coordinator verified 10/10 local copies byte-identical to the source, 10/10 queued printer results with distinct Windows job IDs, and 10/10 converted JPEGs with identical output hashes.
- Queue finished Normal with no pending jobs.
- No physical paper was produced. No guest photos were used.

See docs/knowledge-base/OUTPUT_TARGETS.md and tools/printer-poc/dsrx1-virtual-smoke/README.md.

## Not yet accepted for production use

This adapter is not wired into StoreConfig, delivery.NewDispatcher or Wails Settings. Do not enable it for guest sessions until all of the following are designed and tested:

- Explicit printer-enabled/queue/form/orientation settings with safe defaults and schema migration.
- A durable print-intent record before spool submission, plus persistence of the returned spool job ID and target status. This first slice is now implemented as the isolated DurablePrinter wrapper and manifest-v3 PrintAttempts history (ADR 0033), but is not yet wired into the standard delivery factory or guest workflow.
- Conservative crash recovery for the window between spooler acceptance and durable result persistence. Manifest submitting/uncertain state and an exclusive crash lock now fail closed; no ambiguous job is blindly retried.
- Operator-visible queued/failed/uncertain status is surfaced in the read-only recovery panel. Automated spooler reconciliation and a manual operator resolution workflow remain open; there is deliberately no print retry action yet.
- A bounded policy for synchronous Windows driver calls. StartDocW/EndDoc can block and cannot be forcibly interrupted by the context once inside a driver call.
- Physical acceptance with the selected printer/media: layout/crop, color, dimensions, cut, repeatability, offline operation, paper-out/cover-open and disconnect behavior.

The adapter uses the queue's current default media form. It does not yet set a named paper form. The DS-RX1 virtual queue's vertical flip is a converter-specific workaround and must not be applied by default to physical USB printing. Native Ubuntu/Linux printing remains a separate driver/provider evaluation.

## Consequences

- The current Preview gains a tested, reusable Windows spooler adapter without external Go dependencies.
- Job ID plus queued status means Windows accepted the job; it does not prove the printer physically completed it.
- Local save and print remain independent targets.
- Production integration remains deferred until printer settings/defaults, automated spooler reconciliation, operator resolution of uncertain jobs and physical hardware acceptance are complete. Durable intent and crash-safe no-auto-retry behavior are implemented separately in ADR 0033.

## References

- Microsoft Learn: [DEVMODEW](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/ns-wingdi-devmodew)
- Microsoft Learn: [DocumentPropertiesW](https://learn.microsoft.com/en-us/windows/win32/printdocs/documentproperties)
- Microsoft Learn: [StartDocW](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-startdocw)
- Microsoft Learn: [GDI+ Printing](https://learn.microsoft.com/en-us/windows/win32/gdiplus/-gdiplus-printing-use)


## Durable submission follow-up (2026-11-19)

Durable attempt persistence and read-only recovery presentation are now implemented in ADR 0033. The DS-RX1 virtual smoke was upgraded to submit through DurablePrinter and verified one v3 session with queued status, unique document name and job ID 29. This does not enable the target in StoreConfig or the normal guest-session delivery factory. Automated Windows spooler reconciliation, explicit user resolution and physical printer acceptance remain outstanding.
