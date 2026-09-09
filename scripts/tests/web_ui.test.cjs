const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const root = path.resolve(__dirname, "../..");
const staticDir = path.join(root, "internal", "webui", "static");
const html = fs.readFileSync(path.join(staticDir, "index.html"), "utf8");
const css = fs.readFileSync(path.join(staticDir, "styles.css"), "utf8");
const jsPath = path.join(staticDir, "app.js");
const js = fs.readFileSync(jsPath, "utf8");

test("the browser module has valid JavaScript syntax", () => {
  const result = spawnSync(process.execPath, ["--check", jsPath], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
});

test("the shell is offline-only and compatible with the strict CSP", () => {
  assert.match(html, /<link rel="stylesheet" href="\/styles\.css">/);
  assert.match(html, /<script type="module" src="\/app\.js"><\/script>/);
  assert.doesNotMatch(html, /<(script|style)(?![^>]*src=)[^>]*>[^<]+/i);
  assert.doesNotMatch(html + css + js, /https?:\/\//i);
  assert.doesNotMatch(html, /on(?:click|load|error)\s*=/i);
});

test("all documented management API surfaces are wired under same-origin v1", () => {
  assert.match(js, /const API_ROOT = "\/api\/v1"/);
  for (const route of ["/health", "/cell", "/presets", "/config", "/profile", "/connections", "/subscribers", "/sms", "/calls", "/calls/history", "/network"]) {
    assert.ok(js.includes(route), `missing API route ${route}`);
  }
  for (const method of ["POST", "DELETE", "PUT", "PATCH"]) {
    assert.match(js, new RegExp(`method:\\s*['\"]${method}['\"]`), `missing ${method} action`);
  }
  assert.match(js, /fetch\(`\$\{API_ROOT\}\$\{path\}`/);
});

test("authentication is memory/session scoped and never persistent", () => {
  assert.match(js, /sessionStorage\.getItem\("gsm\.bearer"\)/);
  assert.match(js, /headers\.Authorization = `Bearer \$\{state\.token\}`/);
  assert.doesNotMatch(js, /\blocalStorage\s*(?:\.\s*(?:getItem|setItem|removeItem|clear)|\[)/);
  assert.doesNotMatch(html + js, /Bearer\s+[A-Za-z0-9._~+\/-]{8,}/);
});

test("requests time out, cancel, and visible polling cannot overlap", () => {
  assert.match(js, /new AbortController\(\)/);
  assert.match(js, /const POLL_MS = 12_000/);
  assert.match(js, /document\.hidden\|\|state\.busyPoll/);
  assert.match(js, /state\.routeController\?\.abort\(\)/);
  assert.match(js, /state\.pending\.has\(button\)/);
  assert.match(js, /if \(signal\?\.aborted\) abort\(\)/);
  assert.match(js, /previous\?\.cleanup\(\); previous\?\.controller\.abort\(\)/);
  assert.match(js, /removeEventListener\("abort", onParentAbort\)/);
  assert.match(js, /const route=state\.route;.*state\.loadedRoutes\.add\(route\)/s);
});

test("credential changes, confirmations, async actions, and edits are race-safe", () => {
  assert.match(js, /function abortApiRequests\(\).*state\.apiControllers.*controller\.abort\(\)/s);
  assert.match(js, /#clear-token[\s\S]*abortApiRequests\(\)[\s\S]*sessionStorage\.removeItem\("gsm\.bearer"\)[\s\S]*location\.reload\(\)/);
  assert.match(js, /button\.addEventListener\("click", \(\) => Promise\.resolve\(withPending\(button, handler\)\)\.catch\(showError\)\)/);
  assert.match(js, /dialog\.returnValue="";.*dialog\.showModal\(\)/);
  assert.match(js, /customDirty=true/);
  assert.match(js, /!state\.customInitialized && !state\.customDirty/);
  assert.match(js, /previous\.disabled=true;next\.disabled=true;state\.pages\[name\]=Math\.max\(0,page\+delta\)/);
});

test("server-provided values are inserted as text rather than HTML", () => {
  assert.match(js, /node\.textContent =/);
  assert.match(js, /\.replaceChildren\(\)/);
  assert.doesNotMatch(js, /\.innerHTML\s*=/);
  assert.doesNotMatch(js, /insertAdjacentHTML|document\.write/);
});

test("every static translated key exists in both language dictionaries", () => {
  const keys = new Set([...html.matchAll(/data-i18n(?:-aria-label)?="([^"]+)"/g)].map(match => match[1]));
  const zh = js.slice(js.indexOf("zh: {"), js.indexOf("\n  en: {"));
  const en = js.slice(js.indexOf("\n  en: {"));
  for (const key of keys) {
    const property = new RegExp(`(?:^|[,\\s])${key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}:`);
    assert.match(zh, property, `missing Chinese key ${key}`);
    assert.match(en, property, `missing English key ${key}`);
  }
});

test("responsive and accessibility motion rules are present", () => {
  assert.match(html, /class="skip-link"/);
  assert.match(html, /aria-live="polite"/);
  assert.match(css, /:focus-visible/);
  assert.match(css, /@media \(max-width: 830px\)/);
  assert.match(css, /@media \(prefers-reduced-motion: reduce\)/);
});

test("SMS and subscriber inputs mirror the strict API contract", () => {
  assert.match(html, /name="imsi"[^>]*minlength="15"[^>]*maxlength="15"[^>]*pattern="\[0-9\]\{15\}"/);
  assert.match(html, /name="sender"[^>]*minlength="2"[^>]*maxlength="15"[^>]*pattern="\[0-9\]\{2,15\}"/);
  assert.match(html, /name="text"[^>]*maxlength="159"/);
  assert.match(js, /new TextEncoder\(\)\.encode\(payload\.text\)\.length>159/);
  assert.ok(html.includes("101、411、111、112、911、2600、2602"));
});

test("help reflects the checked-in APN and voice diagnostic contract", () => {
  for (const value of [">internet<", ">default<", ">2600<", ">2602<"]) assert.ok(html.includes(value), `missing ${value}`);
  assert.match(html, /data-i18n="versionLiveHint"/);
  assert.match(js, /state\.health\?\.version \|\| state\.meta\?\.version/);
});
