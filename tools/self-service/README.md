# Self-service diagnostics

These scripts are read-only helpers for a local checkout. They do not open the camera, start the player, write the station profile, create sample photos, delete files, or upload diagnostics.

## Inspect the reference/sample environment

From the repository root:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\inspect-environment.ps1
~~~

Check Windows Plug-and-Play camera names without opening the camera:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\inspect-environment.ps1 -CheckCamera
~~~

Inspect a different profile without changing it:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\inspect-environment.ps1 -ConfigPath "$env:LOCALAPPDATA\PhotoBooth\store-config-v1.json" -CheckCamera
~~~

The checker validates:
- Go availability/version.
- Station config JSON schema version, supported local/free modes and shot range.
- Frame JSON existence, schema version and equality between configured shot count and slot count.
- Whether configured session/output directories already exist (it does not create them).
- Optional Windows PnP camera names. PnP presence is not proof that Media Foundation can stream or capture.

Exit code is 1 when a required config/tool check fails. Missing storage directories and an unchecked camera are warnings, not automatic failures.

## Other self-service resources

- [Windows Quickstart](../../docs/QUICKSTART_WINDOWS.md)
- [Windows Troubleshooting](../../docs/troubleshooting/WINDOWS_TROUBLESHOOTING.md)
- [Compatibility report template](../../docs/COMPATIBILITY_REPORT_TEMPLATE.md)
- [Camera compatibility matrix](../../docs/knowledge-base/CAMERA_COMPATIBILITY_MATRIX.md)
- [Camera POC instructions](../camera-poc/README.md)

Do not attach guest photos, webcam frames, rendered strips, credentials, payment screenshots, tokens, serial numbers, or unredacted personal paths to issue reports.
