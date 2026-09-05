package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	fmt.Println("=== VERIFIKASI LANGSUNG BINARY PRODUCTION aether-cbt.exe ===")

	releaseDir, err := filepath.Abs("release-test")
	if err != nil {
		fmt.Printf("FAIL: Gagal resolve release-test path: %v\n", err)
		os.Exit(1)
	}

	exePath := filepath.Join(releaseDir, "aether-cbt.exe")
	if _, err := os.Stat(exePath); err != nil {
		fmt.Printf("FAIL: Binary tidak ditemukan di %s: %v\n", exePath, err)
		os.Exit(1)
	}

	fmt.Printf("Menjalankan binary: %s\nWorking directory: %s\n", exePath, releaseDir)
	cmd := exec.Command(exePath)
	cmd.Dir = releaseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Printf("FAIL: Gagal menjalankan binary: %v\n", err)
		os.Exit(1)
	}

	// Ensure process is killed on exit
	defer func() {
		fmt.Println("Menghentikan server binary...")
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := "http://localhost:3000"

	// Wait for server to start
	ready := false
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		resp, err := client.Get(baseURL + "/api/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			ready = true
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	if !ready {
		fmt.Println("FAIL: Server tidak merespons di http://localhost:3000 dalam waktu 6 detik")
		os.Exit(1)
	}
	fmt.Println("Server aktif dan merespons. Memulai pengujian live...")

	failures := 0
	testCase := func(name string, fn func() error) {
		fmt.Printf("[TEST] %-60s ... ", name)
		if err := fn(); err != nil {
			fmt.Printf("GAGAL: %v\n", err)
			failures++
		} else {
			fmt.Printf("LULUS\n")
		}
	}

	// 1. GET /admin dari browser tanpa custom header -> 200 OK text/html
	testCase("GET /admin mengembalikan 200 OK dengan HTML SvelteKit", func() error {
		req, _ := http.NewRequest("GET", baseURL+"/admin", nil)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, ekspektasi 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			return fmt.Errorf("Content-Type = %q, ekspektasi text/html", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "<!doctype html>") && !strings.Contains(string(body), "<html") {
			return fmt.Errorf("body bukan HTML: %s", string(body[:min(100, len(body))]))
		}
		if strings.Contains(string(body), "Tenant ID is required") {
			return fmt.Errorf("terblokir oleh tenant middleware")
		}
		return nil
	})

	// 2. GET / mengembalikan 200 OK dengan HTML landing page
	testCase("GET / mengembalikan 200 OK dengan HTML landing page", func() error {
		req, _ := http.NewRequest("GET", baseURL+"/", nil)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, ekspektasi 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			return fmt.Errorf("Content-Type = %q, ekspektasi text/html", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), `"status":"running"`) {
			return fmt.Errorf("mengembalikan JSON mentah lama, bukan HTML frontend")
		}
		return nil
	})

	// 3. GET /student/login dan /supervisor/login
	testCase("GET /student/login mengembalikan 200 OK dengan HTML", func() error {
		resp, err := client.Get(baseURL + "/student/login")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, ekspektasi 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			return fmt.Errorf("Content-Type = %q, ekspektasi text/html", ct)
		}
		return nil
	})

	testCase("GET /supervisor/login mengembalikan 200 OK dengan HTML", func() error {
		resp, err := client.Get(baseURL + "/supervisor/login")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, ekspektasi 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			return fmt.Errorf("Content-Type = %q, ekspektasi text/html", ct)
		}
		return nil
	})

	// 4. Aset statis frontend (_app/...)
	testCase("GET /_app/... aset statis frontend termuat 200 OK", func() error {
		webBuildDir := filepath.Join(releaseDir, "web", "build")
		if _, err := os.Stat(webBuildDir); err != nil {
			webBuildDir = filepath.Join("web", "build")
		}
		assetPath := ""
		if matches, err := filepath.Glob(filepath.Join(webBuildDir, "_app", "immutable", "entry", "start.*.js")); err == nil && len(matches) > 0 {
			assetPath = "/_app/immutable/entry/" + filepath.Base(matches[0])
		}
		if assetPath == "" {
			return fmt.Errorf("tidak ada chunk entry start.*.js ditemukan di %s", webBuildDir)
		}
		resp, err := client.Get(baseURL + assetPath)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, ekspektasi 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "javascript") {
			return fmt.Errorf("Content-Type = %q, ekspektasi javascript", ct)
		}
		return nil
	})

	// 5. GET /api/health mengembalikan 200 OK {"status":"ok"}
	testCase("GET /api/health tanpa header tenant -> 200 OK status:ok", func() error {
		resp, err := client.Get(baseURL + "/api/health")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, ekspektasi 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"status":"ok"`) {
			return fmt.Errorf("body = %s, ekspektasi status:ok", string(body))
		}
		return nil
	})

	// 6. GET /api/qrcode?text=TEST mengembalikan 200 OK image/png
	testCase("GET /api/qrcode?text=TEST tanpa header tenant -> 200 OK image/png", func() error {
		resp, err := client.Get(baseURL + "/api/qrcode?text=TEST")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, ekspektasi 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "image/png") {
			return fmt.Errorf("Content-Type = %q, ekspektasi image/png", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
		if !bytes.HasPrefix(body, pngHeader) {
			return fmt.Errorf("file bukan PNG valid")
		}
		return nil
	})

	// 7. POST /api/ispring/webhook diproses tanpa penolakan missing tenant header
	testCase("POST /api/ispring/webhook diproses tanpa tenant header", func() error {
		form := url.Values{}
		form.Add("sid", "2026001")
		form.Add("sp", "10")
		form.Add("tp", "30")
		form.Add("attempt_token", "invalid-token-for-check")

		resp, err := client.PostForm(baseURL+"/api/ispring/webhook", form)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		// Should NOT fail with 400 "Tenant ID is required"
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "Tenant ID is required") {
			return fmt.Errorf("webhook ditolak karena missing tenant header!")
		}
		// Expect 403 active session not found (since token is mock), confirming tenant check passed
		if resp.StatusCode != http.StatusForbidden {
			return fmt.Errorf("status = %d, body = %s", resp.StatusCode, string(body))
		}
		if !strings.Contains(string(body), "active session not found") {
			return fmt.Errorf("body = %s, ekspektasi 'active session not found'", string(body))
		}
		return nil
	})

	// 8. GET /api/exam/content/* lolos pengecualian tenant middleware
	testCase("GET /api/exam/content/* lolos dari tenant middleware", func() error {
		resp, err := client.Get(baseURL + "/api/exam/content/index.html")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "Tenant ID is required") {
			return fmt.Errorf("terblokir oleh tenant middleware")
		}
		// Expect 401 Unauthorized because cookie is absent (content auth), NOT 400 tenant required
		if resp.StatusCode != http.StatusUnauthorized {
			return fmt.Errorf("status = %d, body = %s", resp.StatusCode, string(body))
		}
		return nil
	})

	// 9. GET /admin/settings dan /student/exam disajikan sebagai SPA SvelteKit
	testCase("GET /admin/settings disajikan sebagai HTML SvelteKit", func() error {
		resp, err := client.Get(baseURL + "/admin/settings")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, body = %s, ekspektasi 200", resp.StatusCode, string(body))
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			return fmt.Errorf("Content-Type = %q, ekspektasi text/html", ct)
		}
		return nil
	})

	testCase("GET /student/exam disajikan sebagai HTML SvelteKit", func() error {
		resp, err := client.Get(baseURL + "/student/exam")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, body = %s, ekspektasi 200", resp.StatusCode, string(body))
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			return fmt.Errorf("Content-Type = %q, ekspektasi text/html", ct)
		}
		return nil
	})

	// 10. GET /API/nonexistent mengembalikan JSON error (bukan HTML SPA)
	testCase("GET /API/nonexistent mengembalikan JSON (bukan HTML SPA)", func() error {
		resp, err := client.Get(baseURL + "/API/nonexistent")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusOK {
			return fmt.Errorf("status code = 200, rute API tidak boleh mengembalikan status 200")
		}
		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "text/html") {
			return fmt.Errorf("mengembalikan HTML frontend untuk rute API tidak terdaftar")
		}
		if strings.Contains(string(body), "<!doctype html>") || strings.Contains(string(body), "<html") {
			return fmt.Errorf("body mengembalikan HTML frontend untuk rute API")
		}
		return nil
	})

	// 11. GET /API/health (uppercase) lolos pengecualian
	testCase("GET /API/health (uppercase) -> 200 OK status:ok", func() error {
		resp, err := client.Get(baseURL + "/API/health")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code = %d, ekspektasi 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"status":"ok"`) {
			return fmt.Errorf("body = %s, ekspektasi status:ok", string(body))
		}
		return nil
	})

	fmt.Println("================================================================")
	if failures > 0 {
		fmt.Printf("HASIL PENGUJIAN: %d PENGUJIAN GAGAL!\n", failures)
		os.Exit(1)
	} else {
		fmt.Println("HASIL PENGUJIAN: 100% LULUS! SEMUA SPESIFIKASI TERPENUHI!")
	}
}
