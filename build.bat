@echo off
SETLOCAL

echo [1/2] Cleaning old artifacts...
if exist cnc-led-display.exe del cnc-led-display.exe
if exist cnc-led-display del cnc-led-display

echo [2/2] Building Go application...
go build -o cnc-led-display.exe cnc_coords.go

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo ERROR: Build failed!
    exit /b %ERRORLEVEL%
)

echo.
echo SUCCESS: cnc-led-display.exe is ready.
echo.
pause
ENDLOCAL
