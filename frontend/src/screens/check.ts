import { h, icon, statusBadge, mount } from "../dom";
import { t, has } from "../i18n";
import { state, go, replace, link, toast, refreshUndo, markNav } from "../app";
import { CheckService, FixService, ProfileService, Events, CheckProgress } from "../api";
import type { Result, RunSummary, ProgressEvent, Step } from "../api";
import { wizardScreen } from "./wizard";
import { searchScreen } from "./search";
import { reportScreen } from "./report";
import { supportScreen } from "./support";
import { repairScreen, runScenario, tile } from "./repair";
import { scenario, type RepairScenario } from "./scenarios";
import { profileCard } from "./profile";
import { dailyTip } from "./tip";

export const GROUP_OF: Record<string, string> = {
  C01: "install", C21: "install",
  C02: "windows", C03: "windows", C04: "windows", C05: "windows", C06: "windows", C07: "windows", C15: "windows", C17: "windows", C18: "windows",
  C08: "vanguard", C09: "vanguard",
  C11: "network", C12: "network", C13: "network", C14: "network", C16: "network", C19: "network", C23: "network",
  C10: "riot", C20: "riot", C22: "riot",
};

const GROUP_ORDER = ["install", "windows", "vanguard", "network", "riot"];
const QUICK = ["C01", "C03", "C09", "C10", "C11", "C13", "C18", "C19", "C20", "C22"];

export function resultParams(r: { params?: Record<string, string | undefined> | null }): Record<string, string> {
  const p: Record<string, string> = {};
  for (const [k, v] of Object.entries(r.params ?? {})) p[k] = v ?? "";
  if (p.missing !== undefined) p.missingText = p.missing.split(",").filter(Boolean).map((x) => t("ondemand.req." + x)).join(", ");
  return p;
}

export function resultText(r: Result): string {
  return t(r.messageKey, resultParams(r));
}

export function resultLine(r: Result, expanded = false): HTMLElement {
  const status = String(r.status);
  const details = (r.details ?? []).filter((d) => d.key !== "servers.host" || expanded);
  const body = h("div", { class: "check-body" },
    h("div", { class: "check-title" }, h("span", { text: t("check." + r.checkId) }), h("span", { class: "check-id", text: r.checkId })),
    h("div", { class: "check-msg", text: resultText(r) }),
    details.length ? h("ul", { class: "check-details" }, details.map((d) => h("li", { class: "detail detail-" + String(d.status) }, t(d.key, resultParams(d))))) : null,
    r.sourceUrl ? link(t("common.official_article"), r.sourceUrl, "link link-small") : null);
  return h("li", { class: "check check-" + status }, statusBadge(status, t("status." + status)), body);
}

function profileSlot(): HTMLElement {
  const slot = h("div", { class: "profile-slot" });
  void ProfileService.Get().then((v) => mount(slot, profileCard(v))).catch(() => undefined);
  return slot;
}

export function homeScreen(): HTMLElement {
  const input = h("input", { class: "input", id: "code", type: "text", placeholder: "VAN 68", attrs: { maxlength: "64", autocomplete: "off", spellcheck: "false" }, aria: { describedby: "code-hint" } });
  const find = () => {
    const q = input.value.trim();
    if (q) void go(() => searchScreen(q));
    else input.focus();
  };
  input.addEventListener("keydown", (e) => { if (e.key === "Enter") find(); });
  const chip = (id: string) => h("button", { class: "chip", type: "button", on: { click: () => startCheck("full", id) } }, t("issue." + id));
  const auto = scenario("auto");
  const quick = ["quick", "network", "nodes"].map((id) => scenario(id)).filter((x): x is RepairScenario => !!x);
  return h("section", { class: "screen home" },
    h("div", { class: "hero" },
      h("p", { class: "kicker", text: t("home.kicker") }),
      h("h1", { class: "hero-title" }, t("home.title_1"), " ", h("em", { text: t("home.title_2") })),
      h("p", { class: "hero-sub", text: t("home.subtitle") }),
      h("div", { class: "hero-actions" },
        h("span", { class: "cta-frame" }, h("button", { class: "btn btn-primary", type: "button", on: { click: () => startCheck("full", "") } }, icon("radar"), t("home.check"))),
        auto ? h("button", { class: "btn btn-secondary btn-large", type: "button", on: { click: () => runScenario(auto) } }, icon("bolt"), t("scenario.auto.title")) : null),
      h("p", { class: "hero-hint", text: t("home.check_hint") })),
    profileSlot(),
    h("div", { class: "two-col" },
      h("div", { class: "card code-card" },
        h("h2", { class: "section-title" }, h("label", { for: "code", text: t("home.code_label") })),
        h("div", { class: "row" }, input, h("button", { class: "btn btn-secondary", type: "button", on: { click: find } }, icon("search"), t("home.find"))),
        h("p", { class: "muted small", id: "code-hint", text: t("home.code_hint") })),
      h("div", { class: "card issues" },
        h("h2", { class: "section-title", text: t("home.common") }),
        h("div", { class: "chips" }, chip("wont_launch"), chip("wont_update"), chip("crashes"), chip("high_ping")))),
    h("div", null,
      h("div", { class: "row wrap" },
        h("h2", { class: "section-title grow", text: t("home.repair") }),
        h("button", { class: "btn btn-ghost btn-small", type: "button", on: { click: () => { markNav("repair"); void go(repairScreen); } } }, t("home.repair_all"), icon("chevron"))),
      h("div", { class: "tiles" }, quick.map((sc, i) => tile(sc, i, true)))),
    dailyTip(),
    h("p", { class: "disclaimer-short", text: t("disclaimer.short") }));
}

