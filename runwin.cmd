@echo off
chcp 65001 >nul
cls

echo ====================================
echo 请使用go 18.0版本...
echo 请提前安装Deno...
echo powershell内执行安装指令:
echo "irm https://deno.land/install.ps1 | iex"
echo ====================================

echo ====================================
echo 删除旧的nvm.exe...
echo ====================================
del .\bin\nvm.exe  >nul 2>nul


echo ====================================
echo Building nvm.exe...
echo ====================================

cd .\src
if errorlevel 1 (
    echo Error: Failed to change directory to .\src
    exit /b 1
)

go build -o ..\bin\nvm.exe
if errorlevel 1 (
    echo Error: Go build failed
    cd ..
    exit /b 1
)

echo Build successful!
echo.

cd ..
echo ====================================
echo Running build.js with Deno...
echo ====================================

deno run --allow-read --allow-write --allow-run build.js
if errorlevel 1 (
    echo Error: Deno script failed
    exit /b 1
)

echo.
echo ====================================
echo All tasks completed successfully!
echo ====================================
