# Build script untuk Aether CBT Password Generator (Production Tool)
#
# Hasil: aether-password-generator.exe (bisa dijalankan tanpa Go)
#
# Cara pakai:
#   .\scripts\build-password-generator.ps1

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   Build Aether Password Generator      " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$source = "cmd/password-generator/main.go"
$outputDir = "build"
$outputName = "aether-password-generator.exe"

# Buat folder build jika belum ada
if (-not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

Write-Host "Membangun executable untuk Windows..." -ForegroundColor Yellow

# Build untuk Windows (amd64)
$env:GOOS = "windows"
$env:GOARCH = "amd64"

go build -ldflags="-s -w" -o "$outputDir/$outputName" $source

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build gagal!" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "✅ Build berhasil!" -ForegroundColor Green
Write-Host "File hasil: $outputDir\$outputName" -ForegroundColor Cyan
Write-Host ""
Write-Host "File ini bisa disertakan dalam setiap rilis Aether CBT." -ForegroundColor Yellow
Write-Host "Admin sekolah cukup double-click untuk menjalankannya." -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Cyan
