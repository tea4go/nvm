@echo off
set /P NVM_PATH="Enter the absolute path where the nvm-windows zip file is extracted/copied to: "
set NVM_HOME=%NVM_PATH%
set NVM_SYMLINK=%USERPROFILE%\.nvms\nodejs
setx NVM_HOME "%NVM_HOME%"
setx NVM_SYMLINK "%NVM_SYMLINK%"

echo PATH=%PATH% > %NVM_HOME%\PATH.txt

for /f "skip=2 tokens=2,*" %%A in ('reg query "HKCU\Environment" /v Path 2^>nul') do (
  setx PATH "%%B;%%NVM_HOME%%;%%NVM_SYMLINK%%"
)

if exist "%SYSTEMDRIVE%\Program Files (x86)\" (
set SYS_ARCH=64
) else (
set SYS_ARCH=32
)

if not exist "%USERPROFILE%\.nvms" mkdir "%USERPROFILE%\.nvms"

(echo root: %NVM_HOME% && echo path: %NVM_SYMLINK% && echo arch: %SYS_ARCH% && echo proxy: none) > %NVM_HOME%\settings.txt

notepad %NVM_HOME%\settings.txt
@echo on
