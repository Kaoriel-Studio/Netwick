import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const rules = [
  [/\binnerHTML\b/, "innerHTML"],
  [/\bouterHTML\b/, "outerHTML"],
  [/\binsertAdjacentHTML\b/, "insertAdjacentHTML"],
  [/\bdocument\.write(ln)?\b/, "document.write"],
  [/\beval\s*\(/, "eval"],
  [/\bnew\s+Function\b/, "new Function"],
  [/\bsetTimeout\s*\(\s*["'`]/, "setTimeout with string"],
  [/\bsetInterval\s*\(\s*["'`]/, "setInterval with string"],
  [/\bwindow\.open\s*\(/, "window.open (use Go OpenURL)"],
  [/\bfetch\s*\(\s*["'`]https?:/, "external fetch"],
  [/\bhref\s*=\s*["'`]https?:/, "external href"],
  [/\bsrc\s*=\s*["'`]https?:/, "external src"],
  [/\blocalStorage\b/, "localStorage (settings live in Go)"],
];

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, out);
    else if (/\.(ts|js|html)$/.test(name)) out.push(p);
  }
  return out;
}

const files = [...walk("src"), "index.html"];
let failed = 0;
for (const f of files) {
  const lines = readFileSync(f, "utf8").split("\n");
  lines.forEach((line, i) => {
    for (const [re, name] of rules) {
      if (re.test(line)) {
        console.error(`${f}:${i + 1}: forbidden ${name}`);
        failed++;
      }
    }
  });
}
const html = readFileSync("index.html", "utf8");
if (!html.includes("default-src 'self'; script-src 'self'; style-src 'self'")) {
  console.error("index.html: CSP meta tag missing or changed");
  failed++;
}
if (failed) {
  console.error(`security lint: ${failed} problem(s)`);
  process.exit(1);
}
console.log(`security lint: ${files.length} files clean`);
