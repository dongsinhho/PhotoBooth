# Printer Selection and Acceptance Plan

**Status:** Prepared; no physical printer selected or accepted. This document is a decision aid, not a claim of printer support.

## Current station evidence - 2026-11-18

The original read-only inventory on 2026-11-18 found only Microsoft Print to PDF and OneNote queues. Subsequent operator setup installed a DNP DS-RX1 x64 software driver and created a virtual-only DS-RX1 queue. A later read-only inventory confirmed its port is Photobooth-port/Multi File Port Monitor, not a USB/network-attached physical device. No physical printer or media kit is available on ShinPc; physical acceptance remains PENDING.

The internal/booth/output package now has a Windows GDI target and DurablePrinter wrapper, and the Wails Windows Preview can construct LocalProvider + DurablePrinter when an operator explicitly enables printer settings in StoreConfig v2. The root CLI remains local-only; Wails demo sessions and local-output retry remain local-only. Synthetic/virtual queue evidence is not physical-device evidence. See [OUTPUT_TARGETS.md](OUTPUT_TARGETS.md) and [ADR 0010](../adr/0010-independent-output-targets.md).

## Candidate comparison - public listings checked 2026-11-18

| Candidate | Evidence and fit | Remaining uncertainty |
|---|---|---|
| **HiTi P525L - preferred first evaluation candidate (Proposed)** | Dye-sublimation; USB 2.0; 4x6, 5x7, 6x8 and 2x6 strip formats; manufacturer rates 12.8s for 4x6 and 20.5s for 6x8, with 500 4x6 or 250 6x8 prints per roll. Vietnam listings observed around 19.85-22.5M VND, depending on seller/bundle. A HiTi China download portal lists a 5.68MB open-source Linux driver archive for P520L/P525L/P720L/P728L/P750L. | Confirm exact Windows 11 driver with the seller/manufacturer; build and test the Linux filter/PPD on Ubuntu 26.04.1; confirm consumable SKU, real per-print cost, local warranty/service and new-unit availability. The Linux download listing is not proof of Ubuntu compatibility. |
| **DNP DS620A - faster Windows-oriented alternative (Proposed)** | Dye-sublimation; USB 2.0; 4x6, 5x7 and 6x8; manufacturer rates 8.3s for 4x6 and 15.7s for 6x8, with 400 4x6 or 200 6x8 prints per roll. Vietnam listings observed around 33.55-34.95M VND. DNP's current download catalog lists a DS620 Windows 11 driver. | Higher observed purchase price; confirm exact DS620A hardware revision/driver, media cost and local service. Linux support was not established in this review. |

**Recommendation, not a product decision:** evaluate HiTi P525L first if the reference setup should balance cost, booth-friendly 2x6/4x6/6x8 formats, and a possible Linux path. Prefer DNP DS620A if faster throughput and a clearly listed Windows 11 driver outweigh the higher cost. Do not buy either solely from this document; request a live demo and written confirmation of driver/media/warranty.

## Selection gate

Before implementation, record the operator-selected exact model, new/used condition, connection (start with direct USB), Windows driver version, firmware version, paper/ribbon kit SKU and target print formats. Confirm the kit and consumables can be replenished in Viet Nam at an acceptable per-print cost. Obtain or borrow the physical unit for acceptance. A seller listing or source-driver archive alone does not close this gate.

For a future Linux path, require the vendor's exact model-specific driver/PPD and a successful build/install/print on the selected Ubuntu 26.04.1 x86_64 target. Do not infer this from a Windows driver, a PPD alone, WSL2, or a cross-compile.

## First physical acceptance path

