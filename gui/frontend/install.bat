@echo off
cd /d "d:\projects\openhijack\gui\frontend"
set NPM_CONFIG_USERCONFIG=
set NPM_CONFIG_GLOBALCONFIG=
npm install --prefer-offline --no-audit --no-fund 2>nul
exit /b 0
