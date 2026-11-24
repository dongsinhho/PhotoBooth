Unicode true
!include "MUI2.nsh"

!ifndef PHOTOBOOTH_PACKAGE_FILE
  !error "PHOTOBOOTH_PACKAGE_FILE must point to a versioned PhotoBooth ZIP."
!endif
!ifndef PHOTOBOOTH_CHECKSUM_FILE
  !error "PHOTOBOOTH_CHECKSUM_FILE must point to the adjacent .sha256 file."
!endif
!ifndef PHOTOBOOTH_INSTALL_SCRIPT
  !error "PHOTOBOOTH_INSTALL_SCRIPT is required."
!endif
!ifndef PHOTOBOOTH_UNINSTALL_SCRIPT
  !error "PHOTOBOOTH_UNINSTALL_SCRIPT is required."
!endif
!ifndef PHOTOBOOTH_RELEASE_COMMON
  !error "PHOTOBOOTH_RELEASE_COMMON is required."
!endif
!ifndef PHOTOBOOTH_VERSION
  !error "PHOTOBOOTH_VERSION is required."
!endif
!ifndef PHOTOBOOTH_OUTPUT_FILE
  !error "PHOTOBOOTH_OUTPUT_FILE is required."
!endif

!define PRODUCT_NAME "PhotoBooth"
!define SETUP_ROOT "$LOCALAPPDATA\Programs\PhotoBooth Setup"
!define APP_ROOT "$LOCALAPPDATA\Programs\PhotoBooth"
!define SETUP_REG_KEY "Software\PhotoBooth\Bootstrapper"
!define UNINSTALL_REG_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\PhotoBooth"

Name "${PRODUCT_NAME} ${PHOTOBOOTH_VERSION}"
OutFile "${PHOTOBOOTH_OUTPUT_FILE}"
InstallDir "${SETUP_ROOT}"
InstallDirRegKey HKCU "${SETUP_REG_KEY}" "SetupRoot"
RequestExecutionLevel user
SetCompressor /SOLID lzma
CRCCheck on
ShowInstDetails show
ShowUninstDetails show
BrandingText "PhotoBooth - per-user installer"

!define MUI_ABORTWARNING
!define MUI_ICON "..\..\ui-spike\wails-v2\build\windows\icon.ico"
!define MUI_UNICON "..\..\ui-spike\wails-v2\build\windows\icon.ico"
!define MUI_WELCOMEPAGE_TITLE "Install PhotoBooth"
!define MUI_WELCOMEPAGE_TEXT "PhotoBooth installs for the current Windows user. The application remains local-first. A first installation separately requests administrator approval to apply the machine-wide SMB default-off security baseline."
!define MUI_FINISHPAGE_TEXT "PhotoBooth has been installed for the current Windows user. It will start in Customer Mode after sign-in."
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Function .onInit
  SetShellVarContext current
FunctionEnd

Section "Install PhotoBooth" SEC_INSTALL
  SetOutPath "$PLUGINSDIR"
  File /oname=PhotoBooth-release.zip "${PHOTOBOOTH_PACKAGE_FILE}"
  File /oname=PhotoBooth-release.zip.sha256 "${PHOTOBOOTH_CHECKSUM_FILE}"
  File /oname=Install-PhotoBoothWindowsPackage.ps1 "${PHOTOBOOTH_INSTALL_SCRIPT}"
  File /oname=Release.Common.ps1 "${PHOTOBOOTH_RELEASE_COMMON}"

  DetailPrint "Validating and activating the immutable PhotoBooth release..."
  !ifdef PHOTOBOOTH_ALLOW_UNSIGNED_PREVIEW
    ExecWait '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoLogo -NoProfile -ExecutionPolicy Bypass -File "$PLUGINSDIR\Install-PhotoBoothWindowsPackage.ps1" -PackagePath "$PLUGINSDIR\PhotoBooth-release.zip" -InstallRoot "${APP_ROOT}" -AllowUnsignedPreview' $0
  !else
    ExecWait '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoLogo -NoProfile -ExecutionPolicy Bypass -File "$PLUGINSDIR\Install-PhotoBoothWindowsPackage.ps1" -PackagePath "$PLUGINSDIR\PhotoBooth-release.zip" -InstallRoot "${APP_ROOT}"' $0
  !endif
  ${If} $0 != 0
    MessageBox MB_ICONSTOP|MB_OK "PhotoBooth installation failed (exit code $0). The application was not activated. Review the details and retry after resolving the reported blocker."
    Abort
  ${EndIf}

  SetOutPath "$INSTDIR\scripts"
  File /oname=Uninstall-PhotoBoothWindowsPackage.ps1 "${PHOTOBOOTH_UNINSTALL_SCRIPT}"
  File /oname=Release.Common.ps1 "${PHOTOBOOTH_RELEASE_COMMON}"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  WriteRegStr HKCU "${SETUP_REG_KEY}" "SetupRoot" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_REG_KEY}" "DisplayName" "PhotoBooth"
  WriteRegStr HKCU "${UNINSTALL_REG_KEY}" "DisplayVersion" "${PHOTOBOOTH_VERSION}"
  WriteRegStr HKCU "${UNINSTALL_REG_KEY}" "Publisher" "PhotoBooth project"
  WriteRegStr HKCU "${UNINSTALL_REG_KEY}" "InstallLocation" "${APP_ROOT}"
  WriteRegStr HKCU "${UNINSTALL_REG_KEY}" "UninstallString" '$\"$INSTDIR\uninstall.exe$\"'
  WriteRegDWORD HKCU "${UNINSTALL_REG_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_REG_KEY}" "NoRepair" 1

  DetailPrint "PhotoBooth active release: ${PHOTOBOOTH_VERSION}"
  DetailPrint "Operator data remains under %LOCALAPPDATA%\PhotoBooth."
SectionEnd

Function un.onInit
  SetShellVarContext current
FunctionEnd

Section "Uninstall"
  IfFileExists "${APP_ROOT}\active.json" 0 app_already_removed
    DetailPrint "Removing PhotoBooth binaries and release history..."
    ExecWait '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoLogo -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\scripts\Uninstall-PhotoBoothWindowsPackage.ps1" -InstallRoot "${APP_ROOT}" -Confirm:$$false' $0
    ${If} $0 != 0
      MessageBox MB_ICONSTOP|MB_OK "PhotoBooth could not be uninstalled (exit code $0). The setup files were kept so you can close the app or resolve the blocker and retry."
      Abort
    ${EndIf}
  app_already_removed:

  ReadRegStr $0 HKCU "${SETUP_REG_KEY}" "SetupRoot"
  ${If} $0 == $INSTDIR
    DeleteRegKey HKCU "${SETUP_REG_KEY}"
  ${EndIf}
  DeleteRegKey HKCU "${UNINSTALL_REG_KEY}"
  RMDir /r "$SMPROGRAMS\PhotoBooth"
  RMDir /r "$INSTDIR"
SectionEnd
