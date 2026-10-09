# Install one clif release into a directory you own: no administrator rights,
# no package manager, the same binary on every machine that pins the same
# version. The Windows twin of scripts/install.sh.
#
#   powershell -ExecutionPolicy Bypass -File scripts\install.ps1 [-Version 0.5.0] [-Dir <dir>]
#
# -Version  0.5.0, v0.5.0 or latest. Left out, the first of clif.version and
#           scripts\clif.version in the working directory names it, so a
#           project, its developers and its CI install one version from one
#           file; with neither, latest.
# -Dir      where clif.exe goes: $env:CLIF_INSTALL_DIR, else
#           %LOCALAPPDATA%\clif\bin. It is not added to PATH; the script says
#           how when it is not there already.
#
# CLIF_DOWNLOAD_URL replaces https://github.com/cfmleditor/clif/releases, for a
# mirror laid out the same way. The archive is checked against the release's
# checksums.txt when the release has one. A release's clif-with-cflint archive
# is preferred: it puts cflint.exe beside clif.exe, which clif lints with
# rather than downloading CFLint on first use. Earlier releases have only the
# clif archive, and releases from before the rename to clif only cfmleditor-lsp
# archives; one of those is installed as clif.exe. Windows on ARM runs the
# amd64 build.
[CmdletBinding()]
param(
    [string]$Version = "",
    [string]$Dir = ""
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# Windows PowerShell 5.1 on .NET Framework may still offer only TLS 1.0, which
# GitHub refuses.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

if (-not $Version) {
    foreach ($f in @('clif.version', 'scripts\clif.version')) {
        if (Test-Path $f) {
            $Version = (Get-Content $f -Raw).Trim()
            break
        }
    }
}
if (-not $Version) { $Version = 'latest' }

if (-not $Dir) {
    $Dir = if ($env:CLIF_INSTALL_DIR) { $env:CLIF_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'clif\bin' }
}

$releases = if ($env:CLIF_DOWNLOAD_URL) { $env:CLIF_DOWNLOAD_URL } else { 'https://github.com/cfmleditor/clif/releases' }
$base = if ($Version -eq 'latest') { "$releases/latest/download" } else { "$releases/download/v$($Version.TrimStart('v'))" }

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("clif-install-" + [System.Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    $archive = Join-Path $tmp 'archive.zip'
    # The archive asset, and the name of the binary inside it.
    $name = 'clif'
    $asset = ''
    foreach ($candidate in @('clif-with-cflint', 'clif', 'cfmleditor-lsp')) {
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$candidate-windows-amd64.zip" -OutFile $archive
            $asset = "$candidate-windows-amd64.zip"
            if ($candidate -eq 'cfmleditor-lsp') { $name = 'cfmleditor-lsp' }
            break
        } catch {
            continue
        }
    }
    if (-not $asset) { throw "clif install: no windows/amd64 archive for $Version at $base" }
    $sums = Join-Path $tmp 'checksums.txt'
    $haveSums = $true
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $sums
    } catch {
        $haveSums = $false
    }

    if ($haveSums) {
        $want = ''
        foreach ($line in Get-Content $sums) {
            $parts = $line -split '\s+', 2
            if ($parts.Count -eq 2 -and ($parts[1] -eq $asset -or $parts[1] -eq "*$asset")) { $want = $parts[0].ToLower() }
        }
        $got = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLower()
        if (-not $want -or $want -ne $got) {
            throw "clif install: $asset does not match the release's checksums.txt (want $want, got $got)"
        }
    } else {
        Write-Warning "clif install: $Version has no checksums.txt, so $asset was not verified"
    }

    Expand-Archive -Path $archive -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Path $Dir -Force | Out-Null
    $exe = Join-Path $Dir 'clif.exe'
    Copy-Item (Join-Path $tmp "$name.exe") $exe -Force

    $installed = & $exe version
    Write-Output "Installed $installed to $exe"

    # A cflint.exe this script did not put there is left alone; .clif-cflint
    # marks the one it did, which a later run replaces.
    $cflint = Join-Path $tmp 'cflint.exe'
    if (Test-Path $cflint) {
        $dest = Join-Path $Dir 'cflint.exe'
        $marker = Join-Path $Dir '.clif-cflint'
        if ((Test-Path $dest) -and -not (Test-Path $marker)) {
            Write-Warning "clif install: left the existing $dest in place; clif lints with it, since it is beside clif"
        } else {
            Copy-Item $cflint $dest -Force
            New-Item -ItemType File -Path $marker -Force | Out-Null
            Write-Output "Installed CFLint to $dest"
        }
    }

    $onPath = ($env:PATH -split ';') | Where-Object { $_.TrimEnd('\') -ieq $Dir.TrimEnd('\') }
    if (-not $onPath) {
        Write-Warning "clif install: $Dir is not on PATH. For this user, from now on: [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ';$Dir', 'User')"
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
