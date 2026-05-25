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
	length := flag.Int("length", 14, "Panjang setiap password")
	noSymbols := flag.Bool("no-symbols", false, "Tanpa simbol")
	flag.Parse()

	charset := lower + upper + digits
	if !*noSymbols {
		charset += symbols
	}

	fmt.Println("========================================")
	fmt.Println("   Aether CBT - Password Generator (Go) ")
	fmt.Println("========================================")
	fmt.Printf("\nMenghasilkan %d password dengan panjang %d karakter...\n\n", *count, *length)

	for i := 1; i <= *count; i++ {
		pwd, err := generatePassword(*length, charset, *noSymbols)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Password %d : %s\n", i, pwd)
	}

	fmt.Println("\n========================================")
	fmt.Println("Rekomendasi: Gunakan password berbeda untuk setiap peran.")
	fmt.Println("Lihat prosedur lengkap di docs/credential-rotation.md")
	fmt.Println("========================================")
}

func generatePassword(length int, charset string, noSymbols bool) (string, error) {
	var password strings.Builder
	password.Grow(length)

	// Pastikan minimal ada 1 huruf besar, 1 angka, dan 1 simbol (jika diizinkan)
	mustHave := []rune{upper[0], digits[0]}
	if !noSymbols {
		mustHave = append(mustHave, symbols[0])
	}

	for _, ch := range mustHave {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		// Sisipkan karakter wajib di posisi acak
		pos, _ := rand.Int(rand.Reader, big.NewInt(int64(length)))
		password.WriteRune(ch)
		if pos.Int64() < int64(password.Len()) {
			password.Reset()
			password.WriteRune(ch)
		}
	}

	// Isi sisanya dengan karakter acak
	for password.Len() < length {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password.WriteRune(rune(charset[n.Int64()]))
	}

	// Shuffle sederhana
	pwd := []rune(password.String())
	for i := range pwd {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(len(pwd))))
		pwd[i], pwd[j.Int64()] = pwd[j.Int64()], pwd[i]
	}

	return string(pwd), nil
}
