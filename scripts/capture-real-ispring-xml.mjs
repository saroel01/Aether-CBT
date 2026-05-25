#!/usr/bin/env node
/**
 * Capture Real iSpring QuizReport XML
 * 
 * This script:
 * 1. Patches the published iSpring quiz to send results to localhost.
 * 2. Serves the quiz at http://localhost:4000
 * 3. Captures the real quizReport XML when you submit the quiz.
 * 4. Saves authentic XML files to tests/fixtures/ispring/
 * 
 * Usage:
 *   node scripts/capture-real-ispring-xml.mjs
 * 
 * Then open http://localhost:4000 in your browser (Chrome/Edge recommended).
 * Fill the form, answer some questions, and submit.
 * The real XML will be saved automatically.
 */

import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PROJECT_ROOT = path.resolve(__dirname, '..');
const QUIZ_DIR = path.join(PROJECT_ROOT, 'contoh_soal', 'KIMIA_XII_UAS_2025 (Published)');
const FIXTURE_DIR = path.join(PROJECT_ROOT, 'tests', 'fixtures', 'ispring');

const CAPTURE_PORT = 3999;
const QUIZ_PORT = 4000;

let capturedCount = 0;

function patchIndexHtml(originalHtml) {
  // Replace the webhook submission URL with our local capture server
  return originalHtml.replace(
    /"ss":\s*\{\s*"e":\s*true,\s*"u":\s*"[^"]*"\s*\}/,
    `"ss":{"e":true,"u":"http://localhost:${CAPTURE_PORT}"}`
  );
}

function createCaptureServer() {
  return http.createServer((req, res) => {
    // Enable CORS
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'POST, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', '*');

    if (req.method === 'OPTIONS') {
      res.writeHead(204);
      res.end();
      return;
    }

    if (req.method === 'POST') {
      let body = '';
      req.on('data', chunk => body += chunk);
      req.on('end', () => {
        const params = new URLSearchParams(body);
        const dr = params.get('dr');

        if (dr && dr.length > 100) {
          capturedCount++;
          const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
          const xmlFile = path.join(FIXTURE_DIR, `kimia-xii-uas-2025-real-${timestamp}.xml`);
          const metaFile = path.join(FIXTURE_DIR, `kimia-xii-uas-2025-real-${timestamp}.json`);

          fs.writeFileSync(xmlFile, dr, 'utf8');

          const meta = {
            captured_at: new Date().toISOString(),
            sp: params.get('sp'),
            tp: params.get('tp'),
            sid: params.get('sid'),
            USER_NAME: params.get('USER_NAME'),
            qt: params.get('qt'),
            xml_length: dr.length
          };
          fs.writeFileSync(metaFile, JSON.stringify(meta, null, 2), 'utf8');

          console.log(`\n[CAPTURED #${capturedCount}] Real iSpring quizReport XML`);
          console.log(`  Saved: ${path.relative(PROJECT_ROOT, xmlFile)}`);
          console.log(`  Score: ${params.get('sp')}/${params.get('tp')}`);
          console.log(`  Length: ${dr.length} characters\n`);
        }

        res.writeHead(200, { 'Content-Type': 'text/plain' });
        res.end('OK');
      });
    } else {
      res.writeHead(200, { 'Content-Type': 'text/plain' });
      res.end('iSpring XML Capture Server is running');
    }
  });
}

function createQuizServer(patchedHtml) {
  return http.createServer((req, res) => {
    let urlPath = req.url.split('?')[0];
    if (urlPath === '/' || urlPath === '/index.html') {
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(patchedHtml);
      return;
    }

    const filePath = path.join(QUIZ_DIR, urlPath);
    const ext = path.extname(filePath).toLowerCase();
    const mimeTypes = {
      '.js': 'application/javascript',
      '.css': 'text/css',
      '.png': 'image/png',
      '.jpg': 'image/jpeg',
      '.html': 'text/html',
      '.svg': 'image/svg+xml',
      '.woff': 'font/woff',
      '.woff2': 'font/woff2',
      '.ico': 'image/x-icon'
    };

    try {
      const data = fs.readFileSync(filePath);
      res.writeHead(200, { 'Content-Type': mimeTypes[ext] || 'application/octet-stream' });
      res.end(data);
    } catch {
      res.writeHead(404);
      res.end('Not found');
    }
  });
}

async function main() {
  if (!fs.existsSync(QUIZ_DIR)) {
    console.error('ERROR: Quiz folder not found at', QUIZ_DIR);
    process.exit(1);
  }

  fs.mkdirSync(FIXTURE_DIR, { recursive: true });

  console.log('\n=== Aether CBT - Real iSpring XML Capture Tool ===\n');

  // Patch the quiz
  console.log('[1/3] Patching quiz to send results to local capture server...');
  const originalHtml = fs.readFileSync(path.join(QUIZ_DIR, 'index.html'), 'utf8');
  const patchedHtml = patchIndexHtml(originalHtml);
  console.log('      Webhook URL patched to localhost.');

  // Start servers
  const captureServer = createCaptureServer();
  const quizServer = createQuizServer(patchedHtml);

  await new Promise(resolve => captureServer.listen(CAPTURE_PORT, resolve));
  await new Promise(resolve => quizServer.listen(QUIZ_PORT, resolve));

  console.log('[2/3] Capture server running on port', CAPTURE_PORT);
  console.log('[3/3] Quiz server running on port', QUIZ_PORT);

  console.log('\n' + '='.repeat(60));
  console.log('OPEN THIS URL IN YOUR BROWSER (Chrome or Edge recommended):');
  console.log(`  http://localhost:${QUIZ_PORT}/`);
  console.log('='.repeat(60));

  console.log('\nINSTRUCTIONS:');
  console.log('  1. The quiz should load normally.');
  console.log('  2. Fill the authorization form (choose any options + fill text fields).');
  console.log('  3. Answer at least 5–10 questions (any answers are fine for testing).');
  console.log('  4. Click submit / finish when ready.');
  console.log('  5. The REAL quizReport XML from iSpring will be saved automatically.');
  console.log('\nFiles will be saved to:');
  console.log(`  ${path.relative(PROJECT_ROOT, FIXTURE_DIR)}`);

  console.log('\nPress Ctrl+C when you are done capturing.\n');

  // Keep the process alive
  process.on('SIGINT', () => {
    console.log(`\n\nCapture finished. Total real XML captured: ${capturedCount}`);
    console.log('Servers stopped.\n');
    captureServer.close();
    quizServer.close();
    process.exit(0);
  });
}

main().catch(err => {
  console.error('Fatal error:', err);
  process.exit(1);
});
