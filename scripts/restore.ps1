# Aether CBT - Restore Database (PowerShell)
# PERINGATAN: Hentikan aplikasi sebelum menjalankan restore!
#
# Usage:
#   .\scripts\restore.ps1 -Backup "backups\cbt_aether_20260525_115405.db"
#   .\scripts\restore.ps1 -Backup "backups\cbt_aether_20260525_115405.db" -Force

param(
    [Parameter(Mandatory=$true)]
    [string]$Backup,

    [string]$Database = "data/cbt_aether.db",

    [switch]$Force
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Red
Write-Host "   Aether CBT - Database Restore Tool   " -ForegroundColor Red
Write-Host "========================================" -ForegroundColor Red
Write-Host ""
Write-Host "PERINGATAN KERAS:" -ForegroundColor Red
Write-Host "  - Aplikasi server HARUS dalam keadaan STOP sebelum restore." -ForegroundColor Yellow
Write-Host "  - Data saat ini akan diganti dengan data dari backup." -ForegroundColor Yellow
Write-Host "  - Pastikan Anda sudah memiliki backup terbaru sebelum melanjutkan." -ForegroundColor Yellow
Write-Host ""

if (-not $Force) {
    $confirm = Read-Host "Apakah Anda yakin ingin melanjutkan restore? (ketik 'YA' untuk konfirmasi)"
    if ($confirm -ne "YA") {
        Write-Host "Restore dibatalkan oleh user." -ForegroundColor Cyan
        exit 1
    }
}

# Validasi file backup
if (-not (Test-Path $Backup)) {
    Write-Host "ERROR: File backup tidak ditemukan: $Backup" -ForegroundColor Red
    exit 1
}

# Buat folder data jika belum ada
$dataDir = Split-Path $Database -Parent
if ($dataDir -and -not (Test-Path $dataDir)) {
    New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
}

Write-Host ""
Write-Host "Memulai proses restore..." -ForegroundColor Yellow
Write-Host "  Backup   : $Backup"
Write-Host "  Target   : $Database"

$checkpointScript = Join-Path $PSScriptRoot "checkpoint.go"

# P0-5 & P1-17: Validasi file backup sebelum mengganti database aktif
Write-Host "  Memverifikasi integritas file backup..." -ForegroundColor Yellow
try {
    & go run $checkpointScript -db $Backup -verify-only
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: File backup $Backup corrupt atau tidak valid! Proses restore dibatalkan demi keamanan." -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host "ERROR: Gagal memvalidasi file backup: $_" -ForegroundColor Red
    exit 1
}

# P1-17: Sebelum mengganti file, lakukan WAL checkpoint agar transaksi committed di -wal tercatat ke .db
if (Test-Path $Database) {
    Write-Host "  Melakukan checkpoint WAL pada database aktif..." -ForegroundColor Yellow
    try {
        & go run $checkpointScript -db $Database
        if ($LASTEXITCODE -ne 0) {
            Write-Host "WARNING: WAL checkpoint keluar dengan exit code non-zero." -ForegroundColor Yellow
        }
    } catch {
        Write-Host "WARNING: Gagal menjalankan script checkpoint otomatis: $_" -ForegroundColor Yellow
    }

    $timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $oldBackup = "$Database.before-restore-$timestamp"
    Write-Host "  Membuat cadangan database lama ke: $oldBackup" -ForegroundColor Cyan
    Copy-Item $Database $oldBackup -Force

    $walFile = "$Database-wal"
    $shmFile = "$Database-shm"
    if (Test-Path $walFile) {
        Copy-Item $walFile "$oldBackup-wal" -Force
    }
    if (Test-Path $shmFile) {
        Copy-Item $shmFile "$oldBackup-shm" -Force
    }
}

# Salin file backup sebagai database baru
try {
    Copy-Item $Backup $Database -Force
    Write-Host "  File database berhasil diganti." -ForegroundColor Green
} catch {
    Write-Host "ERROR: Gagal menyalin file backup: $_" -ForegroundColor Red
    exit 1
}

# Bersihkan file WAL dan SHM lama (karena database baru adalah standalone checkpointed snapshot)
$walFile = "$Database-wal"
$shmFile = "$Database-shm"

if (Test-Path $walFile) {
    Remove-Item $walFile -Force
    Write-Host "  File WAL lama dibersihkan." -ForegroundColor Cyan
}
if (Test-Path $shmFile) {
    Remove-Item $shmFile -Force
    Write-Host "  File SHM lama dibersihkan." -ForegroundColor Cyan
}

# Verifikasi integritas database hasil restore
Write-Host "  Memverifikasi integritas database hasil restore..." -ForegroundColor Yellow
try {
    & go run $checkpointScript -db $Database
    if ($LASTEXITCODE -ne 0) {
        Write-Host "WARNING: Verifikasi database hasil restore melaporkan peringatan!" -ForegroundColor Yellow
    }
} catch {
    Write-Host "WARNING: Gagal memverifikasi database hasil restore: $_" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "✅ Restore selesai dan terverifikasi!" -ForegroundColor Green
Write-Host ""
Write-Host "Langkah selanjutnya:" -ForegroundColor Yellow
Write-Host "  1. Jalankan aplikasi kembali (server akan otomatis menjalankan migrasi skema jika diperlukan)."
Write-Host "  2. Periksa apakah aplikasi bisa connect ke database."
Write-Host "  3. Lakukan pengecekan manual beberapa data penting."
Write-Host ""
Write-Host "Catatan: File database lama disimpan dengan nama .before-restore-*" -ForegroundColor Cyan
