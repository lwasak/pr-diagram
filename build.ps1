# build.ps1 — Builds the .NET Roslyn analyzer (win-x64 self-contained) and the Go CLI.
# Run from the project root: .\build.ps1
param(
    [string]$RID = "win-x64"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Write-Host "==> Publishing .NET analyzer ($RID)..." -ForegroundColor Cyan
dotnet publish analyzer/analyzer.csproj `
    -r $RID `
    --self-contained true `
    -c Release `
    -p:PublishSingleFile=true `
    -o analyzer/dist

if ($LASTEXITCODE -ne 0) { Write-Error "dotnet publish failed"; exit 1 }

Write-Host "==> Building Go binary..." -ForegroundColor Cyan
go build -o prdiagram.exe .

if ($LASTEXITCODE -ne 0) { Write-Error "go build failed"; exit 1 }

Write-Host ""
Write-Host "Build complete." -ForegroundColor Green
Write-Host "Run: .\prdiagram.exe --dir examples"
Write-Host "Dry-run: .\prdiagram.exe --dir examples --dry-run"
