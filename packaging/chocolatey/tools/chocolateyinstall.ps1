# The URL and checksum are filled in by the release workflow
# (package-managers.yml) from the release's clif-windows-amd64.zip.
$ErrorActionPreference = 'Stop'
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition

$packageArgs = @{
  packageName    = $env:ChocolateyPackageName
  unzipLocation  = $toolsDir
  url64bit       = '__URL__'
  checksum64     = '__CHECKSUM__'
  checksumType64 = 'sha256'
}

# Only an amd64 build is published. Windows on ARM runs it under emulation.
Install-ChocolateyZipPackage @packageArgs

# Chocolatey shims clif.exe on its own. The name before the rename is shimmed
# too, for editor extensions that still look for cfmleditor-lsp on PATH.
Install-BinFile -Name 'cfmleditor-lsp' -Path (Join-Path $toolsDir 'clif.exe')
