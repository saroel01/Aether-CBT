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
          console.log(`[CAPTURE] #${capturedResults.length}: sp=${r.sp} tp=${r.tp} dr=${(r.dr || "").length} chars`);
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
      if (up === "/" || up === "/index.html") {
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
        res.end(html);
        return;
      }
      const fp = path.join(QUIZ_DIR, up);
      const ext = path.extname(fp).toLowerCase();
      const t = { ".js": "application/javascript", ".css": "text/css", ".png": "image/png", ".jpg": "image/jpeg", ".svg": "image/svg+xml", ".woff": "font/woff", ".woff2": "font/woff2", ".ico": "image/x-icon", ".html": "text/html", ".xml": "text/xml" };
      try { const d = fs.readFileSync(fp); res.writeHead(200, { "Content-Type": t[ext] || "application/octet-stream" }); res.end(d); }
      catch { res.writeHead(404); res.end(); }
    });
    server.listen(QUIZ_PORT, () => resolve(server));
  });
}

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

/**
 * Expert-level patching:
 * We decode the big base64 data, pre-fill the AuthorizationSlide answers,
 * then re-encode so the player thinks auth is already completed.
 */
function patchQuizDataWithAuthPreFill(originalHtml) {
  // Extract the big data= "..." string
  const dataMatch = originalHtml.match(/var data = "([^"]+)"/);
  if (!dataMatch) throw new Error("Cannot find var data = ... in index.html");

  let b64 = dataMatch[1];
  // Fix padding
  const pad = (4 - (b64.length % 4)) % 4;
  if (pad) b64 += "=".repeat(pad);

  const jsonStr = Buffer.from(b64, "base64").toString("utf8");
  const data = JSON.parse(jsonStr.replace(/^\uFEFF/, "")); // remove BOM if present

  // The auth slide is under d.sl.au
  const authSlide = data.d.sl.au;
  if (!authSlide || !authSlide.D || !authSlide.D.C || !authSlide.D.C.au) {
    console.warn("[PATCH] Authorization slide structure not found as expected. Proceeding without prefill.");
    return originalHtml;
  }

  const authFields = authSlide.D.C.au.f; // array of field definitions

  // Pre-fill values based on the real quiz
  const prefill = {
    "SEKOLAH": "SMAN MODAL BANGSA ARUN",
    "MAPEL": "KIMIA XII UAS",
    "KELAS": "XII 4",
    "NO_TES": "TEST001",
    "NAMA_PESERTA": "Test Student Expert"
  };

  // iSpring stores the answers in the slide data under a different path sometimes.
  // We will also set values on the C.au structure if it holds current values.
  if (authSlide.D.C.au && Array.isArray(authSlide.D.C.au.f)) {
    authSlide.D.C.au.f.forEach(field => {
      const name = field.n; // e.g. SEKOLAH, NO_TES, etc.
      if (prefill[name]) {
        // For select fields, set the value
        if (field.t === "select" || field.tp === "select") {
          field.v = [prefill[name]];
          field.selected = prefill[name];
        } else {
          field.v = [prefill[name]];
        }
      }
    });
  }

  // Some versions store current answers in D.au or similar
  if (authSlide.D.au && authSlide.D.au.f) {
    authSlide.D.au.f.forEach(field => {
      const name = field.n;
      if (prefill[name]) field.v = [prefill[name]];
    });
  }

  // Re-encode
  const newJson = JSON.stringify(data);
  const newB64 = Buffer.from(newJson).toString("base64");

  // Replace in HTML
  const patchedHtml = originalHtml.replace(
    /var data = "[^"]+"/,
    `var data = "${newB64}"`
  );

  // Also patch the webhook URL
  return patchedHtml.replace(
    /"ss":\s*\{"e":\s*true,\s*"u":\s*"[^"]*"\}/,
    `"ss":{"e":true,"u":"http://localhost:${CAPTURE_PORT}"}`
  );
}

