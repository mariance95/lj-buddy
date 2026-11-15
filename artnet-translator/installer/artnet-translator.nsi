; Instalador de LJ Buddy ArtNet Translator para Windows (NSIS 3).
; Compilar con:  makensis -DVERSION=1.0.0 installer/artnet-translator.nsi
; Si existe installer\drivers\CDM-Setup.exe (instalador del driver FTDI CDM,
; descargado de https://ftdichip.com/drivers/vcp-drivers/), se incluye como
; componente opcional.

Unicode true
!include "MUI2.nsh"

!ifndef VERSION
  !define VERSION "1.0.0"
!endif
!define APP "LJ Buddy ArtNet Translator"
!define EXE "lj-buddy-artnet-translator.exe"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\LJBuddyArtNetTranslator"
!define FTDI_URL "https://ftdichip.com/drivers/vcp-drivers/"

Name "${APP} ${VERSION}"
OutFile "..\dist\LJ-Buddy-ArtNet-Translator-Setup-${VERSION}.exe"
InstallDir "$PROGRAMFILES64\${APP}"
InstallDirRegKey HKLM "Software\LJBuddyArtNetTranslator" "InstallDir"
RequestExecutionLevel admin
SetCompressor /SOLID lzma

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${APP}"
VIAddVersionKey "FileDescription" "Instalador de ${APP}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "LegalCopyright" "MIT"

!define MUI_ICON "..\assets\icon.ico"
!define MUI_UNICON "..\assets\icon.ico"
!define MUI_ABORTWARNING
!define MUI_COMPONENTSPAGE_SMALLDESC
!define MUI_FINISHPAGE_RUN "$INSTDIR\${EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Iniciar ${APP} ahora"
!ifndef WITH_FTDI
  !define MUI_FINISHPAGE_LINK "Descargar el driver FTDI (solo si Windows no reconoce la interfaz)"
  !define MUI_FINISHPAGE_LINK_LOCATION "${FTDI_URL}"
!endif

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "Spanish"

Section "${APP} (programa)" SecMain
  SectionIn RO
  ; Cierra una versión en ejecución para poder reemplazar el ejecutable.
  nsExec::Exec 'taskkill /F /IM ${EXE}'
  Sleep 500
  SetOutPath "$INSTDIR"
  File "..\dist\windows\${EXE}"
  File "..\assets\icon.ico"
  File /oname=LEEME.txt "..\README.md"
  File /oname=LICENCIA.txt "..\..\LICENSE"
  WriteUninstaller "$INSTDIR\Desinstalar.exe"

  CreateDirectory "$SMPROGRAMS\${APP}"
  CreateShortcut "$SMPROGRAMS\${APP}\${APP}.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\icon.ico"
  CreateShortcut "$SMPROGRAMS\${APP}\Desinstalar ${APP}.lnk" "$INSTDIR\Desinstalar.exe"

  ; Regla de firewall: Art-Net entra por UDP 6454.
  nsExec::Exec 'netsh advfirewall firewall delete rule name="LJ Buddy ArtNet Translator"'
  nsExec::Exec 'netsh advfirewall firewall add rule name="LJ Buddy ArtNet Translator" dir=in action=allow protocol=UDP localport=6454 program="$INSTDIR\${EXE}" enable=yes'

  WriteRegStr HKLM "Software\LJBuddyArtNetTranslator" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayName" "${APP}"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\icon.ico"
  WriteRegStr HKLM "${UNINST_KEY}" "Publisher" "LJ Buddy"
  WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINST_KEY}" "UninstallString" '"$INSTDIR\Desinstalar.exe"'
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" 1
SectionEnd

Section "Acceso directo en el escritorio" SecDesktop
  CreateShortcut "$DESKTOP\${APP}.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\icon.ico"
SectionEnd

Section /o "Iniciar con Windows" SecAuto
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "LJBuddyArtNetTranslator" '"$INSTDIR\${EXE}" -no-browser'
SectionEnd

!ifdef WITH_FTDI
Section "Driver FTDI (Enttec Open DMX / USB Pro)" SecFTDI
  SetOutPath "$INSTDIR\drivers"
  File "drivers\CDM-Setup.exe"
  DetailPrint "Instalando el driver FTDI..."
  ExecWait '"$INSTDIR\drivers\CDM-Setup.exe"'
SectionEnd
!endif

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain} "El puente Art-Net a DMX y su panel de control."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Crea un icono en el escritorio."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecAuto} "Arranca en segundo plano (bandeja del sistema) al iniciar sesión."
  !ifdef WITH_FTDI
    !insertmacro MUI_DESCRIPTION_TEXT ${SecFTDI} "Driver oficial de FTDI para que Windows reconozca la interfaz USB-DMX."
  !endif
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  nsExec::Exec 'taskkill /F /IM ${EXE}'
  Sleep 500
  nsExec::Exec 'netsh advfirewall firewall delete rule name="LJ Buddy ArtNet Translator"'
  Delete "$INSTDIR\${EXE}"
  Delete "$INSTDIR\icon.ico"
  Delete "$INSTDIR\LEEME.txt"
  Delete "$INSTDIR\LICENCIA.txt"
  Delete "$INSTDIR\drivers\CDM-Setup.exe"
  RMDir "$INSTDIR\drivers"
  Delete "$INSTDIR\Desinstalar.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${APP}\${APP}.lnk"
  Delete "$SMPROGRAMS\${APP}\Desinstalar ${APP}.lnk"
  RMDir "$SMPROGRAMS\${APP}"
  Delete "$DESKTOP\${APP}.lnk"
  DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "LJBuddyArtNetTranslator"
  DeleteRegKey HKLM "${UNINST_KEY}"
  DeleteRegKey HKLM "Software\LJBuddyArtNetTranslator"
SectionEnd
