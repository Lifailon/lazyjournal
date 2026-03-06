$OS = $([System.Environment]::OSVersion).VersionString
$ARCH = $env:PROCESSOR_ARCHITECTURE.ToLower()

Write-Host "System: " -NoNewline
Write-Host $OS -ForegroundColor Green
Write-Host "Architecture: " -NoNewline
Write-Host $ARCH -ForegroundColor Green

$binPath = "$env:LOCALAPPDATA\lazyjournal"
$configPath = "$HOME\.config\lazyjournal"

if (!(Test-Path $binPath)) {
    New-Item -Path $binPath -ItemType Directory | Out-Null
    Write-Host "Directory created: " -NoNewline
    Write-Host $binPath -ForegroundColor Blue
}

if (!(Test-Path $configPath)) {
    New-Item -Path $configPath -ItemType Directory | Out-Null
    Write-Host "Directory created: " -NoNewline
    Write-Host $configPath -ForegroundColor Blue
}

$beforeEnvPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (!($($beforeEnvPath).Split(";") -contains $binPath)) {
    $afterEnvPath = $beforeEnvPath + ";$binPath"
    [Environment]::SetEnvironmentVariable("Path", $afterEnvPath, "User")
    Write-Host "The path has been added to the Path environment variable for the current user."
}

$GITHUB_LATEST_VERSION = (Invoke-RestMethod "https://api.github.com/repos/Lifailon/lazyjournal/releases/latest").tag_name
if ($null -ne $GITHUB_LATEST_VERSION) {
    $urlDownload = "https://github.com/Lifailon/lazyjournal/releases/download/$GITHUB_LATEST_VERSION/lazyjournal-$GITHUB_LATEST_VERSION-windows-$ARCH.exe"
    Invoke-RestMethod -Uri $urlDownload -OutFile "$binPath\lazyjournal.exe"
    Invoke-RestMethod -Uri https://raw.githubusercontent.com/Lifailon/lazyjournal/refs/heads/main/config.yml -OutFile "$configPath\config.yml"
    Write-Host "✔  Installation completed " -NoNewline
    Write-Host "successfully" -ForegroundColor Green -NoNewline
    Write-Host " in " -NoNewline
    Write-Host "$binPath\lazyjournal.exe" -ForegroundColor Blue -NoNewline
    Write-Host " (version:" -NoNewline
    Write-Host " $GITHUB_LATEST_VERSION" -ForegroundColor Green -NoNewline
    Write-Host ") and configuration in" -NoNewline
    Write-Host " $binPath\config.yml" -ForegroundColor Blue
    Write-Host "To launch the interface from anywhere, re-login to the current session"
} else {
    Write-Host "Error. " -ForegroundColor Red -NoNewline
    Write-Host "Unable to get the latest version from GitHub repository, check your internet connection."
}
