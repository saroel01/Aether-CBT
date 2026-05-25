# Aether CBT - Restore Database (PowerShell)
# PERINGATAN: Hentikan aplikasi sebelum menjalankan restore!
#
# Usage:
#   .\scripts\restore.ps1 -Backup "backups\cbt_aether_20260525_115405.db"

param(
    [Parameter(Mandatory=$true)]
    [string]$Backup,

    [string]$Database = "data/cbt_aether.db"
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

$confirm = Read-Host "Apakah Anda yakin ingin melanjutkan restore? (ketik 'YA' untuk konfirmasi)"

if ($confirm -ne "YA") {
    Write-Host "Restore dibatalkan oleh user." -ForegroundColor Cyan
    exit 0
}

# Validasi file backup
if (-not (Test-Path $Backup)) {
    Write-Host "ERROR: File backup tidak ditemukan: $Backup" -ForegroundColor Red
    exit 1
}

# Buat folder data jika belum ada
$dataDir = Split-Path $Database -Parent
if (-not (Test-Path $dataDir)) {
    New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
}

Write-Host ""
Write-Host "Memulai proses restore..." -ForegroundColor Yellow
Write-Host "  Backup   : $Backup"
Write-Host "  Target   : $Database"

# Backup file database lama (jika ada) dengan timestamp
if (Test-Path $Database) {
    $timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $oldBackup = "$Database.before-restore-$timestamp"
    Write-Host "  Membuat cadangan database lama ke: $oldBackup" -ForegroundColor Cyan
    Copy-Item $Database $oldBackup -Force
}

# Salin file backup sebagai database baru
try {
    Copy-Item $Backup $Database -Force
    Write-Host "  File database berhasil diganti." -ForegroundColor Green
} catch {
    Write-Host "ERROR: Gagal menyalin file backup: $_" -ForegroundColor Red
    exit 1
}

# Bersihkan file WAL dan SHM lama (jika ada)
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

Write-Host ""
Write-Host "✅ Restore selesai!" -ForegroundColor Green
Write-Host ""
Write-Host "Langkah selanjutnya:" -ForegroundColor Yellow
Write-Host "  1. Jalankan aplikasi kembali."
Write-Host "  2. Periksa apakah aplikasi bisa connect ke database."
Write-Host "  3. Lakukan pengecekan manual beberapa data penting."
Write-Host ""
Write-Host "Catatan: File database lama disimpan dengan nama .before-restore-*" -ForegroundColor Cyan