export async function startCheck(mode: "full" | "quick", focus: string): Promise<void> {
  state.focus = focus;
  await go(() => progressScreen(mode, focus));
}

function progressScreen(mode: "full" | "quick", focus: string): HTMLElement {
  const ids = mode === "quick" ? QUICK : Object.keys(GROUP_OF);
  const total: Record<string, number> = {};
  const done: Record<string, number> = {};
  const worst: Record<string, string> = {};
  for (const id of ids) {
    const g = GROUP_OF[id];
    total[g] = (total[g] ?? 0) + 1;
    done[g] = 0;
  }
  const groups = GROUP_ORDER.filter((g) => total[g]);
  const rows: Record<string, HTMLElement> = {};
  const list = h("ul", { class: "progress-list", aria: { live: "polite" } }, groups.map((g) => {
    const row = h("li", { class: "progress-row pending" }, h("span", { class: "spinner", aria: { hidden: "true" } }), h("span", { class: "progress-label", text: t("group." + g) }), h("span", { class: "progress-state", text: t("progress.checking") }));
    rows[g] = row;
    return row;
  }));
  const bar = h("progress", { class: "progress-bar", attrs: { max: String(ids.length), value: "0" }, aria: { label: t("progress.label") } });
  const sev = (s: string) => ({ fail: 4, warn: 3, unknown: 2, info: 1, ok: 0 } as Record<string, number>)[s] ?? 0;
  let count = 0;
  const off = Events.On(CheckProgress, (ev: { data: ProgressEvent }) => {
    const d = ev.data;
    const g = d.group;
    if (!rows[g]) return;
    count++;
    bar.value = Math.min(count, ids.length);
    done[g] = (done[g] ?? 0) + 1;
    const s = String(d.status) === "info" ? "ok" : String(d.status);
    if (!worst[g] || sev(s) > sev(worst[g])) worst[g] = s;
    if (done[g] >= total[g]) {
      const st = worst[g] ?? "ok";
      const row = rows[g];
      row.className = "progress-row done done-" + st;
      mount(row, statusBadge(st, t("status." + st)), h("span", { class: "progress-label", text: t("group." + g) }), h("span", { class: "progress-state", text: t("progress.state." + st) }));
    }
  });
  const cancel = h("button", { class: "btn btn-secondary", type: "button", on: { click: () => { void CheckService.Cancel(); } } }, t("common.cancel"));
  const screen = h("section", { class: "screen progress" },
    h("h1", { class: "screen-title", text: t("progress.title") }),
    h("p", { class: "muted", text: t("progress.subtitle") }),
    bar, list, h("div", { class: "actions" }, cancel));
  void (async () => {
    try {
      const sum = await CheckService.Run(mode, focus);
      off();
      if (!sum) return;
      if (mode === "full") state.last = sum;
      if (sum.cancelled) {
        toast(t("progress.cancelled"));
      } else if (mode === "full") {
        void announceMedals();
      }
      await replace(() => resultScreen(sum));
    } catch {
      off();
      toast(t("error.check_failed"));
      await replace(homeScreen);
    }
  })();
  return screen;
}

async function announceMedals(): Promise<void> {
  try {
    const v = await ProfileService.Get();
    const fresh = (v.medals ?? []).filter((m) => m.fresh);
    if (fresh.length === 0) return;
    toast(t("medal.new", { name: fresh.map((m) => t(`medal.${m.id}.name`)).join(", ") }));
    await ProfileService.Seen();
  } catch {
    return;
  }
}

const LEVEL_ICON: Record<string, string> = { green: "ok", yellow: "warn", red: "fail", unknown: "unknown" };

