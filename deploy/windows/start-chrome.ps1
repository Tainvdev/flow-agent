param(
    [string]$ChromePath = "",
    [string]$ProfileList = $env:FLOW_BROWSER_PROFILES,
    [string]$ProfileRoot = $env:FLOW_WINDOWS_PROFILE_ROOT
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$envFile = Join-Path $repoRoot ".env"

if (Test-Path -LiteralPath $envFile) {
    foreach ($line in Get-Content -LiteralPath $envFile) {
        if ([string]::IsNullOrWhiteSpace($ProfileList) -and $line -match '^FLOW_BROWSER_PROFILES=(.*)$') {
            $ProfileList = $Matches[1].Trim()
        }
        if ([string]::IsNullOrWhiteSpace($ProfileRoot) -and $line -match '^FLOW_WINDOWS_PROFILE_ROOT=(.*)$') {
            $ProfileRoot = $Matches[1].Trim()
        }
    }
}

if ([string]::IsNullOrWhiteSpace($ChromePath)) {
    $candidates = @(
        "$env:ProgramFiles\Google\Chrome\Application\chrome.exe",
        "$env:ProgramFiles(x86)\Google\Chrome\Application\chrome.exe",
        "$env:LOCALAPPDATA\Google\Chrome\Application\chrome.exe"
    )
    $ChromePath = $candidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}
if ([string]::IsNullOrWhiteSpace($ChromePath) -or -not (Test-Path -LiteralPath $ChromePath)) {
    throw "Không tìm thấy Google Chrome. Truyền -ChromePath hoặc cài Chrome trước."
}

if ([string]::IsNullOrWhiteSpace($ProfileList)) {
    $ProfileList = "profile-1,profile-2,profile-3"
}
if ([string]::IsNullOrWhiteSpace($ProfileRoot)) {
    $ProfileRoot = Join-Path $env:LOCALAPPDATA "FlowAgent\ChromeProfiles"
}

$extensionDir = (Resolve-Path (Join-Path $repoRoot "flow-extension")).Path
New-Item -ItemType Directory -Force -Path $ProfileRoot | Out-Null

$index = 0
foreach ($rawName in $ProfileList -split ',') {
    $name = $rawName.Trim()
    if ([string]::IsNullOrWhiteSpace($name) -or $name -notmatch '^[A-Za-z0-9._-]+$') {
        throw "Tên Chrome profile không hợp lệ: '$rawName'"
    }

    $profileDir = Join-Path $ProfileRoot $name
    New-Item -ItemType Directory -Force -Path $profileDir | Out-Null
    $positionX = ($index % 3) * 420
    $positionY = [math]::Floor($index / 3) * 360

    Write-Host "Starting Chrome profile $name"
    Write-Host "Loading Flow Bridge from $extensionDir"
    $arguments = @(
        "--user-data-dir=$profileDir",
        "--disable-extensions-except=$extensionDir",
        "--load-extension=$extensionDir",
        "--no-first-run",
        "--no-default-browser-check",
        "--password-store=basic",
        "--disable-features=Translate",
        "--window-size=800,650",
        "--window-position=$positionX,$positionY",
        "https://flow.google.com/"
    )
    Start-Process -FilePath $ChromePath -ArgumentList $arguments
    $index++
    Start-Sleep -Milliseconds 500
}
