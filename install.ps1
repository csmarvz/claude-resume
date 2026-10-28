$ErrorActionPreference = 'Stop'
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$name = "claude-resume-windows-$arch.exe"
$base = 'https://github.com/csmarvz/claude-resume/releases/latest/download'
$dir = Join-Path $env:LOCALAPPDATA 'Programs\claude-resume'
New-Item -ItemType Directory -Force $dir | Out-Null
$tmp = Join-Path $dir 'claude-resume.exe.tmp'
$sums = Join-Path $dir 'checksums.txt'
Invoke-WebRequest "$base/$name" -OutFile $tmp -UseBasicParsing
Invoke-WebRequest "$base/checksums.txt" -OutFile $sums -UseBasicParsing
$want = Get-Content $sums | ForEach-Object { $f = -split $_; if ($f[1] -eq $name) { $f[0] } } | Select-Object -First 1
Remove-Item $sums
if (-not $want -or (Get-FileHash $tmp -Algorithm SHA256).Hash -ne $want) {
  Remove-Item $tmp
  throw 'The download does not match its checksum; nothing was installed.'
}
Move-Item -Force $tmp (Join-Path $dir 'claude-resume.exe')
Write-Host "Installed $dir\claude-resume.exe"
$path = (Get-Item 'HKCU:\Environment').GetValue('Path', '', 'DoNotExpandEnvironmentNames')
if (($path -split ';') -notcontains $dir) {
  $new = if ($path) { $path.TrimEnd(';') + ';' + $dir } else { $dir }
  New-ItemProperty 'HKCU:\Environment' -Name Path -Value $new -PropertyType ExpandString -Force | Out-Null
  [Environment]::SetEnvironmentVariable('CLAUDE_RESUME_INSTALL', '1', 'User')
  [Environment]::SetEnvironmentVariable('CLAUDE_RESUME_INSTALL', $null, 'User')
  Write-Host "Added $dir to your PATH; open a new terminal to use it."
}
