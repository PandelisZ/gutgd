@echo off
setlocal

set "MINGW_BIN=C:\msys64\mingw64\bin"

if not exist "%MINGW_BIN%\gcc.exe" (
  echo MSYS2 mingw64 gcc was not found at %MINGW_BIN%\gcc.exe 1>&2
  exit /b 1
)

if not exist "%MINGW_BIN%\g++.exe" (
  echo MSYS2 mingw64 g++ was not found at %MINGW_BIN%\g++.exe 1>&2
  exit /b 1
)

set "PATH=%MINGW_BIN%;%PATH%"
set "CGO_ENABLED=1"
set "CC=%MINGW_BIN%\gcc.exe"
set "CXX=%MINGW_BIN%\g++.exe"

if "%~1"=="" (
  exit /b 0
)

call %*
exit /b %ERRORLEVEL%
