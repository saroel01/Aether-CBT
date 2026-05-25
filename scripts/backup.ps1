# Aether CBT - Backup Database (PowerShell)
# Usage:
#   .\scripts\backup.ps1
#   .\scripts\backup.ps1 -Database "data/cbt_aether.db" -Output "backups"

param(
    [string]$Database = "data/cbt_aether.db",
    [string]$Output = "backups"
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   Aether CBT - Database Backup Tool    " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Cek apakah Go tersedia
try {
    $goVersion = go version
    Write-Host "Go ditemukan: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "ERROR: Go tidak ditemukan di PATH!" -ForegroundColor Red
    Write-Host "Silakan install Go atau jalankan dari environment yang sudah ada Go." -ForegroundColor Yellow
    exit 1
}

# Jalankan backup tool
Write-Host "Menjalankan backup tool..." -ForegroundColor Yellow
Write-Host ""

$backupScript = Join-Path $PSScriptRoot "backup.go"

try {
    go run $backupScript -db $Database -out $Output
    $exitCode = $LASTEXITCODE
} catch {
    Write-Host "ERROR: Gagal menjalankan backup tool: $_" -ForegroundColor Red
    exit 1
}

if ($exitCode -ne 0) {
    Write-Host ""
    Write-Host "Backup gagal dengan kode error $exitCode" -ForegroundColor Red
    exit $exitCode
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "Backup selesai dengan sukses." -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
