@echo off
set "PATH=C:\msys64\ucrt64\bin;%PATH%"
go build -ldflags -H=windowsgui -o token_widget.exe .
echo Build complete: token_widget.exe