1. **Identify and install:** record printer model, firmware, USB connection, media kit, driver version, Windows edition/build and architecture. Install only a vendor-provided driver; confirm the queue and printer-ready state before PhotoBooth integration.
2. **Print fixture:** use a synthetic, non-guest test image with color patches, neutral grays, fine text, sharp edges, border guides and a known aspect ratio. Preserve the fixture and its SHA-256; do not use guest photos.
3. **Format checks:** after selecting the intended formats, print at least one 4x6 and one 6x8 (and a 2x6 strip layout if that is part of the chosen kit). Verify physical dimensions, orientation, crop, safe margins, cut, finish, color consistency and time-to-paper. Record observations; set numeric tolerances only after the exact printer/media are selected.
4. **Repeatability:** print 10 consecutive fixture copies at the normal booth cadence. Record job acceptance, spool/job IDs where available, time-to-first-print, average interval, jams/errors and media count before/after. No unexplained duplicate, missing, truncated or mis-cut print is acceptable for the initial gate.
5. **Independent outputs:** run one synthetic session with local save and printing enabled. Verify each target reports independently: printer failure must not remove the local saved photo; local-save failure must not silently suppress a configured print attempt.
6. **Failure and recovery:** separately test printer unavailable before enqueue, USB disconnect while idle, paper/ribbon exhausted or cover-open using the vendor's safe procedure, spooler/job cancellation, and recovery. Never auto-resubmit an ambiguous job: a queued spool job is not proof that paper physically emerged.
7. **Offline:** repeat the normal capture/render/local-save/print path with Internet disconnected. No cloud, LAN, QR or payment dependency is permitted.

## Implementation boundary

Follow ADR 0010: printing is a separate output.Target consuming the immutable rendered source. Return queued only when the OS/vendor spooler has accepted the job and retain an external job ID when available. Do not report physical completion without reliable device confirmation. Keep local saving independent. Printer retry/duplicate policy must be designed against the selected driver's actual job semantics; do not add a generic blind retry loop.

## Sources checked 2026-11-18

- HiTi P525L product specifications: https://www.hiti.com/Product/Product.aspx?MenuID=2&lang=EN
- HiTi P525L Wi-Fi/PC driver FAQ: https://www.hiti.com/Support/FAQ_detail.aspx?ID=2&QID=32&lang=EN
- HiTi China download portal, including the listed Linux open-source package: https://hitichengyan.com/?a=index&c=Lists&m=home&tid=15
- HiTi P525L Vietnam listings: https://mayinvanphong.com.vn/san-pham/may-in-anh-giay-nhiet-hiti-p525l-in-kho-10x15cm-13x18cm-15x20cm-giao-tiep-usb-2-0-wifi/ ; https://toanphat.com/may-in-anh-giay-nhiet-hiti-p525l-in-kho-10x15cm-13x18cm-15x20cm-giao-tiep-usb-20-wifi/p6614.html ; https://sphoto.vn/hiti
- DNP DS620A product specifications: https://www.dnpphoto.com/products/printers/ds620a
- DNP Asia downloads, listing the DS620 Windows 11 driver: https://www.dnpimagingcomm.asia/downloads?tabid=RX1+HS
- DNP DS620 Vietnam listings: https://simpletech.vn/products/may-in-dnp-dp-ds620-chinh-hang ; https://sphoto.vn/may-in-anh-dnp-ds620-chinh-hang

Prices and stock are seller listings, not quotes; reconfirm VAT, bundle contents, warranty, media SKU and availability before purchase.

**Current blocker:** no selected/available physical printer on ShinPc. Until the operator selects and supplies a model, keep the physical printer adapter and physical acceptance tests unimplemented/pending.

## DNP DS-RX1 Windows virtual queue test — 2026-11-19

The operator installed the DNP DS-RX1 x64 Windows driver and created a queue named DS-RX1 on ShinPc. It is not connected to a physical printer: the queue's port is Photobooth-port using the Multi File Port Monitor, which writes DNP raster data to a local folder and invokes a local raster-to-JPEG converter.

The installed driver exposes 6x4/4x6/6x8/5x7 media options, color, 300x300 and 300x600 dpi and Glossy/Matte controls. The queue is using Windows winprint/RAW. The actual queue/port was verified in a read-only inventory; no queue, port, driver or registry settings were changed.

A synthetic orientation/color fixture was sent through four GDI+ job variants. For this specific virtual converter, Landscape=true plus a vertical-only source flip produced the correctly oriented 1200x1800 JPEG. A Go Windows GDI output target was then implemented and exercised:
- 10 consecutive printer-only jobs: 10/10 accepted by the Windows spooler, 10/10 converted into 1200x1800 JPEGs; all converted files had the same SHA-256.
- 10 consecutive local-plus-printer Coordinator jobs: 10/10 local copies matched the source fixture byte-for-byte, 10/10 printer jobs returned distinct queued job IDs, and 10/10 virtual conversions produced the expected JPEG dimensions and identical output hashes.
- Queue ended Normal with zero jobs. No physical paper was produced.

