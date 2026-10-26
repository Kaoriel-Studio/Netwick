import { h, icon, mount } from "../dom";
import { t, tl } from "../i18n";
import { go, link, backButton, screenHead } from "../app";
import { KBService } from "../api";
import type { KBEntry, KBMatch, Step } from "../api";
import { wizardScreen } from "./wizard";

function stepLabel(s: Step): string {
  switch (s.kind) {
    case "fix": return t(`fix.${s.id}.title`);
    case "guide": return t(`guide.${s.id}.title`);
    case "info": return t(`info.${s.id}.title`);
    case "check": return t("search.step_check", { name: t("check." + s.id) });
    default: return t("search.step_article");
  }
}

export function entryView(e: KBEntry): HTMLElement {
  const steps = (e.steps ?? []).filter((s) => s.kind !== "article");
  const runnable = steps.length > 0;
  return h("div", { class: "card entry" },
    h("p", { class: "entry-code", text: e.code }),
    h("h2", { class: "entry-title", text: tl(e.title) }),
    h("p", { text: tl(e.explain) }),
    runnable ? h("div", null,
      h("h3", { class: "sub-title", text: t("search.steps") }),
      h("ol", { class: "steps-list" }, steps.map((s) => h("li", { text: stepLabel(s) })))) : null,
    h("div", { class: "actions row wrap" },
      runnable ? h("button", { class: "btn btn-primary", type: "button", on: { click: () => void go(() => wizardScreen(e.steps ?? [], e.code, e.code + " · " + tl(e.title), e.sourceUrl)) } }, t("search.run")) : null,
      link(t("search.article"), e.sourceUrl, "btn btn-secondary")),
    h("p", { class: "muted small", text: t("search.verified", { date: e.verifiedAt }) }));
}

export function searchScreen(initial: string): HTMLElement {
  const input = h("input", { class: "input", id: "q", type: "text", value: initial, placeholder: t("search.placeholder"), attrs: { maxlength: "64", autocomplete: "off", spellcheck: "false" } });
  const results = h("div", { class: "search-results", aria: { live: "polite" } });
  const run = async () => {
    const q = input.value.trim();
    if (!q) {
      mount(results);
      return;
    }
    let matches: KBMatch[] = [];
    try {
      matches = (await KBService.Search(q)) ?? [];
    } catch {
      matches = [];
    }
    if (matches.length === 0) {
      mount(results, h("div", { class: "card soft" }, h("p", { class: "strong", text: t("search.none", { q }) }), h("p", { class: "muted", text: t("search.none_hint") })));
      return;
    }
    if (matches.length === 1) {
      mount(results, entryView(matches[0].entry));
      return;
    }
    mount(results, h("p", { class: "muted", text: t("search.many") }), h("ul", { class: "match-list" }, matches.map((m) => h("li", null,
      h("button", { class: "match", type: "button", on: { click: () => mount(results, h("button", { class: "btn btn-ghost btn-small", type: "button", on: { click: () => void run() } }, icon("back"), t("search.back_results")), entryView(m.entry)) } },
        h("span", { class: "match-code", text: m.entry.code }), h("span", { class: "match-title", text: tl(m.entry.title) }), icon("chevron"))))));
  };
  input.addEventListener("keydown", (e) => { if (e.key === "Enter") void run(); });
  if (initial) void run();
  return h("section", { class: "screen search" },
    backButton(),
    ...screenHead(t("search.kicker"), t("search.title")),
    h("div", { class: "row" }, input, h("button", { class: "btn btn-primary", type: "button", on: { click: () => void run() } }, icon("search"), t("home.find"))),
    h("p", { class: "muted small", text: t("search.examples") }),
    h("p", { class: "notice notice-info small", text: t("search.caveat") }),
    results);
}
