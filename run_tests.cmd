@echo off
echo Ejecutando tests del filesystem server...
cd /d "C:\MCPs\clone\mcp-filesystem-server"
echo.
echo === TESTS DE FILESYSTEMSERVER ===
go test ./filesystemserver -v -timeout=60s
echo.
echo === TESTS COMPLETADOS ===
pause