export function resultScreen(sum: RunSummary): HTMLElement {
  const v = sum.verdict;
  const level = String(v.level);
  const problems = v.problems ?? [];
  const recs = v.recommendations ?? [];
  const steps: Step[] = v.steps ?? [];
  const top = problems[0];
  const sentence = top ? resultText(top) : t(v.messageKey);
  let primary: HTMLElement;
  switch (v.primaryAction) {
    case "fix":
      primary = h("button", { class: "btn btn-primary btn-large", type: "button", on: { click: () => void go(() => wizardScreen(steps, "", t("wizard.title_fix"))) } }, t("result.fix"));
      break;
    case "show":
      primary = h("button", { class: "btn btn-primary btn-large", type: "button", on: { click: () => void go(() => wizardScreen(steps.length ? steps : (top?.steps ?? []), "", t("wizard.title_improve"))) } }, t("result.show"));
      break;
    default:
      primary = h("button", { class: "btn btn-primary btn-large", type: "button", on: { click: () => void launchGame() } }, icon("play"), t("result.launch"));
  }
  if ((v.primaryAction === "fix" || v.primaryAction === "show") && steps.length === 0 && (top?.steps ?? []).length === 0) {
    primary = h("button", { class: "btn btn-primary btn-large", type: "button", on: { click: () => void go(() => searchScreen("")) } }, t("result.find_code"));
  }
  const all = sum.results ?? [];
  const detailsList = h("ul", { class: "check-list" }, all.slice().sort((a, b) => order(a) - order(b)).map((r) => resultLine(r)));
  const riotSide = top?.checkId === "C22" && String(top.status) === "warn";
  if (riotSide) {
    primary = link(t("result.status_page"), "https://status.riotgames.com/", "btn btn-primary btn-large");
  }
  const verdictCard = h("div", { class: "verdict verdict-" + level, role: "status" },
    h("div", { class: "verdict-light", aria: { hidden: "true" } }, icon(LEVEL_ICON[level] ?? "unknown", "verdict-icon")),
    h("div", { class: "verdict-text" },
      h("h1", { class: "verdict-title", text: riotSide ? t("verdict.title.riot") : t("verdict.title." + level) }),
      h("p", { class: "verdict-sentence", text: level === "green" ? t(v.messageKey) : sentence }),
      problems.length > 1 ? h("p", { class: "muted small", text: t("result.more_problems", { count: problems.length - 1 }) }) : null));
  const problemsBlock = problems.length > 1
    ? h("div", { class: "card" }, h("h2", { class: "section-title", text: t("result.problems") }), h("ul", { class: "check-list compact" }, problems.map((r) => resultLine(r))))
    : null;
  const recBlock = recs.length
    ? h("div", { class: "card rec" }, h("h2", { class: "section-title", text: t("result.recommendations") }),
      h("ul", { class: "check-list compact" }, recs.map((r) => resultLine(r))),
      h("button", { class: "btn btn-secondary", type: "button", on: { click: () => {
        const s: Step[] = [];
        for (const r of recs) for (const st of r.steps ?? []) s.push(st);
        void go(() => wizardScreen(s, "", t("wizard.title_improve")));
      } } }, t("result.apply_recs")))
    : null;
  const notFound = level === "green"
    ? h("div", { class: "card soft" },
      h("p", { class: "strong", text: t("result.not_found_title") }),
      h("p", { class: "muted", text: t("result.not_found_text") }),
      h("div", { class: "row wrap" },
        state.focus === "high_ping" || state.focus === "wont_update" ? h("button", { class: "btn btn-primary", type: "button", on: { click: () => void go(() => searchScreen("CONNECTION")) } }, t("result.network_guide")) : null,
        h("button", { class: "btn btn-secondary", type: "button", on: { click: () => { const a = scenario("auto"); if (a) runScenario(a); } } }, icon("bolt"), t("scenario.auto.title")),
        h("button", { class: "btn btn-secondary", type: "button", on: { click: () => void go(() => searchScreen("")) } }, t("result.find_code")),
        h("button", { class: "btn btn-ghost", type: "button", on: { click: () => void go(supportScreen) } }, icon("support"), t("result.contact"))))
    : null;
  return h("section", { class: "screen result" },
    verdictCard,
    h("div", { class: "actions" }, primary),
    sum.cancelled ? h("p", { class: "notice", text: t("progress.cancelled_note") }) : null,
    v.unknown > 0 ? h("p", { class: "muted small", text: t("result.unknown", { count: v.unknown }) }) : null,
    problemsBlock,
    recBlock,
    h("details", { class: "card details" }, h("summary", { text: t("result.details", { count: all.length }) }), detailsList),
    h("div", { class: "row wrap" },
      h("button", { class: "btn btn-ghost", type: "button", on: { click: () => void go(reportScreen) } }, icon("copy"), t("result.report")),
      h("button", { class: "btn btn-ghost", type: "button", on: { click: () => void startCheck("full", state.focus) } }, t("result.again"))),
    notFound);
}

function order(r: Result): number {
  const s: Record<string, number> = { fail: 0, warn: 1, unknown: 3, info: 2, ok: 4 };
  return (s[String(r.status)] ?? 5) * 100 + parseInt(r.checkId.slice(1), 10);
}

export async function launchGame(): Promise<void> {
  try {
    const out = await FixService.LaunchGame();
    if (out?.ok) toast(t("launch.ok"));
    else toast(t(out?.errorKey || "fix.error.no_client"));
  } catch {
    toast(t("fix.error.no_client"));
  }
  await refreshUndo();
}

export function hasKey(k: string): boolean {
  return has(k);
}
