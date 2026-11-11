param(
 [ValidateSet('windows','darwin','linux')][string]$TargetOS='windows',
 [ValidateSet('amd64','arm64')][string]$Arch='amd64',
 [string]$OutputDirectory=(Join-Path $PSScriptRoot 'release')
)
$ErrorActionPreference='Stop'
$taskGo=(Get-Command go -ErrorAction Stop).Source
$env:CGO_ENABLED='0'
$env:GOOS=$TargetOS
$env:GOARCH=$Arch
$taskOutput=[IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $taskOutput -Force | Out-Null
$taskName='NgaReader'
if($TargetOS -eq 'windows'){$taskName+='.exe'}
$taskFlags='-s -w -X github.com/egoist/mygo.production=1'
if($TargetOS -eq 'windows'){$taskFlags='-H=windowsgui '+$taskFlags}
Push-Location -LiteralPath $PSScriptRoot
try {
 & $taskGo build -trimpath -tags mygo_noinspector -ldflags $taskFlags -o (Join-Path $taskOutput $taskName) .
 if($LASTEXITCODE -ne 0){throw 'Build failed'}
 Write-Output (Join-Path $taskOutput $taskName)
} finally {Pop-Location}
