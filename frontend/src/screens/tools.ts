import { h, icon, mount } from "../dom";
import { t, dateTime } from "../i18n";
import { link, toast, backButton, state } from "../app";
import { CheckService, ToolsService } from "../api";
import type { Crosshair, Result } from "../api";
import { resultLine, launchGame, resultParams } from "./check";

const TABS = ["ready", "ondemand", "crosshair", "sens", "connection", "links"] as const;
type Tab = typeof TABS[number];

export function toolsScreen(initial: Tab = "ready"): HTMLElement {
  const panel = h("div", { class: "tab-panel", role: "tabpanel", id: "tool-panel" });
  const buttons: HTMLButtonElement[] = [];
  const select = (tab: Tab) => {
    for (const b of buttons) {
      const on = b.dataset.tab === tab;
      b.setAttribute("aria-selected", String(on));
      b.tabIndex = on ? 0 : -1;
    }
    mount(panel, render(tab));
  };
  const list = h("div", { class: "tabs", role: "tablist", aria: { label: t("tools.title") } }, TABS.map((tab, i) => {
    const b = h("button", { class: "tab", role: "tab", type: "button", data: { tab }, aria: { controls: "tool-panel" }, on: {
      click: () => select(tab),
      keydown: (e) => {
        if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
          const n = (i + (e.key === "ArrowRight" ? 1 : TABS.length - 1)) % TABS.length;
          buttons[n].focus();
          select(TABS[n]);
        }
      },
    } }, t("tools.tab." + tab));
    buttons.push(b);
    return b;
  }));
  select(initial);
  return h("section", { class: "screen tools" }, backButton(), h("h1", { class: "screen-title", text: t("tools.title") }), list, panel);
}

function render(tab: Tab): HTMLElement {
  switch (tab) {
    case "ready": return readyTool();
    case "ondemand": return onDemandTool();
    case "crosshair": return crosshairTool();
    case "sens": return sensTool();
    case "connection": return connectionTool();
    default: return linksTool();
  }
}

function spinner(text: string): HTMLElement {
  return h("div", { class: "working" }, h("span", { class: "spinner" }), h("span", { text }));
}

function readyTool(): HTMLElement {
  const out = h("div", { aria: { live: "polite" } });
  const run = async () => {
    mount(out, spinner(t("tools.ready.running")));
    try {
      const sum = await CheckService.Run("quick", "");
      if (!sum) return;
      const level = String(sum.verdict.level);
      const shown = (sum.results ?? []).filter((r) => ["C03", "C09", "C10", "C11", "C13", "C18"].includes(r.checkId));
      mount(out,
        h("p", { class: "verdict-inline verdict-" + level, text: t("tools.ready." + level) }),
        h("ul", { class: "check-list compact" }, shown.map((r) => resultLine(r))),
        h("button", { class: "btn btn-primary", type: "button", on: { click: () => void launchGame() } }, icon("play"), t("result.launch")));
    } catch {
      mount(out, h("p", { class: "notice notice-fail", text: t("error.check_failed") }));
    }
  };
  return h("div", { class: "card" },
    h("h2", { class: "section-title", text: t("tools.ready.title") }),
    h("p", { class: "muted", text: t("tools.ready.text") }),
    h("button", { class: "btn btn-primary", type: "button", on: { click: () => void run() } }, t("tools.ready.button")),
    out);
}

