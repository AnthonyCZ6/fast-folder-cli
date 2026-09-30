; Asistente de instalación de fast-folder-cli para Windows (Inno Setup 6.7+ o 7.x).
;
; Compilar desde la raíz del repositorio, con los ejecutables en dist\:
;
;   ISCC.exe /DAppVersion=1.1.0 installer\fast-folder-cli.iss
;
; Genera dist\fast-folder-cli-setup.exe, que contiene los ejecutables x64 y ARM64
; e instala el adecuado para el equipo. La instalación es por usuario (no pide
; permisos de administrador) y usa la misma carpeta que install.ps1.
;
; Este archivo debe guardarse en UTF-8 con BOM para que Inno Setup lea bien los
; acentos.

#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
; Versión numérica para las propiedades del archivo: "1.2.0-rc.1" -> "1.2.0".
#define NumericVersion Copy(AppVersion, 1, Pos("-", AppVersion + "-") - 1)

#define AppName "fast-folder-cli"
#define AppExe "fast-folder-cli.exe"
#define AppPublisher "Anthony"
#define AppURL "https://github.com/AnthonyCZ6/fast-folder-cli"

[Setup]
; Identificador fijo de la aplicación: no debe cambiar nunca, así cada versión
; nueva actualiza la instalación anterior en lugar de duplicarla.
AppId={{FB5D8A51-AE89-4FF9-AEFC-A2EA71DD7C5A}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}/issues
AppUpdatesURL={#AppURL}/releases
AppCopyright=Copyright (c) 2026 {#AppPublisher}
VersionInfoVersion={#NumericVersion}
VersionInfoDescription={#AppName} - instalador

; Por usuario y sin administrador: {autopf} equivale a %LOCALAPPDATA%\Programs.
PrivilegesRequired=lowest
DefaultDirName={autopf}\{#AppName}
DisableProgramGroupPage=yes
MinVersion=10.0
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible

; Avisa a Windows de que cambió el PATH para que las terminales nuevas lo vean.
ChangesEnvironment=yes

LicenseFile=..\LICENSE
WizardStyle=modern
UninstallDisplayName={#AppName}
UninstallDisplayIcon={app}\{#AppExe}

OutputDir=..\dist
OutputBaseFilename=fast-folder-cli-setup
Compression=lzma2/max
SolidCompression=yes

[Languages]
; Solo español, igual que los mensajes de la propia herramienta.
Name: "es"; MessagesFile: "compiler:Languages\Spanish.isl"

[Messages]
FinishedLabelNoIcons=El asistente terminó de instalar [name] en su equipo.%n%nPara usarlo, abra una terminal nueva (PowerShell o Símbolo del sistema) y escriba:%n%n    fast --help%n%no, con el nombre completo, fast-folder-cli --help

[CustomMessages]
Options=Opciones:
AddToPath=Agregar fast-folder-cli al PATH (recomendado: permite usarlo desde cualquier terminal)
FastAlias=Crear el atajo "fast" (para escribir fast en lugar de fast-folder-cli)
OpenTerminal=Abrir una terminal para probar fast-folder-cli

[Tasks]
Name: "addtopath"; Description: "{cm:AddToPath}"; GroupDescription: "{cm:Options}"
Name: "fastalias"; Description: "{cm:FastAlias}"; GroupDescription: "{cm:Options}"

[Files]
Source: "..\dist\fast-folder-cli-windows-amd64.exe"; DestDir: "{app}"; DestName: "{#AppExe}"; Check: not IsArm64; Flags: ignoreversion
Source: "..\dist\fast-folder-cli-windows-arm64.exe"; DestDir: "{app}"; DestName: "{#AppExe}"; Check: IsArm64; Flags: ignoreversion
; Atajo "fast": copia del mismo ejecutable (Inno Setup la guarda una sola vez
; dentro del instalador). Funciona en cualquier terminal, a diferencia de un
; alias de PowerShell, y se actualiza y desinstala junto con el programa.
Source: "..\dist\fast-folder-cli-windows-amd64.exe"; DestDir: "{app}"; DestName: "fast.exe"; Check: not IsArm64; Tasks: fastalias; Flags: ignoreversion
Source: "..\dist\fast-folder-cli-windows-arm64.exe"; DestDir: "{app}"; DestName: "fast.exe"; Check: IsArm64; Tasks: fastalias; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; DestName: "LICENSE.txt"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[InstallDelete]
; Si al actualizar se desmarca el atajo, se elimina el que existía.
Type: files; Name: "{app}\fast.exe"; Tasks: not fastalias

[Run]
; Abre cmd con la carpeta ya en el PATH de esa ventana (los procesos lanzados
; por el instalador heredan su entorno, que aún no incluye el PATH nuevo).
Filename: "{cmd}"; Parameters: "/k ""set ""PATH={app};%PATH%"" && fast-folder-cli --help"""; Description: "{cm:OpenTerminal}"; Flags: postinstall nowait skipifsilent

[Code]
const
  EnvironmentKey = 'Environment';

function NormalizeDir(const Dir: string): string;
begin
  Result := Uppercase(RemoveBackslashUnlessRoot(Trim(Dir)));
end;

{ Devuelve la lista de rutas Paths (separadas por ';') sin las entradas que
  apuntan a Dir. El resto de entradas se conserva exactamente igual. }
function RemoveDirFromList(Paths: string; const Dir: string; var Removed: Boolean): string;
var
  Entry: string;
  P: Integer;
  First: Boolean;
begin
  Result := '';
  Removed := False;
  First := True;
  Paths := Paths + ';';
  while Paths <> '' do
  begin
    P := Pos(';', Paths);
    Entry := Copy(Paths, 1, P - 1);
    Delete(Paths, 1, P);
    if (Entry <> '') and (NormalizeDir(Entry) = NormalizeDir(Dir)) then
      Removed := True
    else
    begin
      if not First then
        Result := Result + ';';
      Result := Result + Entry;
      First := False;
    end;
  end;
end;

{ El PATH se lee y escribe sin expandir las variables (%USERPROFILE%...) para no
  alterar las demás entradas. Se guarda como REG_EXPAND_SZ, el tipo que Windows
  usa por defecto para el PATH. }
procedure AddDirToUserPath(const Dir: string);
var
  Paths: string;
  Found: Boolean;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    Paths := '';
  RemoveDirFromList(Paths, Dir, Found);
  if Found then
  begin
    Log('La carpeta ya estaba en el PATH: ' + Dir);
    Exit;
  end;
  if (Paths <> '') and (Paths[Length(Paths)] <> ';') then
    Paths := Paths + ';';
  if RegWriteExpandStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths + Dir) then
    Log('Carpeta agregada al PATH: ' + Dir)
  else
    Log('No se pudo actualizar el PATH del usuario');
end;

procedure RemoveDirFromUserPath(const Dir: string);
var
  Paths, NewPaths: string;
  Removed: Boolean;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    Exit;
  NewPaths := RemoveDirFromList(Paths, Dir, Removed);
  if Removed and RegWriteExpandStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', NewPaths) then
    Log('Carpeta quitada del PATH: ' + Dir);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    { Al actualizar, desmarcar la opción también quita la carpeta del PATH. }
    if WizardIsTaskSelected('addtopath') then
      AddDirToUserPath(ExpandConstant('{app}'))
    else
      RemoveDirFromUserPath(ExpandConstant('{app}'));
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    RemoveDirFromUserPath(ExpandConstant('{app}'));
end;
