@echo off
set "PATH=C:\msys64\ucrt64\bin;%PATH%"
go build -ldflags -H=windowsgui -o smpltkn.exe .
echo Build complete: smpltkn.exe
