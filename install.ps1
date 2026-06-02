$ErrorActionPreference = "Stop"

$Repo = "Ksschkw/driftlock"
$BinName = "driftlock"
$DefaultPrefix = "$env:LOCALAPPDATA\Programs\driftlock"

Write-Host "[driftlock-install] Starting..." -ForegroundColor Green

# detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$Target = "${BinName}-windows-${Arch}.exe"

# fetch latest release tag
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Tag = $release.tag_name
if (-not $Tag) { throw "Could not determine latest release tag." }

$DownloadUrl = "https://github.com/$Repo/releases/download/$Tag/$Target"

# download
$TempDir = Join-Path $env:TEMP "driftlock-install-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
$Installer = Join-Path $TempDir $Target

Write-Host "Downloading $Target from $DownloadUrl ..."
Invoke-WebRequest -Uri $DownloadUrl -OutFile $Installer

# optional checksum
$ChecksumUrl = "$DownloadUrl.sha256"
try {
    $ChecksumFile = Join-Path $TempDir "$Target.sha256"
    Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumFile
    $ExpectedHash = (Get-Content $ChecksumFile).Split()[0]
    $ActualHash = (Get-FileHash -Path $Installer -Algorithm SHA256).Hash
    if ($ExpectedHash -ne $ActualHash) {
        throw "Checksum mismatch! Expected $ExpectedHash, got $ActualHash"
    }
    Write-Host "Checksum verified." -ForegroundColor Green
} catch {
    Write-Host "Could not verify checksum (or file missing). Continuing..." -ForegroundColor Yellow
}

# install
$Prefix = if ($env:DRIFTLOCK_PREFIX) { $env:DRIFTLOCK_PREFIX } else { $DefaultPrefix }
if (-not (Test-Path $Prefix)) { New-Item -ItemType Directory -Force -Path $Prefix | Out-Null }
$ExePath = Join-Path $Prefix "$BinName.exe"
Copy-Item -Path $Installer -Destination $ExePath -Force

Write-Host "Driftlock installed to $ExePath" -ForegroundColor Green

# ensure in PATH
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$Prefix*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$Prefix", "User")
    $env:Path += ";$Prefix"
    Write-Host "Added $Prefix to user PATH." -ForegroundColor Green
}

Remove-Item -Recurse -Force $TempDir
Write-Host "Done. Restart your terminal, then run 'driftlock init' inside a Git repo." -ForegroundColor Green