The Go target is now wired into the Wails Windows Preview as an opt-in local-plus-durable-printer dispatcher. StoreConfig v2 remains disabled by default. The adapter applies a configured media form and orientation per job through DEVMODEW; the tested FlipVertical option belongs only to the current virtual converter chain and must not be reused for physical hardware without verifying orientation.

See [DS-RX1 virtual smoke README](../../tools/printer-poc/dsrx1-virtual-smoke/README.md) and the current [OUTPUT_TARGETS.md](OUTPUT_TARGETS.md).

**Physical acceptance remains blocked:** this setup proves Windows GDI -> driver -> spooler -> configured virtual port/converter -> JPEG and the local/print output coordination contract. It does not prove USB communication, paper feed/cut, media handling, physical color, throughput, error state reporting, or actual prints. Re-run the physical acceptance sequence only after a real printer and media kit are connected.


## StoreConfig v2 reservation (2026-11-19)

The local station profile supports an operator-managed printer section: printer.enabled (defaults false), printer.queue_name, printer.media_form, printer.orientation (portrait or landscape) and printer.transform.flip_vertical (defaults false). Queue/form are optional while disabled and required when enabled. media_form is the exact driver DEVMODEW.dmFormName and must fit 31 UTF-16 code units. Schema-v1 profiles migrate in memory to schema v2; the source file is only rewritten on an explicit Save.

This does not select or certify a physical printer, install a physical device or close acceptance. The Wails Windows Preview constructs LocalProvider + DurablePrinter only when an operator explicitly enables the profile. The guest Settings UI does not expose printer controls, demo sessions remain local-only, local-output retry does not submit a printer job, and the root CLI remains local-only. See STORE_CONFIG.md.


## Windows Preview opt-in output integration and DEVMODE form smoke (2026-11-20)

The Wails Windows Preview now consumes the operator-managed StoreConfig v2 printer block when `printer.enabled=true`. It constructs an independent `LocalProvider + DurablePrinter` coordinator for real-camera sessions only. The default profile remains `printer.enabled=false`. Demo sessions remain local-only, the root CLI remains local-only, and `RetryPhotoOutput` / `RetryRecoveredPhotoOutput` use a dedicated local-only path that cannot submit a printer job. The result screen reports local-save status separately from Windows spooler acceptance and does not claim physical paper output.

The Windows GDI adapter now accepts an optional exact driver form name through `DEVMODEW.dmFormName` on each job, preserving the driver-private DEVMODE tail and asking `DocumentPropertiesW` to validate the result. `media_form` must fit 31 UTF-16 code units. No queue defaults or registry values are changed by this path.

A read-only inspection of the DS-RX1 virtual driver/Windows Forms registry found exact form names including `(6x4)`, `(6x8)`, `(5x7)`, `(5x3.5)` and `PR (4x6)`. These are names exposed by this installed virtual driver, not a universal media-name contract for other printers.

One synthetic virtual smoke was run with queue `DS-RX1`, form `(6x4)`, landscape=true and the converter-specific vertical flip. GDI/DocumentPropertiesW accepted the per-job form; Windows returned spooler job ID `32`; the durable manifest recorded one queued attempt; local output matched the source SHA-256; and the Multi File Port Monitor converted the job to a 1200x1800 JPEG. Exact queue inspection after conversion returned Found=false (the job was no longer listed), and physical_print_confirmed remained false. Queue status returned Normal with no remaining jobs. The new 81,081-byte JPEG was verified and then removed; all six pre-existing output files were checked and preserved. No physical paper was produced.

**Current gate status:** Windows Preview software integration is implemented and virtual-queue validated. Physical printer selection/acceptance remains **PENDING / DEFERRED — no physical hardware or media kit available**. Do not treat this smoke as evidence for physical orientation, color, dimensions, cut, throughput, paper-out or disconnect behavior. Keep the default profile disabled until hardware acceptance is possible.
