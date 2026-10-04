@echo off
rem Busca una carpeta con fast-folder-cli y entra en ella (Simbolo del sistema).
rem Uso: fcd [termino] [opciones de fast-folder-cli]
rem
rem Se instala junto a fast-folder-cli.exe. Un archivo .cmd se ejecuta dentro
rem del mismo cmd.exe que lo llama, por eso puede cambiar su carpeta actual.
rem La carpeta elegida llega en un archivo temporal en UTF-8, que se lee con la
rem pagina de codigos 65001 para conservar los acentos.
rem
rem Este archivo debe contener solo caracteres ASCII: cmd.exe lo vuelve a leer
rem despues de cambiar la pagina de codigos.
setlocal
set "FCD_FILE=%TEMP%\fcd-%RANDOM%%RANDOM%.txt"
"%~dp0fast-folder-cli.exe" --cd-file "%FCD_FILE%" %*
set "FCD_EXIT=%ERRORLEVEL%"
if not exist "%FCD_FILE%" goto end

for /f "tokens=2 delims=:." %%c in ('chcp') do set "FCD_CP=%%c"
chcp 65001 >nul
set "FCD_DIR="
set /p FCD_DIR=<"%FCD_FILE%"
chcp %FCD_CP% >nul
del "%FCD_FILE%" 2>nul
if not defined FCD_DIR goto end

endlocal & cd /d "%FCD_DIR%"
exit /b 0

rem Sin carpeta elegida, termina con el codigo de fast-folder-cli (por
rem ejemplo, 2 si las opciones no son validas).
:end
endlocal & exit /b %FCD_EXIT%