async function main() {
  fs.mkdirSync(FIXTURE_DIR, { recursive: true });

  const originalHtml = fs.readFileSync(path.join(QUIZ_DIR, "index.html"), "utf8");
  const patchedHtml = patchQuizDataWithAuthPreFill(originalHtml);

  console.log("[SERVER] Starting with EXPERT pre-filled auth data...");
  const captureServer = await createCaptureServer();
  const quizServer = await createQuizServer(patchedHtml);
  console.log(`[CAPTURE] :${CAPTURE_PORT}   [QUIZ] :${QUIZ_PORT}`);

  const browser = await puppeteer.launch({
    headless: true,
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-web-security"]
  });

  const page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 800 });

  // Expose a function so page can send the real XML back to us
  await page.exposeFunction("sendRealQuizReport", async (xml, meta) => {
    console.log("[INJECTED] Real quizReport received from iSpring player!");
    capturedResults.push({
      dr: xml,
      sp: meta.score || "0",
      tp: meta.total || "100",
      sid: meta.sid || "TEST001",
      USER_NAME: meta.name || "Expert Simulation"
    });
  });

  console.log("[BROWSER] Opening quiz with pre-filled auth...");
  await page.goto(`http://localhost:${QUIZ_PORT}/index.html`, { waitUntil: "networkidle0", timeout: 60000 });
  await sleep(4000);

  // === EXPERT INJECTION ===
  console.log("[EXPERT] Injecting model-level control code...");
  const injectionResult = await page.evaluate(() => {
    const results = [];

    // 1. Try to find the player object from the callback
    let player = null;
    const origStart = window.QuizPlayer && window.QuizPlayer.start;
    if (origStart) {
      window.QuizPlayer.start = function(...args) {
        const cb = args[8]; // the callback function(player)
        if (typeof cb === "function") {
          args[8] = function(p) {
            player = p;
            window.__AETHER_ISPRING_PLAYER__ = p;
            results.push("Player object captured via hook");
            return cb(p);
          };
        }
        return origStart.apply(this, args);
      };
    }

    // 2. Aggressive discovery after a short delay
    setTimeout(() => {
      // Search common globals
      const candidates = [];
      for (const key of Object.keys(window)) {
        try {
          const obj = window[key];
          if (obj && typeof obj === "object") {
            if (typeof obj.evaluation === "function" || typeof obj.generateSessionXml === "function" ||
                (obj.quiz && typeof obj.quiz === "function")) {
              candidates.push(key);
              window[`__AETHER_MODEL_${key}`] = obj;
            }
          }
        } catch {}
      }
      if (candidates.length) results.push("Found model candidates: " + candidates.join(", "));

      // Try to find the quiz model from the player if we have it
      if (window.__AETHER_ISPRING_PLAYER__) {
        const p = window.__AETHER_ISPRING_PLAYER__;
        // Common patterns in iSpring players
        const possibleModels = [p.quiz, p.model, p.session, p._quiz, p.data];
        possibleModels.forEach((m, i) => {
          if (m) {
            window.__AETHER_QUIZ_MODEL__ = m;
            results.push("Model attached from player at index " + i);
          }
        });
      }

      results.push("Discovery complete. Window has __AETHER_ISPRING_PLAYER__: " + !!window.__AETHER_ISPRING_PLAYER__);
    }, 2500);

    return results;
  });

  console.log("[EXPERT] Injection result:", injectionResult);

  // Give time for discovery and for the (hopefully pre-filled) auth to be accepted
  await sleep(6000);

  // Now attempt to drive at model level
  console.log("[EXPERT] Attempting model-level answer + report generation...");
  const driveResult = await page.evaluate(async () => {
    const out = [];

    const model = window.__AETHER_QUIZ_MODEL__ || window.__AETHER_ISPRING_PLAYER__;
    if (!model) {
      out.push("No model found yet");
      return out;
    }

    out.push("Model found. Type: " + (model.constructor ? model.constructor.name : typeof model));

    // Try to find generateSessionXml capability
    let reportGenerator = null;
    if (typeof model.generateSessionXml === "function") reportGenerator = model;
    else if (model.reportGenerator && typeof model.reportGenerator.generateSessionXml === "function") reportGenerator = model.reportGenerator;

    if (reportGenerator) {
      out.push("Found reportGenerator!");
    }

    // Attempt to auto-complete questions if the model exposes methods
    try {
      if (typeof model.next === "function") {
        for (let i = 0; i < 60; i++) {
          if (typeof model.setAnswer === "function") {
            // best effort
          }
          model.next();
          await new Promise(r => setTimeout(r, 80));
        }
        out.push("Called next() many times");
      }
    } catch (e) {
      out.push("Error during navigation: " + e.message);
    }

    // Final attempt: force evaluation and report
    try {
      if (reportGenerator && typeof reportGenerator.generateSessionXml === "function") {
        const xml = reportGenerator.generateSessionXml(model);
        if (xml && xml.length > 1000) {
          // Send back to Node
          window.sendRealQuizReport(xml, {
            score: model.evaluation ? model.evaluation().awardedScore() : "0",
            total: model.evaluation ? model.evaluation().maxScore() : "100"
          });
          out.push("SUCCESS: Generated and sent real quizReport XML via sendRealQuizReport");
          return out;
        }
      }
    } catch (e) {
      out.push("Error generating report: " + e.message);
    }

    out.push("Could not force report generation via model. Will rely on normal flow.");
    return out;
  });

  console.log("[EXPERT] Drive result:", driveResult);

  // Fallback: if the above didn't work, do smarter UI interaction now that auth should be passed
  await sleep(3000);
  console.log("[FALLBACK] Doing smarter post-auth interaction...");

  for (let q = 0; q < 50; q++) {
    const state = await page.evaluate(() => {
      const c = document.querySelector("div[id^='q_']");
      if (!c) return { done: true };
      const text = c.textContent?.replace(/\s+/g, " ").trim().substring(0, 120) || "";

      if (text.includes("SILAHKAN ISI DATA")) return { stillAuth: true };

      // Click any visible choice-like elements
      const pointers = [];
      const content = c.querySelector(".content");
      if (content) {
        const walk = (el) => {
          for (const ch of el.children) {
            try {
              const cs = getComputedStyle(ch);
              if (cs.cursor === "pointer" && ch.offsetWidth > 5 && ch.offsetHeight > 5) {
                const cls = ch.className?.toString() || "";
                if (!cls.includes("start") && !cls.includes("show_slides") && !cls.includes("control")) {
                  pointers.push(ch);
                }
              }
            } catch {}
            walk(ch);
          }
        };
        walk(content);
      }

      if (pointers.length > 0) {
        pointers[Math.floor(Math.random() * Math.min(4, pointers.length))].click();
        return { clickedChoice: pointers.length, text };
      }

      // Click next/submit buttons
      const btns = Array.from(c.querySelectorAll("button")).filter(b => b.offsetWidth > 0);
      for (const b of btns) {
        const t = (b.textContent || "").trim().toLowerCase();
        if (t.includes("next") || t.includes("submit") || t.includes("check") || t.includes("lanjut") || t.includes("periksa")) {
          b.click();
          return { clickedBtn: t, text };
        }
      }

      if (text.includes("Hasil") || text.includes("result") || text.includes("score") || text.includes("Nilai") || text.includes("Lulus")) {
        return { quizComplete: true, text };
      }

      return { noAction: true, btnCount: btns.length, text };
    });

    if (state.done || state.stillAuth || state.quizComplete) {
      console.log(`[Q${q}]`, JSON.stringify(state).substring(0, 140));
      break;
    }
    if (q % 8 === 0) console.log(`[Q${q}]`, JSON.stringify(state).substring(0, 100));
    await sleep(350);
  }

  await sleep(5000);

  // Final manual finish attempt
  await page.evaluate(() => {
    const btns = document.querySelectorAll("button");
    for (const b of btns) {
      if (b.offsetWidth > 0) {
        const t = (b.textContent || "").toLowerCase();
        if (t.includes("finish") || t.includes("selesai") || t.includes("review")) {
          b.click();
          return;
        }
      }
    }
  });

  await sleep(8000);

  // Results
  if (capturedResults.length > 0) {
    console.log(`\n[SUCCESS] Captured ${capturedResults.length} real result(s) from iSpring!`);
    capturedResults.forEach((r, i) => {
      if (r.dr && r.dr.length > 500) {
        const name = `kimia-xii-uas-2025-expert-result-${i + 1}.xml`;
        fs.writeFileSync(path.join(FIXTURE_DIR, name), r.dr, "utf8");
        console.log(`[SAVED] ${name} (${r.dr.length} chars) - AUTHENTIC from iSpring`);
      }
      fs.writeFileSync(path.join(FIXTURE_DIR, `kimia-xii-uas-2025-expert-webhook-${i + 1}.json`), JSON.stringify(r, null, 2), "utf8");
    });
  } else {
    console.log("\n[WARN] Still no real XML captured. Saving final debug state...");
    await page.screenshot({ path: path.join(FIXTURE_DIR, "debug-expert-final.png"), fullPage: true });
    fs.writeFileSync(path.join(FIXTURE_DIR, "debug-expert-final.html"), await page.content(), "utf8");
  }

  await browser.close();
  captureServer.close();
  quizServer.close();
  console.log("[DONE] Expert simulation finished.");
}

main().catch(err => {
  console.error("[FATAL]", err);
  process.exit(1);
});
