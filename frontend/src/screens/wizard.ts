import { h, icon, mount, statusBadge } from "../dom";
import { t, has } from "../i18n";
import { state, link, modal, refreshUndo, backButton, resetTo, toast } from "../app";
import { CheckService, FixService } from "../api";
import type { Step, FixView, Outcome, ApplyOptions } from "../api";
import { resultLine, launchGame, homeScreen } from "./check";

const SOURCES: Record<string, string> = {
  reboot: "https://support.riotgames.com/en-us/riot/performance/error-van-57",
  time_sync: "https://support.riotgames.com/en-us/riot/performance/error-van--81",
  usb: "https://support.riotgames.com/en-us/riot/client/error-van-9101",
  practice_range: "https://support.riotgames.com/en-us/riot/client/error-van-9101",
  sign_out_other: "https://support.riotgames.com/en-us/valorant/performance/error-codes-and-solutions-in-valorant",
  uninstall_driver_software: "https://support.riotgames.com/en-us/valorant/performance/van-incompatible-software",
  delete_sys: "https://support.riotgames.com/en-us/valorant/performance/van-incompatible-software",
  riot_id: "https://support.riotgames.com/en-us/valorant/performance/error-codes-and-solutions-in-valorant",
  check_email: "https://support.riotgames.com/en-us/valorant/performance/error-codes-and-solutions-in-valorant",
  server_status: "https://status.riotgames.com/",
  wait: "https://status.riotgames.com/",
};

const PRECHECK = "https://support.riotgames.com/en-us/riot/performance/vanguard-pre-check";

interface Ctx {
  steps: Step[];
  index: number;
  context: string;
  title: string;
  sourceUrl: string;
  host: HTMLElement;
}

export function wizardScreen(steps: Step[], context: string, title: string, sourceUrl = ""): HTMLElement {
  const clean = steps.filter((s, i) => steps.findIndex((x) => x.kind === s.kind && x.id === s.id) === i);
  const host = h("div", { class: "wizard-host" });
  const ctx: Ctx = { steps: clean, index: 0, context, title, sourceUrl, host };
  const screen = h("section", { class: "screen wizard" }, backButton(), h("h1", { class: "screen-title", text: title }), host);
  void show(ctx);
  return screen;
}

function header(ctx: Ctx, name: string): HTMLElement {
  const total = ctx.steps.length;
  return h("div", { class: "step-head" },
    h("p", { class: "step-count", text: t("wizard.step", { n: ctx.index + 1, total }) }),
    h("h2", { class: "step-title", text: name }),
    h("progress", { class: "step-progress", attrs: { max: String(total), value: String(ctx.index) }, aria: { label: t("wizard.progress") } }));
}

function next(ctx: Ctx): void {
  ctx.index++;
  void show(ctx);
}

async function show(ctx: Ctx): Promise<void> {
  if (ctx.index >= ctx.steps.length) {
    finish(ctx, false);
    return;
  }
  const step = ctx.steps[ctx.index];
  switch (step.kind) {
    case "fix":
      await showFix(ctx, step.id);
      return;
    case "guide":
      await showGuide(ctx, step.id);
      return;
    case "info":
      showInfo(ctx, step.id);
      return;
    case "check":
      await showCheck(ctx, step.id);
      return;
    case "article":
      showArticle(ctx);
      return;
    default:
      next(ctx);
  }
}

function field(label: string, text: string): HTMLElement {
  return h("div", { class: "field" }, h("dt", { text: label }), h("dd", { text }));
}

