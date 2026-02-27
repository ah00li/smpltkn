@echo off
set "PATH=C:\msys64\ucrt64\bin;%PATH%"

if "%~1"=="" goto usage
if "%~1"=="-tokens" goto set_tokens
if "%~1"=="-show" goto show
if "%~1"=="-gui" goto gui

:usage
echo Usage: %0 [-tokens R/L] [-show] [-gui]
echo   -tokens 85000/100000  - Set token limit (remaining/limit)
echo   -show                 - Show current values
echo   -gui                  - Launch widget
exit /b 1

:set_tokens
shift
token_widget.exe -tokens %1
goto :eof

:show
token_widget.exe -show
goto :eof

:gui
start token_widget.exe
goto :eof
