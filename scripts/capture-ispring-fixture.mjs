import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import puppeteer from "puppeteer";

const QUIZ_DIR = path.resolve(fileURLToPath(import.meta.url), "..", "..", "contoh_soal", "KIMIA_XII_UAS_2025 (Published)");
const FIXTURE_DIR = path.resolve(fileURLToPath(import.meta.url), "..", "..", "tests", "fixtures", "ispring");
const CAPTURE_PORT = 3999;
const QUIZ_PORT = 4000;
let capturedResults = [];

function createCaptureServer() {
  return new Promise((resolve) => {
    const server = http.createServer((req, res) => {
      res.setHeader("Access-Control-Allow-Origin", "*");
      if (req.method === "OPTIONS") { res.writeHead(204); res.end(); return; }
      if (req.method === "POST") {
        let body = "";
        req.on("data", (c) => (body += c));
        req.on("end", () => {
          const r = Object.fromEntries(new URLSearchParams(body).entries());
          capturedResults.push(r);
          console.log(`[CAPTURE] #${capturedResults.length}: sp=${r.sp} tp=${r.tp} dr=${(r.dr||"").length}`);
          res.writeHead(200); res.end("OK");
        });
      } else { res.writeHead(200); res.end("OK"); }
    });
    server.listen(CAPTURE_PORT, () => resolve(server));
  });
}

function createQuizServer(html) {
  return new Promise((resolve) => {
    const server = http.createServer((req, res) => {
      const up = req.url.split("?")[0];
      if (up === "/" || up === "/index.html") { res.writeHead(200, {"Content-Type":"text/html; charset=utf-8"}); res.end(html); return; }
      const fp = path.join(QUIZ_DIR, up);
      const ext = path.extname(fp).toLowerCase();
      const t = {".js":"application/javascript",".css":"text/css",".png":"image/png",".jpg":"image/jpeg",".svg":"image/svg+xml",".woff":"font/woff",".woff2":"font/woff2",".ico":"image/x-icon",".html":"text/html",".xml":"text/xml"};
      try { const d = fs.readFileSync(fp); res.writeHead(200,{"Content-Type":t[ext]||"application/octet-stream"}); res.end(d); }
      catch { res.writeHead(404); res.end(); }
    });
    server.listen(QUIZ_PORT, () => resolve(server));
  });
}

function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

