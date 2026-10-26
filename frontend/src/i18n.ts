import en from "./i18n/en.json";
import ru from "./i18n/ru.json";

type Dict = Record<string, string>;
const dicts: Record<string, Dict> = { en: en as Dict, ru: ru as Dict };

let lang: "ru" | "en" = navigator.language.toLowerCase().startsWith("ru") ? "ru" : "en";

export function setLang(l: string): void {
  if (l === "ru" || l === "en") lang = l;
  else lang = navigator.language.toLowerCase().startsWith("ru") ? "ru" : "en";
  document.documentElement.lang = lang;
}

export function getLang(): "ru" | "en" {
  return lang;
}

export function has(key: string): boolean {
  return key in dicts[lang];
}

export function t(key: string, params?: Record<string, string | number | undefined> | null): string {
  let s = dicts[lang][key] ?? dicts.en[key] ?? key;
  if (params) {
    s = s.replace(/\{(\w+)\}/g, (_m, k: string) => {
      const v = params[k];
      return v === undefined || v === null ? "" : String(v);
    });
  }
  return s;
}

export function tl(text: { ru: string; en: string } | null | undefined): string {
  if (!text) return "";
  return lang === "ru" ? text.ru : text.en;
}

export function dateTime(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return d.toLocaleString(lang === "ru" ? "ru-RU" : "en-GB", { dateStyle: "medium", timeStyle: "short" });
}
