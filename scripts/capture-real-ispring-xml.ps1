<#
.SYNOPSIS
    Capture Real iSpring QuizReport XML from published quiz (Windows)

.DESCRIPTION
    This script:
    1. Patches the published iSpring quiz to send results to localhost.
    2. Starts a simple capture server.
    3. Serves the quiz at http://localhost:4000
    4. Saves any received quizReport XML to tests/fixtures/ispring/

.USAGE
    1. Run this script from the project root:
       .\scripts\capture-real-ispring-xml.ps1

    2. Open the URL shown in browser (Chrome/Edge recommended).

    3. Fill the form (auth), answer some questions, and click submit.

    4. The real quizReport XML will be saved automatically.

    5. Press Ctrl+C when done.
#>

$ErrorActionPreference = "Stop"

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$QuizDir = Join-Path $ProjectRoot "contoh_soal\KIMIA_XII_UAS_2025 (Published)"
$FixtureDir = Join-Path $ProjectRoot "tests\fixtures\ispring"
$CapturePort = 3999
$QuizPort = 4000

if (-not (Test-Path $QuizDir)) {
    Write-Error "Quiz folder not found: $QuizDir"
    exit 1
}

New-Item -ItemType Directory -Force -Path $FixtureDir | Out-Null

Write-Host "=== Aether CBT - Real iSpring XML Capture ===" -ForegroundColor Cyan
Write-Host ""

# === Patch the webhook URL ===
Write-Host "[1/3] Patching quiz to send results to localhost capture server..." -ForegroundColor Yellow

$IndexPath = Join-Path $QuizDir "index.html"
$OriginalHtml = Get-Content -LiteralPath $IndexPath -Raw -Encoding UTF8

# Find and replace the submission URL in the base64 data
$PatchedHtml = $OriginalHtml -replace 
    '"ss":\s*\{\s*"e":\s*true,\s*"u":\s*"[^"]*"\s*\}', 
    ('"ss":{"e":true,"u":"http://localhost:' + $CapturePort + '"}')

$PatchedIndexPath = Join-Path $env:TEMP "ispring-patched-index.html"
$PatchedHtml | Out-File -LiteralPath $PatchedIndexPath -Encoding UTF8

Write-Host "    Patched index saved temporarily." -ForegroundColor Green

# === Start Capture Server ===
Write-Host "[2/3] Starting capture server on port $CapturePort..." -ForegroundColor Yellow

$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:$CapturePort/")
$listener.Start()

Write-Host "    Capture server running." -ForegroundColor Green
Write-Host ""

# === Start simple file server for the quiz ===
Write-Host "[3/3] Starting quiz server on port $QuizPort..." -ForegroundColor Yellow
Write-Host ""
Write-Host ">>> OPEN THIS URL IN YOUR BROWSER (Chrome or Edge):" -ForegroundColor Green
Write-Host "    http://localhost:$QuizPort/" -ForegroundColor White
Write-Host ""
Write-Host ">>> INSTRUCTIONS:" -ForegroundColor Cyan
Write-Host "    1. The page should load the real iSpring quiz." -ForegroundColor White
Write-Host "    2. Fill the authorization form (Sekolah, Mapel, Kelas, No Ujian, Nama)." -ForegroundColor White
Write-Host "    3. Answer at least 5-10 questions (any answers are fine)." -ForegroundColor White
Write-Host "    4. Click submit / finish when done." -ForegroundColor White
Write-Host "    5. Real quizReport XML will be saved automatically here:" -ForegroundColor White
Write-Host "       $FixtureDir" -ForegroundColor White
Write-Host ""
Write-Host ">>> Press Ctrl+C in this window when you are finished capturing." -ForegroundColor Yellow
Write-Host ""

# Simple static file server using HttpListener (second listener)
$quizListener = New-Object System.Net.HttpListener
$quizListener.Prefixes.Add("http://localhost:$QuizPort/")
$quizListener.Start()

$capturedCount = 0

try {
    while ($listener.IsListening -or $quizListener.IsListening) {
        # Handle capture POSTs
        if ($listener.IsListening) {
            $captureContext = $listener.GetContextAsync().GetAwaiter().GetResult()
            $request = $captureContext.Request
            $response = $captureContext.Response

            if ($request.HttpMethod -eq "POST") {
                $reader = New-Object System.IO.StreamReader($request.InputStream)
                $body = $reader.ReadToEnd()
                $reader.Close()

                $params = [System.Web.HttpUtility]::ParseQueryString($body)
                $dr = $params["dr"]

                if ($dr) {
                    $capturedCount++
                    $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
                    $xmlFile = Join-Path $FixtureDir "kimia-xii-uas-2025-real-$timestamp.xml"
                    $metaFile = Join-Path $FixtureDir "kimia-xii-uas-2025-real-$timestamp.json"

                    [System.IO.File]::WriteAllText($xmlFile, $dr, [System.Text.Encoding]::UTF8)

                    $meta = @{
                        captured_at = (Get-Date).ToString("o")
                        sp = $params["sp"]
                        tp = $params["tp"]
                        sid = $params["sid"]
                        USER_NAME = $params["USER_NAME"]
                        qt = $params["qt"]
                        length = $dr.Length
                    } | ConvertTo-Json -Depth 3

                    [System.IO.File]::WriteAllText($metaFile, $meta, [System.Text.Encoding]::UTF8)

                    Write-Host ""
                    Write-Host "[CAPTURED] Real iSpring quizReport #$capturedCount" -ForegroundColor Green
                    Write-Host "    Saved: $xmlFile" -ForegroundColor Green
                    Write-Host "    Score: $($params['sp'])/$($params['tp'])" -ForegroundColor Green
                    Write-Host ""
                }
            }

            $response.StatusCode = 200
            $response.Close()
        }

        # Handle quiz file serving
        if ($quizListener.IsListening) {
            $quizContext = $quizListener.GetContextAsync().GetAwaiter().GetResult()
            $req = $quizContext.Request
            $res = $quizContext.Response

            $urlPath = $req.Url.LocalPath.TrimStart('/')
            if ([string]::IsNullOrEmpty($urlPath) -or $urlPath -eq "/") {
                $urlPath = "index.html"
            }

            $fullPath = Join-Path $QuizDir $urlPath

            if ($urlPath -eq "index.html") {
                $fullPath = $PatchedIndexPath
            }

            if (Test-Path $fullPath -PathType Leaf) {
                $bytes = [System.IO.File]::ReadAllBytes($fullPath)
                $res.ContentType = "application/octet-stream"
                if ($urlPath.EndsWith(".html")) { $res.ContentType = "text/html; charset=utf-8" }
                if ($urlPath.EndsWith(".js"))   { $res.ContentType = "application/javascript" }
                if ($urlPath.EndsWith(".css"))  { $res.ContentType = "text/css" }
                $res.OutputStream.Write($bytes, 0, $bytes.Length)
            } else {
                $res.StatusCode = 404
            }
            $res.Close()
        }
    }
}
finally {
    $listener.Stop()
    $quizListener.Stop()
    Write-Host ""
    Write-Host "Capture server stopped." -ForegroundColor Yellow
    Write-Host "Total real XML captured: $capturedCount" -ForegroundColor Cyan
}
