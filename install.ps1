$ErrorActionPreference = 'Stop'
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$dir = Join-Path $env:LOCALAPPDATA 'Programs\claude-resume'
New-Item -ItemType Directory -Force $dir | Out-Null
Invoke-WebRequest "https://github.com/csmarvz/claude-resume/releases/latest/download/claude-resume-windows-$arch.exe" -OutFile (Join-Path $dir 'claude-resume.exe')
Write-Host "Installed $dir\claude-resume.exe"
$path = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($path -split ';') -notcontains $dir) {
  [Environment]::SetEnvironmentVariable('Path', "$path;$dir", 'User')
  Write-Host "Added $dir to your PATH; open a new terminal to use it."
}