async function showFix(ctx: Ctx, id: string): Promise<void> {
  let view: FixView | null = null;
  try {
    view = await FixService.Describe(id, ctx.context);
  } catch {
    view = null;
  }
  if (!view || !view.allowed) {
    next(ctx);
    return;
  }
  const v = view;
  const undoText = v.id === "F01" || v.id === "F02" || v.id === "F03"
    ? t("wizard.undo_not_needed")
    : v.reversible ? t("wizard.undo_yes") : v.willRestorePoint ? t("wizard.undo_no_rp") : t("wizard.undo_no");
  const know: string[] = [t("fix." + id + ".know")];
  if (v.requiresReboot) know.push(t("wizard.reboot_needed"));
  if (v.requiresAdmin) know.push(t("wizard.admin_needed"));
  if (v.adapter) know.push(t("wizard.adapter", { name: v.adapter }));
  const manual = h("div", { class: "manual", hidden: true },
    h("h3", { class: "sub-title", text: t("wizard.manual_title") }),
    h("ol", { class: "steps-list" }, manualSteps(id).map((s) => h("li", { text: s }))),
    v.sourceUrl ? link(t("common.official_article"), v.sourceUrl) : null);
  const status = h("div", { class: "step-status", aria: { live: "polite" } });
  const doBtn = h("button", { class: "btn btn-primary", type: "button" }, t("wizard.do"));
  const showBtn = h("button", { class: "btn btn-secondary", type: "button", on: { click: () => { manual.hidden = !manual.hidden; } } }, t("wizard.show_me"));
  const skipBtn = h("button", { class: "btn btn-ghost", type: "button", on: { click: () => next(ctx) } }, t("wizard.skip"));
  doBtn.addEventListener("click", () => void runFix(ctx, id, v, status, [doBtn, showBtn, skipBtn], {}));
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t("fix." + id + ".title")),
    h("dl", { class: "fields" },
      field(t("wizard.what"), t("fix." + id + ".what")),
      field(t("wizard.why"), t("fix." + id + ".why")),
      field(t("wizard.know"), know.join(" ")),
      field(t("wizard.undo"), undoText)),
    v.sourceUrl ? link(t("wizard.source"), v.sourceUrl) : null,
    h("div", { class: "actions row wrap" }, doBtn, showBtn, skipBtn),
    status, manual));
}

function manualSteps(id: string): string[] {
  const out: string[] = [];
  for (let i = 1; i <= 8; i++) {
    const k = `fix.${id}.manual${i}`;
    if (!has(k)) break;
    out.push(t(k));
  }
  return out;
}

function busy(buttons: HTMLButtonElement[], on: boolean): void {
  for (const b of buttons) b.disabled = on;
}

async function runFix(ctx: Ctx, id: string, v: FixView, status: HTMLElement, buttons: HTMLButtonElement[], opts: Partial<ApplyOptions>): Promise<void> {
  if (v.askOnDemand && opts.onDemand === undefined) {
    modal(t("ondemand.question.title"), [t("ondemand.question.text")], [
      { label: t("ondemand.answer.no"), primary: true, onClick: () => void runFix(ctx, id, v, status, buttons, { ...opts, onDemand: "no" }) },
      { label: t("ondemand.answer.yes"), onClick: () => onDemandBlocked(ctx, status) },
      { label: t("ondemand.answer.unknown"), onClick: () => onDemandBlocked(ctx, status) },
    ]);
    return;
  }
  busy(buttons, true);
  mount(status, h("div", { class: "working" }, h("span", { class: "spinner" }), h("span", { text: v.requiresAdmin ? t("wizard.waiting_uac") : t("wizard.working") })));
  let out: Outcome | null = null;
  try {
    out = await FixService.Apply(id, { onDemand: opts.onDemand ?? "", context: ctx.context, skipRestorePoint: opts.skipRestorePoint ?? false });
  } catch {
    out = null;
  }
  busy(buttons, false);
  if (!out) {
    mount(status, h("p", { class: "notice notice-fail", text: t("fix.error.generic") }));
    return;
  }
  if (out.blockedOnDemand) {
    onDemandBlocked(ctx, status);
    return;
  }
  if (out.needConfirmNoRestorePoint) {
    modal(t("rp.failed.title"), [t("rp.failed.text")], [
      { label: t("rp.failed.cancel"), primary: true, onClick: () => mount(status, h("p", { class: "notice", text: t("rp.failed.cancelled") })) },
      { label: t("rp.failed.continue"), onClick: () => void runFix(ctx, id, v, status, buttons, { ...opts, skipRestorePoint: true }) },
    ]);
    return;
  }
  if (out.declined) {
    mount(status, h("p", { class: "notice notice-warn", text: t("fix.error.declined") }));
    return;
  }
  if (!out.ok) {
    mount(status, h("div", null, h("p", { class: "notice notice-fail", text: t(out.errorKey || "fix.error.generic") }),
      h("button", { class: "btn btn-secondary", type: "button", on: { click: () => next(ctx) } }, t("wizard.next"))));
    return;
  }
  await refreshUndo();
  for (const b of buttons) b.hidden = true;
  const msgs = [out.changed ? t("wizard.done") : t("wizard.already")];
  if (out.restorePoint === "created") msgs.push(t("rp.created"));
  if (out.rebootRequired) msgs.push(t("wizard.reboot_now"));
  mount(status, h("p", { class: "notice notice-ok", text: msgs.join(" ") }));
  if (out.rebootRequired) {
    askHelped(ctx, status, false);
    return;
  }
  await recheckAndAsk(ctx, status);
}

