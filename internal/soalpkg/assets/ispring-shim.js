// ispring-shim.js — injected into served iSpring index.html so result submissions are
// redirected to the same-origin Aether webhook with the session context appended, without
// the package needing a hardcoded server URL (Requirements 9.1-9.5).
//
// The shim reads window.__AETHER__ = { webhook, attemptToken, tenantId, sid } which the
// server writes just before this script. It intercepts the browser network layer
// (XMLHttpRequest, fetch, sendBeacon, form submit) so it is resilient to iSpring player
// version differences. Any submission carrying an iSpring result field (dr/sp/tp) is sent
// to __AETHER__.webhook with attempt_token/tenant_id/sid appended.
//
// It also notifies the parent page (the Aether exam screen) via postMessage:
//   - 'aether_result'  fired when a result submission is intercepted, so the page can show
//                      the completion modal even before the server-side queue processes it.
//   - 'aether_progress' { answered, total }  a best-effort count of answered questions so the
//                      page can record periodic progress. The iSpring player markup varies
//                      across versions, so the count is heuristic (checked inputs / answered-
//                      style classes) and is only a coarse signal; the server is authoritative.
(function () {
  "use strict";

  // ---- Layer 3 (mobile-launcher redirect blocker) ----
  // This MUST run before anything else and before the __AETHER__ check below: iSpring's
  // mobile-launcher redirect (location.replace("ismplayer.html")) executes synchronously during
  // parse on iOS/Android, so if we let it through the whole page navigates away before the shim
  // can do anything. The server-side StripMobileLauncherRedirect (layer 1) removes the script
  // block entirely; this is defense-in-depth for the case where a future iSpring version changes
  // the redirect mechanism enough to evade the server regex. We block only navigations to
  // ismplayer.html; everything else passes through to the original implementation. References to
  // the original functions are bound BEFORE the override so the wrapper can still call them.
  (function blockLauncherRedirect() {
    var loc = window.location;
    if (!loc) { return; }
    var origReplace = loc.replace ? loc.replace.bind(loc) : null;
    var origAssign = loc.assign ? loc.assign.bind(loc) : null;
    function isLauncher(url) {
      return typeof url === "string" && url.indexOf("ismplayer.html") >= 0;
    }
    if (origReplace) {
      loc.replace = function (url) {
        if (isLauncher(url)) {
          try { console.warn("[CBT shim] blocked mobile launcher redirect to", url); } catch (e) {}
          return;
        }
        return origReplace(url);
      };
    }
    if (origAssign) {
      loc.assign = function (url) {
        if (isLauncher(url)) {
          try { console.warn("[CBT shim] blocked mobile launcher assign to", url); } catch (e) {}
          return;
        }
        return origAssign(url);
      };
    }
  })();

  var A = window.__AETHER__;
  if (!A || !A.webhook) { return; }

  // Post a message to the parent window (the Aether exam screen). Same-origin (dev proxy /
  // single-port production), so cross-origin is not a concern. Failures are swallowed — the
  // shim's primary job is network redirection; postMessage is a best-effort convenience.
  function notifyParent(type, data) {
    try {
      var msg = { type: type, aether: true };
      if (data) { for (var k in data) { if (Object.prototype.hasOwnProperty.call(data, k)) { msg[k] = data[k]; } } }
      var target = (window.parent && window.parent !== window) ? window.parent : window;
      target.postMessage(msg, "*");
    } catch (e) { /* parent messaging is best-effort */ }
  }

  function hasResult(body) {
    if (body == null) { return false; }
    if (typeof body === "string") { return /(^|&)(dr|sp|tp)=/.test(body); }
    if (typeof FormData !== "undefined" && body instanceof FormData) {
      return body.has("dr") || body.has("sp") || body.has("tp");
    }
    return false;
  }

  // Returns the body with attempt_token/tenant_id/sid appended, preserving its type.
  function enrich(body) {
    var tok = A.attemptToken || "";
    var tid = A.tenantId || "";
    var sid = A.sid || "";
    if (typeof FormData !== "undefined" && body instanceof FormData) {
      var c = new FormData();
      body.forEach(function (v, k) { c.append(k, v); });
      c.append("attempt_token", tok);
      c.append("tenant_id", tid);
      c.append("sid", sid);
      return c;
    }
    var extra = "attempt_token=" + encodeURIComponent(tok) +
      "&tenant_id=" + encodeURIComponent(tid) +
      "&sid=" + encodeURIComponent(sid);
    return (typeof body === "string" && body.length) ? (body + "&" + extra) : extra;
  }

  // XMLHttpRequest
  var XHR = window.XMLHttpRequest;
  if (XHR && XHR.prototype) {
    var oOpen = XHR.prototype.open, oSend = XHR.prototype.send;
    XHR.prototype.open = function (m, u) { return oOpen.apply(this, arguments); };
    XHR.prototype.send = function (body) {
      if (hasResult(body)) {
        try {
          oOpen.call(this, "POST", A.webhook, true);
          var e = enrich(body);
          if (typeof e === "string") { this.setRequestHeader("Content-Type", "application/x-www-form-urlencoded"); }
          var r = oSend.call(this, e);
          notifyParent("aether_result", {});
          return r;
        } catch (err) { /* fall through */ }
      }
      return oSend.apply(this, arguments);
    };
  }

  // fetch
  if (window.fetch) {
    var oFetch = window.fetch;
    window.fetch = function (input, init) {
      init = init || {};
      if (hasResult(init.body)) {
        init = Object.assign({}, init, { method: "POST", body: enrich(init.body) });
        notifyParent("aether_result", {});
        return oFetch.call(this, A.webhook, init);
      }
      return oFetch.apply(this, arguments);
    };
  }

  // navigator.sendBeacon
  if (navigator && typeof navigator.sendBeacon === "function") {
    var oBeacon = navigator.sendBeacon.bind(navigator);
    navigator.sendBeacon = function (url, data) {
      if (hasResult(data)) {
        notifyParent("aether_result", {});
        return oBeacon(A.webhook, enrich(data));
      }
      return oBeacon(url, data);
    };
  }

  // form submit
  if (typeof HTMLFormElement !== "undefined" && HTMLFormElement.prototype) {
    var oSubmit = HTMLFormElement.prototype.submit;
    function hidden(form, name, val) {
      var i = document.createElement("input");
      i.type = "hidden"; i.name = name; i.value = val || "";
      form.appendChild(i);
    }
    HTMLFormElement.prototype.submit = function () {
      try {
        var fd = new FormData(this);
        if (hasResult(fd)) {
          hidden(this, "attempt_token", A.attemptToken);
          hidden(this, "tenant_id", A.tenantId);
          hidden(this, "sid", A.sid);
          this.action = A.webhook;
          notifyParent("aether_result", {});
        }
      } catch (e) { /* fall through */ }
      return oSubmit.apply(this, arguments);
    };
  }

  // ---- Force-submit handler (P2) ----
  // The parent exam page posts { type: 'aether_force_submit', aether: true } when the server-
  // authoritative clock hits 0 (or the local countdown expires). We then drive iSpring's own
  // submit so it composes the result payload from all gathered answers — including ones the
  // student never explicitly submitted in the UI — which flows back through the intercepts above
  // to the webhook. Idempotent via forceSubmitted; a manual submit that already captured the
  // result will have triggered notifyParent('aether_result'), and the player typically refuses a
  // second submit, so double-driving is harmless but we still guard.
  var forceSubmitted = false;
  function forceSubmitQuiz() {
    if (forceSubmitted) { return; }
    forceSubmitted = true;
    // Preferred: iSpring presenter player API (QuizMaker exposes submitQuiz).
    try {
      var player = window.ispringPresenter && window.ispringPresenter.player;
      if (player && typeof player.submitQuiz === "function") {
        player.submitQuiz();
        notifyParent("aether_result", { forced: true });
        return;
      }
    } catch (e) { /* fall through to form fallback */ }
    // Fallback: find the iSpring form carrying a result payload and submit it. This relies on the
    // same hasResult() marker used by the network intercepts, so it only fires on real result forms.
    try {
      for (var i = 0; i < document.forms.length; i++) {
        var form = document.forms[i];
        if (hasResult(new FormData(form))) {
          notifyParent("aether_result", { forced: true });
          form.submit();
          return;
        }
      }
    } catch (e2) { /* no result form available; the player may not have built one yet */ }
    // Last resort: still tell the parent a result was "forced" so it can show the completion modal,
    // even though no payload was sent — the supervisor can then follow up.
    notifyParent("aether_result", { forced: true, empty: true });
  }
  if (typeof window !== "undefined" && typeof window.addEventListener === "function") {
    window.addEventListener("message", function (e) {
      var d = e && e.data;
      if (!d || typeof d !== "object" || d.aether !== true) { return; }
      if (d.type === "aether_force_submit") { forceSubmitQuiz(); }
    });
  }

  // Best-effort progress reporting (Task 47). iSpring QuizMaker's player markup differs by
  // version, so we use a heuristic: count answer inputs that look "answered" (checked radios/
  // checkboxes, non-empty text inputs) versus a total derived from question containers. The
  // numbers are coarse and meant only to let the exam page show live progress; the server's
  // cek_login row remains the source of truth.
  function countAnswered() {
    var doc = document;
    if (!doc || !doc.querySelectorAll) { return null; }
    try {
      var answered = 0;
      // Checked single/multiple-choice inputs.
      answered += doc.querySelectorAll('input[type="radio"]:checked, input[type="checkbox"]:checked').length;
      // Non-empty text inputs commonly used by fill-in / type-in questions.
      doc.querySelectorAll('input[type="text"], input[type="number"], textarea').forEach(function (el) {
        if (el.value && String(el.value).trim().length > 0) { answered += 1; }
      });
      // Heuristic total: distinct question wrappers (iSpring uses elements with class names
      // containing "question" / "slide"). Fallback to answered count so progress never divides
      // by zero or reports an impossible fraction.
      var total = doc.querySelectorAll('[class*="question"], [class*="slide-item"]').length || answered || 0;
      return { answered: answered, total: total };
    } catch (e) { return null; }
  }

  // Poll progress on a light interval. The exam page debounces these into periodic writes, so
  // 2000ms keeps the DOM scans cheap while still feeling live.
  if (typeof window !== "undefined") {
    setInterval(function () {
      var c = countAnswered();
      if (c) { notifyParent("aether_progress", c); }
    }, 2000);
  }
})();