function onDemandTool(): HTMLElement {
  const out = h("div", { aria: { live: "polite" } });
  const reqs = ["win11_25h2", "tpm", "secureboot", "iommu", "vbs", "hvci"];
  void (async () => {
    mount(out, spinner(t("wizard.checking")));
    try {
      const sum = await CheckService.RunOnly(["C08"]);
      const r = sum?.results?.find((x) => x.checkId === "C08");
      if (!r) throw new Error("no result");
      const p = resultParams(r);
      const missing = (p.missing ?? "").split(",").filter(Boolean);
      const unknown = (p.unknown ?? "").split(",").filter(Boolean);
      mount(out,
        h("p", { class: "strong", text: t(r.messageKey, p) }),
        h("ul", { class: "req-list" }, reqs.map((q) => {
          const st = missing.includes(q) ? "fail" : unknown.includes(q) ? "unknown" : "ok";
          return h("li", { class: "req req-" + st }, h("span", { class: "badge badge-" + st }, icon(st === "fail" ? "fail" : st === "ok" ? "ok" : "unknown", "badge-icon")), h("span", { text: t("ondemand.req." + q) }), h("span", { class: "sr-only", text: t("status." + st) }));
        })));
    } catch {
      mount(out, h("p", { class: "notice notice-fail", text: t("error.check_failed") }));
    }
  })();
  return h("div", { class: "card" },
    h("h2", { class: "section-title", text: t("tools.ondemand.title") }),
    h("p", { text: t("tools.ondemand.text") }),
    out,
    h("p", { class: "muted small", text: t("tools.ondemand.note") }),
    link(t("ondemand.link"), "https://support.riotgames.com/en-us/riot/performance/vanguard-pre-check"));
}

function crosshairTool(): HTMLElement {
  const list = h("ul", { class: "cross-list" });
  const name = h("input", { class: "input", id: "xh-name", type: "text", attrs: { maxlength: "40", autocomplete: "off" } });
  const code = h("input", { class: "input mono", id: "xh-code", type: "text", attrs: { maxlength: "400", autocomplete: "off", spellcheck: "false" } });
  const draw = (items: Crosshair[]) => {
    if (items.length === 0) {
      mount(list, h("li", { class: "muted", text: t("tools.cross.empty") }));
      return;
    }
    mount(list, items.map((c) => h("li", { class: "cross" },
      h("div", { class: "cross-info" }, h("span", { class: "strong", text: c.name }), h("code", { class: "cross-code", text: c.code }), h("span", { class: "muted small", text: dateTime(c.createdAt) })),
      h("div", { class: "row" },
        h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: async () => {
          const ok = await ToolsService.CopyCrosshair(c.id).catch(() => false);
          toast(ok ? t("tools.cross.copied") : t("report.copy_failed"));
        } } }, icon("copy"), t("tools.cross.copy")),
        h("button", { class: "btn btn-ghost btn-small", type: "button", aria: { label: t("tools.cross.delete") + " " + c.name }, on: { click: async () => draw((await ToolsService.DeleteCrosshair(c.id).catch(() => null)) ?? items) } }, t("tools.cross.delete"))))));
  };
  void ToolsService.Crosshairs().then((x) => draw(x ?? [])).catch(() => draw([]));
  const add = async () => {
    try {
      const items = await ToolsService.AddCrosshair(name.value, code.value);
      name.value = "";
      code.value = "";
      draw(items ?? []);
    } catch {
      toast(t("tools.cross.invalid"));
    }
  };
  return h("div", { class: "card" },
    h("h2", { class: "section-title", text: t("tools.cross.title") }),
    h("p", { class: "muted", text: t("tools.cross.text") }),
    h("div", { class: "form-grid" },
      h("label", { class: "label", for: "xh-name", text: t("tools.cross.name") }), name,
      h("label", { class: "label", for: "xh-code", text: t("tools.cross.code") }), code),
    h("button", { class: "btn btn-primary", type: "button", on: { click: () => void add() } }, t("tools.cross.add")),
    list);
}

