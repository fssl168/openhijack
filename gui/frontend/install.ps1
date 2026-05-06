$ErrorActionPreference = 'SilentlyContinue'
cd "d:\projects\openhijack\gui\frontend"
$env:NPM_CONFIG_USERCONFIG = ""
$env:NPM_CONFIG_GLOBALCONFIG = ""
npm install --prefer-offline --no-audit --no-fund 2>$null
exit 0
