# ==============================================================================
# Aether CBT - Automated Production Build & Release Script
# ==============================================================================
# This script automates:
# 1. Cleaning old build artifacts
# 2. Compiling the SvelteKit frontend to static assets
# 3. Compiling the Go backend to an optimized executable
# 4. Packaging the production-ready application into a single zip file
# ==============================================================================

$ErrorActionPreference = "Stop"
$StartTime = Get-Date

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "       Aether CBT - Automated Production Build Tool       " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# ------------------------------------------------------------------------------
# 1. Environment Verification
# ------------------------------------------------------------------------------
Write-Host ""
Write-Host "[1/5] Memeriksa dependensi sistem..." -ForegroundColor Yellow

if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Error "Error: Go compiler (go) tidak ditemukan. Silakan pasang Go 1.22+."
}
if (-not (Get-Command "npm" -ErrorAction SilentlyContinue)) {
    Write-Error "Error: Node.js/npm tidak ditemukan. Silakan pasang Node.js 18+."
}

$GoVersion = (go version).Split(" ")[2]
Write-Host "[OK] Go compiler ditemukan ($GoVersion)" -ForegroundColor Green
$NpmVersion = (npm -v)
Write-Host "[OK] Node.js/npm ditemukan (v$NpmVersion)" -ForegroundColor Green

# ------------------------------------------------------------------------------
# 2. Cleanup Old Artifacts
# ------------------------------------------------------------------------------
Write-Host ""
Write-Host "[2/5] Membersihkan sisa build lama..." -ForegroundColor Yellow

$ReleaseDir = "release"
$ZipFile = "aether-cbt-release.zip"

if (Test-Path $ReleaseDir) {
    Remove-Item -Path $ReleaseDir -Recurse -Force
    Write-Host "[OK] Folder '$ReleaseDir' lama berhasil dibersihkan" -ForegroundColor Gray
}
if (Test-Path $ZipFile) {
    Remove-Item -Path $ZipFile -Force
    Write-Host "[OK] Berkas '$ZipFile' lama berhasil dibersihkan" -ForegroundColor Gray
}
if (Test-Path "web/build") {
    Remove-Item -Path "web/build" -Recurse -Force
    Write-Host "[OK] Folder 'web/build' lama berhasil dibersihkan" -ForegroundColor Gray
}

# ------------------------------------------------------------------------------
# 3. Build SvelteKit Frontend
# ------------------------------------------------------------------------------
Write-Host ""
Write-Host "[3/5] Memulai kompilasi Frontend (SvelteKit)..." -ForegroundColor Yellow

Push-Location "web"
try {
    # Pastikan dependensi terpasang jika belum ada
    if (-not (Test-Path "node_modules")) {
        Write-Host "Menyiapkan dependensi frontend (npm install)..." -ForegroundColor Gray
        npm install
    }
    
    # Jalankan build
    npm run build
}
finally {
    Pop-Location
}

if (-not (Test-Path "web/build/index.html")) {
    Write-Error "Error: Kompilasi frontend gagal. Berkas 'web/build/index.html' tidak terbentuk."
}
Write-Host "[OK] Kompilasi Frontend berhasil! Aset ditulis ke 'web/build'" -ForegroundColor Green

# ------------------------------------------------------------------------------
# 4. Build Go Backend
# ------------------------------------------------------------------------------
Write-Host ""
Write-Host "[4/5] Memulai kompilasi Backend (Go)..." -ForegroundColor Yellow

$ExeName = "aether-cbt.exe"
Write-Host "Mengompilasi Go binary dengan optimasi ukuran..." -ForegroundColor Gray
go build -ldflags="-s -w" -o $ExeName cmd/server/main.go

if (-not (Test-Path $ExeName)) {
    Write-Error "Error: Kompilasi backend gagal. Berkas '$ExeName' tidak terbentuk."
}
Write-Host "[OK] Kompilasi Backend berhasil! Berkas '$ExeName' siap digunakan." -ForegroundColor Green

# ------------------------------------------------------------------------------
# 5. Packaging Release Bundle
# ------------------------------------------------------------------------------
Write-Host ""
Write-Host "[5/5] Mengemas aplikasi ke paket siap pakai (Release ZIP)..." -ForegroundColor Yellow

# Buat direktori release baru
New-Item -ItemType Directory -Force -Path $ReleaseDir | Out-Null
New-Item -ItemType Directory -Force -Path "$ReleaseDir/web" | Out-Null

# Salin Executable dan Aset Frontend
Copy-Item -Path $ExeName -Destination "$ReleaseDir/"
Copy-Item -Path "web/build" -Destination "$ReleaseDir/web/" -Recurse

# Salin folder data dan seeder contoh secara opsional jika ingin disertakan langsung
if (Test-Path "data") {
    # Buat folder data kosong untuk database
    New-Item -ItemType Directory -Force -Path "$ReleaseDir/data" | Out-Null
}

# Kompresi folder release menjadi berkas ZIP tunggal
Write-Host "Mengompresi berkas menjadi '$ZipFile'..." -ForegroundColor Gray
Compress-Archive -Path "$ReleaseDir/*" -DestinationPath $ZipFile -Force

# Bersihkan file executable sementara di root
Remove-Item -Path $ExeName -Force

Write-Host ""
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "           PROSES KOMPILASI SELESAI DENGAN SUKSES         " -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
$ElapsedTime = (Get-Date) - $StartTime
$TotalSec = [Math]::Round($ElapsedTime.TotalSeconds, 1)
Write-Host "Total Waktu : $TotalSec detik" -ForegroundColor Gray
Write-Host "Paket Rilis : $ZipFile" -ForegroundColor Cyan
Write-Host "Isi Folder  : Ekstrak ZIP -> Klik ganda 'aether-cbt.exe' untuk menjalankan!" -ForegroundColor Gray
Write-Host "==========================================================" -ForegroundColor Green
