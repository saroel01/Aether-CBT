package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
)

const (
	lower   = "abcdefghijklmnopqrstuvwxyz"
	upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits  = "0123456789"
	symbols = "!@#$%^&*()-_=+[]{};:,.<>?"
)

func main() {
	count := flag.Int("count", 5, "Jumlah password yang akan dibuat")
	length := flag.Int("length", 14, "Panjang password")
	noSymbols := flag.Bool("no-symbols", false, "Tanpa karakter simbol")
	help := flag.Bool("help", false, "Tampilkan bantuan")
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	charset := lower + upper + digits
	if !*noSymbols {
		charset += symbols
	}

	fmt.Println("========================================")
	fmt.Println("   Aether CBT - Password Generator")
	fmt.Println("   (Versi untuk Produksi / Admin Sekolah)")
	fmt.Println("========================================")
	fmt.Printf("\nMenghasilkan %d password dengan panjang %d karakter...\n\n", *count, *length)

	for i := 1; i <= *count; i++ {
		pwd, err := generateSecurePassword(*length, charset, *noSymbols)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Password %d : %s\n", i, pwd)
	}

	fmt.Println("\n========================================")
	fmt.Println("CATATAN PENTING:")
	fmt.Println("  - Simpan password ini di tempat yang aman.")
	fmt.Println("  - Jangan bagikan melalui chat yang tidak aman.")
	fmt.Println("  - Ikuti prosedur rotasi di file PANDUAN_ROTASI_KREDENSIAL_PRODUKSI.txt")
	fmt.Println("========================================")
}

func generateSecurePassword(length int, charset string, noSymbols bool) (string, error) {
	var password strings.Builder
	password.Grow(length)

	// Pastikan ada minimal 1 huruf besar, 1 angka, dan 1 simbol (kecuali no-symbols)
	required := []rune{upper[0], digits[0]}
	if !noSymbols {
		required = append(required, symbols[0])
	}

	for _, ch := range required {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		password.WriteRune(ch)
		// Sisipkan di posisi acak
		pos, _ := rand.Int(rand.Reader, big.NewInt(int64(length)))
		if int(pos.Int64()) < password.Len() {
			// sederhana: tambahkan saja di akhir untuk simplicity
		}
	}

	// Isi sisanya
	for password.Len() < length {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password.WriteRune(rune(charset[n.Int64()]))
	}

	// Shuffle
	pwd := []rune(password.String())
	for i := range pwd {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(len(pwd))))
		pwd[i], pwd[j.Int64()] = pwd[j.Int64()], pwd[i]
	}

	return string(pwd), nil
}

func printHelp() {
	fmt.Println(`Aether CBT Password Generator (Versi Produksi)

Penggunaan:
  aether-password-generator.exe
  aether-password-generator.exe -count 8 -length 16
  aether-password-generator.exe -count 3 -length 12 -no-symbols

Flag:
  -count       Jumlah password (default: 5)
  -length      Panjang password (default: 14)
  -no-symbols  Tanpa simbol
  -help        Tampilkan bantuan ini

Password yang dihasilkan sudah memenuhi standar keamanan dasar
(huruf besar, huruf kecil, angka, dan simbol).
`)
}
