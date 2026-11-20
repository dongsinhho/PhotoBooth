# DS-RX1 virtual printer smoke

Windows-only, synthetic-data smoke for the isolated GDI printer output target.

## What it checks

- Opens an explicitly named Windows printer queue.
- Uses the queue's current default media/form settings unless `-media-form` is supplied; optional form and orientation are applied only to the current job through a driver-provided DEVMODE.
- Sends a generated 1200x1800 color/alignment JPEG through the Windows GDI printer path.
- Creates a disposable manifest-v3 session with frame snapshot, synthetic captures and rendered.jpg.
- Wraps the Windows GDI target in output.DurablePrinter and dispatches it with local save through output.Coordinator.
- Verifies that the local saved photo is byte-identical to the source fixture.
- Verifies the durable PrintAttempt is persisted as queued with the returned spooler job ID, unique attempt ID and session-scoped document name.
- Performs a read-only EnumJobsW lookup using the exact persisted queue, document name and job ID; it records whether the job is still present but does not infer a physical-print outcome when absent.
- Observes one newly created JPEG from the configured Multi File Port Monitor and validates its dimensions and SHA-256.
- Verifies the durable local-output/print coordination while keeping physical_print_confirmed=false.

The smoke does not open or inspect pre-existing output files. It snapshots filenames before each job and only checks the newly appearing JPEG. JSON reports store the local output filename and hash, not the ephemeral TEMP path that is removed after the smoke.

## Run on the configured virtual queue

From the repository root in PowerShell:

~~~powershell
go run ./tools/printer-poc/dsrx1-virtual-smoke -printer DS-RX1 -media-form "(6x4)" -output-dir "C:\Users\<user>\OneDrive\<folder>\Printer" -copies 1 -landscape=true -flip-vertical=true -expected-width 1200 -expected-height 1800 -report "$env:TEMP\photobooth-dsrx1-virtual-smoke.json"
~~~

Use the actual Multi File Port Monitor output folder configured for the queue. The queue must already exist and be idle. Do not point this test at a physical printer when you intend a virtual-only test: submitting a job to a physical queue can produce paper output.

For the tested ShinPc queue, Landscape=true plus FlipVertical=true matches the custom virtual .prn converter. This is a queue/converter-specific result. The general WindowsPrinter option defaults are not forced globally; do not copy this transform to a physical DS-RX1 device without a physical orientation test.

## Output contract

The printer target returns queued only after GDI EndDoc succeeds and includes the job ID returned by StartDocW. This confirms that Windows accepted the spool job; it does not confirm that physical paper emerged. The tool reports physical_print_confirmed=false deliberately.

No queue defaults, registry, driver, port monitor, station configuration or Wails Settings are changed by the smoke. The selected queue's existing port monitor may run its configured post-print command. Inspect and trust that local command before running a test; the DS-RX1 virtual test on ShinPc used a local converter that transformed DNP raster data to JPEG and did not send data over the network.

## Current limitations

- This is not a model-specific physical printer certification or a color-calibration test.
- When `-media-form` is omitted, media size is inherited from the Windows queue defaults. The optional form name must exactly match the installed driver's DEVMODEW form name and is validated by DocumentPropertiesW before a job is submitted.
- FlipVertical is an explicit per-queue compatibility option. The virtual converter on ShinPc requires it; a physical print path may not.
- StartDocW/EndDoc are synchronous Win32 calls. Context cancellation is checked between calls but cannot forcibly interrupt a driver call while it is blocked.
- The Windows Preview has an opt-in StoreConfig v2 path to LocalProvider + DurablePrinter. Printing remains disabled by default; the root CLI remains local-only, demo sessions are local-only and local-output retries never submit a printer job. Kernel-verified lock reconciliation and operator resolution are implemented. Physical printer/media acceptance remains pending.
- A queued job ID is not a durable physical-print completion record. Submitting/uncertain attempts and stale locks must never be retried automatically. Use the document name to inspect the Windows queue before any manual decision.