function onDemandBlocked(ctx: Ctx, status: HTMLElement): void {
  mount(status, h("div", { class: "notice notice-info" },
    h("p", { text: t("ondemand.blocked") }),
    h("ol", { class: "steps-list" }, [1, 2, 3].map((i) => h("li", { text: t("ondemand.steps." + i) }))),
    link(t("ondemand.link"), PRECHECK),
    h("div", { class: "actions" }, h("button", { class: "btn btn-secondary", type: "button", on: { click: () => next(ctx) } }, t("wizard.next")))));
}

async function recheckAndAsk(ctx: Ctx, status: HTMLElement): Promise<void> {
  status.appendChild(h("div", { class: "working" }, h("span", { class: "spinner" }), h("span", { text: t("wizard.rechecking") })));
  try {
    const sum = await CheckService.Run("full", state.focus);
    if (sum) state.last = sum;
    const level = String(sum?.verdict.level ?? "unknown");
    status.lastElementChild?.remove();
    status.appendChild(h("p", { class: "recheck recheck-" + level }, statusBadge(level === "green" ? "ok" : level === "yellow" ? "warn" : "fail", ""), t("wizard.recheck." + level)));
  } catch {
    status.lastElementChild?.remove();
  }
  askHelped(ctx, status, true);
}

function askHelped(ctx: Ctx, status: HTMLElement, rechecked: boolean): void {
  status.appendChild(h("div", { class: "helped" },
    h("p", { class: "strong", text: rechecked ? t("wizard.helped") : t("wizard.helped_after_reboot") }),
    h("div", { class: "row wrap" },
      h("button", { class: "btn btn-primary", type: "button", on: { click: () => finish(ctx, true) } }, t("wizard.helped_yes")),
      h("button", { class: "btn btn-secondary", type: "button", on: { click: () => next(ctx) } }, t("wizard.helped_no")))));
  status.querySelector<HTMLButtonElement>(".helped .btn-primary")?.focus();
}

async function showGuide(ctx: Ctx, id: string): Promise<void> {
  let g;
  try {
    g = await FixService.Guide(id);
  } catch {
    g = null;
  }
  if (!g) {
    next(ctx);
    return;
  }
  const status = h("div", { class: "step-status", aria: { live: "polite" } });
  const steps: string[] = [];
  for (let i = 1; i <= g.steps; i++) steps.push(t(`guide.${id}.s${i}`));
  const opens = (g.opens ?? []).map((o) => h("button", { class: "btn btn-secondary", type: "button", on: { click: () => { FixService.Open(String(o)).catch(() => toast(t("error.open_system"))); } } }, icon("external"), t("open." + String(o))));
  const done = h("button", { class: "btn btn-primary", type: "button", on: { click: async () => {
    await FixService.CompleteGuide(id).catch(() => false);
    if (id === "G12" || id === "G13") {
      finish(ctx, false);
      return;
    }
    await recheckAndAsk(ctx, status);
  } } }, id === "G12" || id === "G13" ? t("wizard.finish") : t("wizard.did_it"));
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t(`guide.${id}.title`)),
    h("p", { text: t(`guide.${id}.intro`) }),
    g.warn ? h("p", { class: "notice notice-warn", text: t(`guide.${id}.warn`) }) : null,
    h("ol", { class: "steps-list" }, steps.map((s) => h("li", { text: s }))),
    opens.length ? h("div", { class: "row wrap" }, opens) : null,
    link(t("common.official_article"), g.sourceUrl),
    h("p", { class: "muted small", text: t("wizard.guide_note") }),
    h("div", { class: "actions row wrap" }, done, h("button", { class: "btn btn-ghost", type: "button", on: { click: () => next(ctx) } }, t("wizard.skip"))),
    status));
}

