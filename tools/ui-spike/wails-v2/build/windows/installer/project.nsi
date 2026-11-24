Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## PhotoBooth is an interactive-user application. Keep NSIS per-user even
## when a developer invokes `wails build -nsis` without an install-scope flag.
## An explicit non-user scope is rejected instead of silently creating a
## machine-wide installer.
!ifndef WAILS_INSTALL_SCOPE
  !define WAILS_INSTALL_SCOPE "user"
!endif
!if "${WAILS_INSTALL_SCOPE}" != "user"
  !error "PhotoBooth requires a per-user NSIS installer (WAILS_INSTALL_SCOPE=user)."
!endif
!ifndef REQUEST_EXECUTION_LEVEL
  !define REQUEST_EXECUTION_LEVEL "user"
!endif
!if "${REQUEST_EXECUTION_LEVEL}" != "user"
  !error "PhotoBooth NSIS installer must not request administrator elevation."
!endif

## The NSIS template is a Windows bootstrapper prototype for the per-user
## PhotoBooth installation. Override Wails' spike project name so the paths,
## shortcuts and uninstall metadata match the release executable/data model.
!ifndef INFO_PROJECTNAME
  !define INFO_PROJECTNAME "PhotoBoothNSISPreview"
!endif
!ifndef INFO_COMPANYNAME
  !define INFO_COMPANYNAME "PhotoBooth project"
!endif
!ifndef INFO_PRODUCTNAME
  !define INFO_PRODUCTNAME "PhotoBooth NSIS Preview"
!endif
!ifndef PRODUCT_EXECUTABLE
  !define PRODUCT_EXECUTABLE "PhotoBooth.exe"
!endif
!ifndef UNINST_KEY_NAME
  !define UNINST_KEY_NAME "PhotoBoothNSISPreview"
!endif

## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
# Install root is fixed under the current user's LOCALAPPDATA; no machine-wide path picker.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
  !endif
!else
  InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

# Reuse the existing per-user install root on upgrade.
InstallDirRegKey HKCU "Software\PhotoBooth\NSISPreviewInstaller" "InstallDir"

Function .onInit
   !insertmacro wails.checkArchitecture

   # Do not overwrite a Run value that is not owned by this install root.
   ReadRegStr $0 HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "PhotoBoothNSISPreview"
   ReadRegStr $1 HKCU "Software\PhotoBooth\NSISPreviewInstaller" "InstallDir"
   ${If} $0 != ""
      ${If} $1 == ""
         MessageBox MB_ICONSTOP "A PhotoBooth startup entry already exists but has no matching installer ownership record. Remove or investigate it before installing."
         Abort
      ${EndIf}
      StrCpy $2 "$\"$1\PhotoBooth.exe$\" --kiosk"
      ${If} $0 != $2
         MessageBox MB_ICONSTOP "The existing PhotoBooth startup entry was modified or points to another location. Refusing to overwrite it."
         Abort
      ${EndIf}
      ${If} $INSTDIR != $1
         MessageBox MB_ICONSTOP "PhotoBooth is already installed in a different per-user folder. Uninstall that copy before changing the install root."
         Abort
      ${EndIf}
   ${EndIf}
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller

    # Per-user ownership metadata and Customer Mode logon startup.
    WriteRegStr HKCU "Software\PhotoBooth\NSISPreviewInstaller" "InstallDir" "$INSTDIR"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "PhotoBoothNSISPreview" "$\"$INSTDIR\${PRODUCT_EXECUTABLE}$\" --kiosk"
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    # Remove only the PhotoBooth Run value that still targets this install.
    ReadRegStr $0 HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "PhotoBoothNSISPreview"
    StrCpy $1 "$\"$INSTDIR\${PRODUCT_EXECUTABLE}$\" --kiosk"
    ${If} $0 == $1
        DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "PhotoBoothNSISPreview"
    ${EndIf}

    ReadRegStr $0 HKCU "Software\PhotoBooth\NSISPreviewInstaller" "InstallDir"
    ${If} $0 == $INSTDIR
        DeleteRegKey HKCU "Software\PhotoBooth\NSISPreviewInstaller"
    ${EndIf}

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
