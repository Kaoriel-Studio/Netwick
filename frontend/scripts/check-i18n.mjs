import { readFileSync } from "node:fs";

const en = JSON.parse(readFileSync("src/i18n/en.json", "utf8"));
const ru = JSON.parse(readFileSync("src/i18n/ru.json", "utf8"));
let failed = 0;
for (const k of Object.keys(en)) {
  if (!(k in ru)) { console.error(`ru.json missing ${k}`); failed++; }
}
for (const k of Object.keys(ru)) {
  if (!(k in en)) { console.error(`en.json missing ${k}`); failed++; }
}
const banned = [/\bRKN\b/i, /\bРКН\b/, /обход блокировок/i, /circumvent/i, /throttl/i, /замедлени/i, /DPI[- ]?(bypass|обход)/i, /обход\s+DPI/i];
for (const [lang, dict] of [["en", en], ["ru", ru]]) {
  for (const [k, v] of Object.entries(dict)) {
    for (const re of banned) {
      if (re.test(v)) { console.error(`${lang}.json ${k}: banned wording ${re}`); failed++; }
    }
  }
}
if (failed) process.exit(1);
console.log(`i18n: ${Object.keys(en).length} keys in sync`);