function showInfo(ctx: Ctx, id: string): void {
  const status = h("div", { class: "step-status", aria: { live: "polite" } });
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t(`info.${id}.title`)),
    h("p", { text: t(`info.${id}.text`) }),
    SOURCES[id] ? link(t("common.official_article"), SOURCES[id]) : null,
    h("div", { class: "actions row wrap" },
      h("button", { class: "btn btn-primary", type: "button", on: { click: async () => {
        await FixService.CompleteGuide(id).catch(() => false);
        if (id === "reboot") {
          mount(status, h("p", { class: "notice notice-info", text: t("info.reboot.after") }));
          askHelped(ctx, status, false);
          return;
        }
        await recheckAndAsk(ctx, status);
      } } }, t("wizard.did_it")),
      h("button", { class: "btn btn-ghost", type: "button", on: { click: () => next(ctx) } }, t("wizard.skip"))),
    status));
}

async function showCheck(ctx: Ctx, id: string): Promise<void> {
  mount(ctx.host, h("div", { class: "card step" }, header(ctx, t("check." + id)), h("div", { class: "working" }, h("span", { class: "spinner" }), h("span", { text: t("wizard.checking") }))));
  let line: HTMLElement | null = null;
  let ok = false;
  try {
    const sum = await CheckService.RunOnly([id]);
    const r = sum?.results?.find((x) => x.checkId === id);
    if (r) {
      line = h("ul", { class: "check-list" }, resultLine(r, true));
      ok = String(r.status) === "ok" || String(r.status) === "info";
      if (!ok && (r.steps ?? []).length) {
        const extra = (r.steps ?? []).filter((s) => !ctx.steps.some((x) => x.kind === s.kind && x.id === s.id));
        ctx.steps.splice(ctx.index + 1, 0, ...extra);
      }
    }
  } catch {
    line = null;
  }
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t("check." + id)),
    line ?? h("p", { class: "muted", text: t("error.check_failed") }),
    h("p", { class: "muted small", text: ok ? t("wizard.check_ok") : t("wizard.check_problem") }),
    h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "button", on: { click: () => next(ctx) } }, t("wizard.next")))));
}

function showArticle(ctx: Ctx): void {
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t("wizard.article_title")),
    h("p", { text: t("wizard.article_text") }),
    ctx.sourceUrl ? link(t("common.official_article"), ctx.sourceUrl) : null,
    h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "button", on: { click: () => next(ctx) } }, t("wizard.next")))));
}

function finish(ctx: Ctx, helped: boolean): void {
  const level = String(state.last?.verdict.level ?? "");
  mount(ctx.host, h("div", { class: "card step finish" },
    h("h2", { class: "step-title", text: helped ? t("wizard.end_helped") : t("wizard.end_nothing") }),
    h("p", { text: helped ? t("wizard.end_helped_text") : t("wizard.end_nothing_text") }),
    level ? h("p", { class: "muted small", text: t("wizard.recheck." + level) }) : null,
    h("div", { class: "actions row wrap" },
      helped ? h("button", { class: "btn btn-primary", type: "button", on: { click: () => void launchGame() } }, icon("play"), t("result.launch")) : null,
      !helped ? h("button", { class: "btn btn-primary", type: "button", on: { click: () => { ctx.steps = [{ kind: "guide", id: "G13" }]; ctx.index = 0; void show(ctx); } } }, t("result.contact")) : null,
      h("button", { class: "btn btn-ghost", type: "button", on: { click: () => void resetTo(homeScreen) } }, t("nav.home")))));
}