function sensTool(): HTMLElement {
  const dpi = h("input", { class: "input", id: "s-dpi", type: "number", value: "800", attrs: { min: "50", max: "64000", step: "50", inputmode: "numeric" } });
  const sens = h("input", { class: "input", id: "s-sens", type: "number", value: "0.4", attrs: { min: "0.001", max: "1000", step: "0.01", inputmode: "decimal" } });
  const from = h("select", { class: "input", id: "s-from" });
  const to = h("select", { class: "input", id: "s-to" });
  const out = h("div", { class: "sens-out", aria: { live: "polite" } });
  const calc = async () => {
    try {
      const r = await ToolsService.Sensitivity(parseFloat(dpi.value), parseFloat(sens.value), from.value, to.value);
      if (!r) return;
      mount(out,
        h("div", { class: "stat" }, h("span", { class: "stat-value", text: String(r.edpi) }), h("span", { class: "stat-label", text: "eDPI" })),
        h("div", { class: "stat" }, h("span", { class: "stat-value", text: String(r.cm360) }), h("span", { class: "stat-label", text: t("tools.sens.cm360") })),
        r.target ? h("div", { class: "stat" }, h("span", { class: "stat-value", text: String(r.converted) }), h("span", { class: "stat-label", text: t("tools.sens.converted", { game: to.selectedOptions[0]?.textContent ?? "" }) })) : null);
    } catch {
      mount(out, h("p", { class: "notice notice-warn", text: t("tools.sens.invalid") }));
    }
  };
  void ToolsService.SensGames().then((games) => {
    for (const g of games ?? []) {
      from.appendChild(h("option", { value: g.id, text: g.name }));
      to.appendChild(h("option", { value: g.id, text: g.name }));
    }
    if (to.options.length > 1) to.selectedIndex = 1;
    void calc();
  });
  for (const el of [dpi, sens, from, to]) el.addEventListener("input", () => void calc());
  return h("div", { class: "card" },
    h("h2", { class: "section-title", text: t("tools.sens.title") }),
    h("div", { class: "form-grid" },
      h("label", { class: "label", for: "s-dpi", text: t("tools.sens.dpi") }), dpi,
      h("label", { class: "label", for: "s-sens", text: t("tools.sens.sens") }), sens,
      h("label", { class: "label", for: "s-from", text: t("tools.sens.from") }), from,
      h("label", { class: "label", for: "s-to", text: t("tools.sens.to") }), to),
    out,
    h("p", { class: "muted small", text: t("tools.sens.note") }));
}

function connectionTool(): HTMLElement {
  const out = h("div", { aria: { live: "polite" } });
  const run = async () => {
    mount(out, spinner(t("tools.ready.running")));
    try {
      const sum = await CheckService.RunOnly(["C13", "C14", "C10"]);
      const get = (id: string): Result | undefined => sum?.results?.find((r) => r.checkId === id);
      const tips: HTMLElement[] = [];
      const c13 = get("C13"), c14 = get("C14"), c10 = get("C10");
      if (c13?.messageKey === "conn.wifi") tips.push(h("li", { text: t("tools.conn.tip_wifi") }));
      if (c14?.messageKey === "hogs.found") tips.push(h("li", { text: t("tools.conn.tip_hogs", resultParams(c14)) }));
      if (tips.length === 0) tips.push(h("li", { text: t("tools.conn.tip_none") }));
      const lat = c10?.params?.latency;
      mount(out,
        lat ? h("div", { class: "stat" }, h("span", { class: "stat-value", text: lat + " ms" }), h("span", { class: "stat-label", text: t("tools.conn.latency") })) : null,
        h("ul", { class: "tips" }, tips),
        h("ul", { class: "check-list compact" }, [c13, c14, c10].filter((x): x is Result => !!x).map((r) => resultLine(r))));
    } catch {
      mount(out, h("p", { class: "notice notice-fail", text: t("error.check_failed") }));
    }
  };
  return h("div", { class: "card" },
    h("h2", { class: "section-title", text: t("tools.conn.title") }),
    h("p", { class: "muted", text: t("tools.conn.text") }),
    h("button", { class: "btn btn-primary", type: "button", on: { click: () => void run() } }, t("tools.conn.button")),
    out,
    link(t("common.official_article"), "https://support.riotgames.com/en-us/riot/performance/troubleshoot-pc-connection"));
}

function linksTool(): HTMLElement {
  const list = h("ul", { class: "link-list" });
  void ToolsService.Links().then((links) => mount(list, (links ?? []).map((l) => h("li", null, link(t("links." + l.id), l.url))))).catch(() => undefined);
  return h("div", { class: "card" }, h("h2", { class: "section-title", text: t("tools.links.title") }), list, state.info?.demo ? h("p", { class: "muted small", text: t("tools.links.demo") }) : null);
}