async function main() {
  fs.mkdirSync(FIXTURE_DIR, { recursive: true });

  const rawHtml = fs.readFileSync(path.join(QUIZ_DIR, "index.html"), "utf8");
  const patchedHtml = rawHtml.replace(
    /"ss":\s*\{"e":\s*true,\s*"u":\s*"[^"]*"\}/,
    `"ss":{"e":true,"u":"http://localhost:${CAPTURE_PORT}"}`
  );

  const captureServer = await createCaptureServer();
  const quizServer = await createQuizServer(patchedHtml);
  console.log(`[SERVER] capture:${CAPTURE_PORT} quiz:${QUIZ_PORT}`);

  const browser = await puppeteer.launch({ headless: true, args: ["--no-sandbox", "--disable-setuid-sandbox"] });
  const page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 800 });

  await page.goto(`http://localhost:${QUIZ_PORT}/index.html`, { waitUntil: "networkidle0", timeout: 60000 });
  await sleep(5000);

  // STEP 1: Click MULAI
  console.log("[1] MULAI...");
  await page.evaluate(() => {
    for (const b of document.querySelectorAll('button')) {
      if (b.textContent?.includes("MULAI") && b.offsetWidth > 0) { b.click(); return; }
    }
  });
  await sleep(2000);

  // STEP 2: Fill each combobox by clicking it, waiting for dropdown, selecting option
  console.log("[2] Filling combobox fields...");
  for (let i = 0; i < 3; i++) {
    const clicked = await page.evaluate((idx) => {
      const fields = document.querySelectorAll('.field');
      const comboBoxFields = Array.from(fields).filter(f => f.querySelector('.combobox'));
      if (idx < comboBoxFields.length) {
        const combo = comboBoxFields[idx].querySelector('.combobox');
        if (combo) { combo.click(); return 'clicked combo ' + idx; }
      }
      return 'no combo ' + idx;
    }, i);
    console.log(`  [2a] ${clicked}`);

    await sleep(1000);

    const selected = await page.evaluate(() => {
      const options = document.querySelectorAll('.option, [role="option"], li[class*="option"], li[class*="item"]');
      if (options.length > 0) { options[0].click(); return `selected from ${options.length} options`; }
      const popup = document.querySelector('.popup, .dropdown, [class*="popup"], [class*="dropdown"], [class*="list"]');
      if (popup) { return `popup found: ${popup.innerHTML.substring(0, 200)}`; }
      const allNew = document.querySelectorAll('[class*="option"], [class*="select"], [class*="list"]');
      return `allNew: ${allNew.length}`;
    });
    console.log(`  [2b] ${selected}`);

    await sleep(800);
  }

  // Fill text inputs
  console.log("[2c] Filling text fields...");
  await page.evaluate(() => {
    const fields = document.querySelectorAll('.field');
    fields.forEach(f => {
      const input = f.querySelector('input');
      if (input && !f.querySelector('.combobox')) {
        const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
        setter.call(input, 'TEST001');
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
      }
    });
  });
  await sleep(500);

  // STEP 3: Click JAWAB
  console.log("[3] JAWAB...");
  await page.evaluate(() => {
    for (const b of document.querySelectorAll('button')) {
      if (b.textContent?.trim() === 'JAWAB' && b.offsetWidth > 0) { b.click(); return; }
    }
  });
  await sleep(3000);

  // Check state
  const state = await page.evaluate(() => document.querySelector("div[id^='q_']")?.textContent?.substring(0, 200));
  console.log("[STATE]", state?.substring(0, 150));

  // If still on auth, take screenshot for debugging
  if (state?.includes('SILAHKAN ISI DATA')) {
    console.log("[WARN] Still on auth form. Saving debug...");
    await page.screenshot({ path: path.join(FIXTURE_DIR, "debug-auth.png"), fullPage: true });

    // Try alternative: directly manipulate iSpring internal state
    console.log("[ALT] Trying to bypass auth via iSpring API...");
    await page.evaluate(() => {
      const fields = document.querySelectorAll('.field');
      fields.forEach((f, i) => {
        f.classList.remove('empty', 'error');
        f.classList.add('filled');
        const combo = f.querySelector('.combobox');
        if (combo) {
          combo.textContent = i === 0 ? 'SMAN MODAL BANGSA ARUN' : i === 1 ? 'KIMIA XII UAS' : 'XII 4';
        }
      });
    });
    await sleep(500);

    // Try JAWAB again
    await page.evaluate(() => {
      for (const b of document.querySelectorAll('button')) {
        if (b.textContent?.trim() === 'JAWAB' && b.offsetWidth > 0) { b.click(); return; }
      }
    });
    await sleep(3000);
  }

  // STEP 4: Answer questions
  console.log("[4] Answering questions...");
  for (let q = 0; q < 55; q++) {
    const r = await page.evaluate(() => {
      const c = document.querySelector("div[id^='q_']");
      if (!c) return { done: true };
      const t = c.textContent?.substring(0, 100) || '';
      if (t.includes('SILAHKAN ISI DATA')) return { stuck: 'auth' };
      if (t.includes('SELAMAT') || t.includes('Hasil') || t.includes('result') || t.includes('score') || t.includes('Nilai') || t.includes('Lulus') || t.includes('Tidak Lulus')) return { quizDone: true, text: t };

      const content = c.querySelector('.content');
      if (!content) return { noContent: true };

      const pointers = [];
      const walk = (el) => {
        for (const ch of el.children) {
          try {
            if (window.getComputedStyle(ch).cursor === 'pointer' && ch.offsetWidth > 5 && ch.offsetHeight > 5) {
              const cls = ch.className?.toString?.() || '';
              if (!cls.includes('start') && !cls.includes('show_slides') && !cls.includes('mark_slide') && !cls.includes('exit_review') && !cls.includes('control_panel') && !cls.includes('bottom_panel') && !cls.includes('submit')) {
                pointers.push(ch);
              }
            }
          } catch {}
          walk(ch);
        }
      };
      walk(content);

      if (pointers.length > 0) {
        pointers[Math.floor(Math.random() * Math.min(3, pointers.length))].click();
        return { answered: pointers.length };
      }

      const btns = Array.from(c.querySelectorAll('button')).filter(b => b.offsetWidth > 0);
      for (const b of btns) {
        const txt = b.textContent?.trim().toLowerCase() || '';
        if (txt.includes('next') || txt.includes('submit') || txt.includes('check') || txt.includes('lanjut') || txt.includes('periksa') || txt.includes('kirim')) {
          b.click();
          return { btn: txt };
        }
      }

      const sub = btns.find(b => b.className?.includes('submit'));
      if (sub) { sub.click(); return { submitted: true }; }

      return { noAction: true, btnCount: btns.length };
    });

    if (r.done || r.stuck === 'auth') { console.log(`[Q${q}]`, JSON.stringify(r)); break; }
    if (r.quizDone) { console.log(`[Q${q}] QUIZ COMPLETE!`); break; }
    if (q % 5 === 0 || r.noAction) console.log(`[Q${q}]`, JSON.stringify(r));
    await sleep(400);
  }

  await sleep(3000);

  // Try finish
  await page.evaluate(() => {
    for (const b of document.querySelectorAll('button')) {
      const t = b.textContent?.trim().toLowerCase() || '';
      if (b.offsetWidth > 0 && (t.includes('finish') || t.includes('selesai') || t.includes('review') || t.includes('close'))) {
        b.click(); return;
      }
    }
  });
  await sleep(8000);

  // Results
  if (capturedResults.length > 0) {
    console.log(`\n[SUCCESS] ${capturedResults.length} result(s)!`);
    for (let i = 0; i < capturedResults.length; i++) {
      const r = capturedResults[i];
      if (r.dr) {
        const n = `kimia-xii-uas-2025-result-${i+1}.xml`;
        fs.writeFileSync(path.join(FIXTURE_DIR, n), r.dr, "utf8");
        console.log(`[SAVED] ${n} (${r.dr.length} chars)`);
      }
      fs.writeFileSync(path.join(FIXTURE_DIR, `kimia-xii-uas-2025-webhook-${i+1}.json`), JSON.stringify(r, null, 2), "utf8");
    }
  } else {
    console.log("\n[WARN] No webhook. Debug...");
    await page.screenshot({ path: path.join(FIXTURE_DIR, "debug-final.png"), fullPage: true });
    fs.writeFileSync(path.join(FIXTURE_DIR, "debug-final.html"), await page.content(), "utf8");
    console.log("[SAVED] debug files");
  }

  await browser.close();
  captureServer.close();
  quizServer.close();
  console.log("[DONE]");
}

main().catch((e) => { console.error("[FATAL]", e); process.exit(1); });
