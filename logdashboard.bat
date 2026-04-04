@echo off
setlocal

set PORT=8091
set LOGDIR=C:\mcp-logs

:: Check if port is in use
netstat -ano | findstr ":%PORT% " | findstr "LISTENING" >nul 2>&1
if %errorlevel%==0 (
    echo.
    echo  Puerto %PORT% esta ocupado.
    echo.
    for /f "tokens=5" %%p in ('netstat -ano ^| findstr ":%PORT% " ^| findstr "LISTENING"') do (
        set PID=%%p
    )
    for /f "tokens=1" %%n in ('tasklist /fi "pid eq %PID%" /fo csv /nh 2^>nul ^| findstr /v "INFO"') do (
        set PNAME=%%n
    )
    echo  Proceso: %PNAME% (PID: %PID%^)
    echo.
    choice /c SN /m "  Matar proceso y continuar? (S/N)"
    if errorlevel 2 (
        echo  Cancelado.
        pause
        exit /b 1
    )
    taskkill /f /pid %PID% >nul 2>&1
    if %errorlevel%==0 (
        echo  Proceso terminado.
    ) else (
        echo  No se pudo matar el proceso. Intenta ejecutar como administrador.
        pause
        exit /b 1
    )
    timeout /t 1 /nobreak >nul
)

echo.
echo  Iniciando dashboard en http://localhost:%PORT%
echo  Logs: %LOGDIR%
echo  Presiona Ctrl+C para detener.
echo.

start http://localhost:%PORT%
"%~dp0logdashboard.exe" -log-dir "%LOGDIR%"

endlocal
