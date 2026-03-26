@echo off
setlocal

pushd "%~dp0" >nul

where go >nul 2>&1
if errorlevel 1 (
    echo.
    echo  Go no esta disponible en PATH.
    echo  Instala Go o abre una terminal con Go configurado.
    echo.
    popd >nul
    exit /b 1
)

echo.
echo  Compilando proyecto Go en %CD%
echo.

go build -o mcp-filesystem-server.exe .
if errorlevel 1 (
    echo.
    echo  La compilacion ha fallado.
    echo.
    popd >nul
    exit /b 1
)

echo.
echo  Compilacion completada correctamente.
echo.

popd >nul
exit /b 0