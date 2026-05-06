$ErrorActionPreference = 'SilentlyContinue'
cd "d:\projects\openhijack\gui\frontend"
$env:NPM_CONFIG_USERCONFIG = ""
$env:NPM_CONFIG_GLOBALCONFIG = ""
npm run build 2>$null
exit $LASTEXITCODE
