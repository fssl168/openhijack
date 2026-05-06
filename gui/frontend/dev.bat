@echo off
cd /d "d:\projects\openhijack\gui\frontend"
set NPM_CONFIG_USERCONFIG=
set NPM_CONFIG_GLOBALCONFIG=
npm run dev -- --host
exit /b 0
