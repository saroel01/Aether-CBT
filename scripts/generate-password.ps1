# Aether CBT - Password Generator
# Membuat password kuat untuk Admin, Ruang, dan Siswa
#
# Usage:
#   .\scripts\generate-password.ps1
#   .\scripts\generate-password.ps1 -Count 5 -Length 16
#   .\scripts\generate-password.ps1 -Count 3 -Length 12 -NoSymbols

param(
    [int]$Count = 5,
    [int]$Length = 14,
    [switch]$NoSymbols
)

$ErrorActionPreference = "Stop"

# Karakter yang digunakan
$lower = "abcdefghijklmnopqrstuvwxyz"
$upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
$digits = "0123456789"
$symbols = "!@#$%^&*()-_=+[]{};:,.<>?"

if ($NoSymbols) {
    $allChars = $lower + $upper + $digits
} else {
    $allChars = $lower + $upper + $digits + $symbols
}

function Get-SecurePassword {
    param([int]$len)

    $password = ""
    $random = [System.Security.Cryptography.RNGCryptoServiceProvider]::new()

    $bytes = New-Object Byte[] $len
    $random.GetBytes($bytes)

    for ($i = 0; $i -lt $len; $i++) {
        $index = $bytes[$i] % $allChars.Length
        $password += $allChars[$index]
    }

    # Pastikan minimal ada 1 huruf besar, 1 angka, dan 1 simbol (jika tidak NoSymbols)
    $hasUpper = $password -cmatch "[A-Z]"
    $hasDigit = $password -match "\d"
    $hasSymbol = if ($NoSymbols) { $true } else { $password -match "[!@#$%^&*()\-_=+\[\]{};:,.<>?]" }

    if (-not $hasUpper -or -not $hasDigit -or -not $hasSymbol) {
        # Regenerate jika tidak memenuhi syarat
        return Get-SecurePassword -len $len
    }

    return $password
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   Aether CBT - Password Generator      " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Menghasilkan $Count password dengan panjang $Length karakter..." -ForegroundColor Yellow
Write-Host ""

$passwords = @()

for ($i = 1; $i -le $Count; $i++) {
    $pwd = Get-SecurePassword -len $Length
    $passwords += $pwd

    $label = "Password $i".PadRight(12)
    Write-Host "$label : " -NoNewline -ForegroundColor White
    Write-Host $pwd -ForegroundColor Green
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Rekomendasi Penggunaan:" -ForegroundColor Yellow
Write-Host "  - Admin Sekolah : Gunakan password terkuat"
Write-Host "  - Pengawas Ruang: Berikan password berbeda per ruang"
Write-Host "  - Siswa         : Hindari password yang sama untuk semua"
Write-Host ""
Write-Host "Ingat: Setelah membuat password baru, segera rotasi sesuai" -ForegroundColor Red
Write-Host "prosedur di docs/credential-rotation.md" -ForegroundColor Red
Write-Host "========================================" -ForegroundColor Cyan
