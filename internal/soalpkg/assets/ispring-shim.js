// ispring-shim.js — injected into served iSpring index.html so result submissions are
// redirected to the same-origin Aether webhook with the session context appended, without
// the package needing a hardcoded server URL (Requirements 9.1-9.5).
//
// The shim reads window.__AETHER__ = { webhook, attemptToken, tenantId, sid } which the
// server writes just before this script. It intercepts the browser network layer
// (XMLHttpRequest, fetch, sendBeacon, form submit) so it is resilient to iSpring player
// version differences. Any submission carrying an iSpring result field (dr/sp/tp) is sent
// to __AETHER__.webhook with attempt_token/tenant_id/sid appended.
(function () {
  "use strict";
  var A = window.__AETHER__;
  if (!A || !A.webhook) { return; }

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
          return oSend.call(this, e);
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
        return oFetch.call(this, A.webhook, init);
      }
      return oFetch.apply(this, arguments);
    };
  }

  // navigator.sendBeacon
  if (navigator && typeof navigator.sendBeacon === "function") {
    var oBeacon = navigator.sendBeacon.bind(navigator);
    navigator.sendBeacon = function (url, data) {
      if (hasResult(data)) { return oBeacon(A.webhook, enrich(data)); }
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
        }
      } catch (e) { /* fall through */ }
      return oSubmit.apply(this, arguments);
    };
  }
})();
