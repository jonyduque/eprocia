@echo off
rem Atalho de duplo clique para o instalar.ps1 (só para o usuário atual).
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0instalar.ps1" %*
echo.
pause
