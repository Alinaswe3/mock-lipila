@echo off
REM Build script for Lipila Mock Payment Gateway (Windows)
REM Requires: Go 1.22+ and a C compiler (gcc) on PATH

setlocal

set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64

echo Building lipila-mock.exe ...
go build -ldflags="-s -w" -o lipila-mock.exe .

if %ERRORLEVEL% neq 0 (
    echo.
    echo Build failed. Make sure gcc is installed and on your PATH.
    echo Install MinGW/MSYS2/TDM-GCC if you haven't already.
    exit /b 1
)
           
echo.
echo Build successful: lipila-mock.exe
echo Run with: lipila-mock.exe
echo Admin UI: http://localhost:8080/admin/
