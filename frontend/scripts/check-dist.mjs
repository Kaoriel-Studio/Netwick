import { existsSync, readFileSync, readdirSync } from "node:fs";

const fail = (m) => { console.error(m); process.exit(1); };
if (!existsSync("dist/index.html")) fail("dist/index.html missing: the window would be empty");
const html = readFileSync("dist/index.html", "utf8");
if (!html.includes("Content-Security-Policy")) fail("CSP missing in dist/index.html");
if (/<script(?![^>]*\bsrc=)[^>]*>/.test(html)) fail("inline script in dist/index.html violates CSP");
if (/<style\b/.test(html)) fail("inline style in dist/index.html violates CSP");
if (/https?:\/\/(?!www\.w3\.org)/.test(html)) fail("external URL in dist/index.html");
const assets = readdirSync("dist/assets");
if (!assets.some((f) => f.endsWith(".js"))) fail("no JS bundle in dist/assets");
console.log("dist ok:", assets.join(", "));
