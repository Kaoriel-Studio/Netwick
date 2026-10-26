import { h, icon, mount, statusBadge } from "../dom";
import { t, has } from "../i18n";
import { state, link, modal, refreshUndo, backButton, resetTo, toast, go } from "../app";
import { CheckService, FixService } from "../api";
import type { Step, FixView, Outcome, ApplyOptions, NodesView } from "../api";
import { resultLine, launchGame, homeScreen } from "./check";
import { supportScreen } from "./support";
import { nodesScreen } from "./nodes";

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

type Mark = "done" | "already" | "skipped" | "failed" | "checked" | "not_needed";

interface Ctx {
  steps: Step[];
  index: number;
  context: string;
  title: string;
  sourceUrl: string;
  host: HTMLElement;
  log: { step: Step; mark: Mark }[];
}

export function stepLabel(s: Step): string {
  switch (s.kind) {
    case "fix": return t(`fix.${s.id}.title`);
    case "guide": return t(`guide.${s.id}.title`);
    case "info": return t(`info.${s.id}.title`);
    case "check": return t("search.step_check", { name: t("check." + s.id) });
    default: return t("search.step_article");
  }
}

function dedupe(steps: Step[]): Step[] {
  return steps.filter((s, i) => steps.findIndex((x) => x.kind === s.kind && x.id === s.id) === i);
}

export function wizardScreen(steps: Step[], context: string, title: string, sourceUrl = ""): HTMLElement {
  const clean = dedupe(steps);
  const host = h("div", { class: "wizard-host" });
  const ctx: Ctx = { steps: clean, index: 0, context, title, sourceUrl, host, log: [] };
  const screen = h("section", { class: "screen wizard" },
    backButton(),
    h("p", { class: "kicker", text: context ? context : t("wizard.kicker") }),
    h("h1", { class: "screen-title", text: title }),
    host);
  if (clean.filter((s) => s.kind !== "article").length > 1) void showPlan(ctx);
  else void show(ctx);
  return screen;
}

function tag(cls: string, key: string): HTMLElement {
  return h("span", { class: "tag tag-" + cls, text: t(key) });
}

async function stepTags(s: Step, ctx: Ctx): Promise<HTMLElement[]> {
  switch (s.kind) {
    case "check": return [tag("safe", "tag.safe")];
    case "guide": return [tag("v1", "tag.guide")];
    case "info": return s.id === "reboot" ? [tag("reboot", "tag.reboot")] : [tag("v1", "tag.info")];
    case "article": return [tag("v1", "tag.article")];
  }
  let v: FixView | null = null;
  try {
    v = await FixService.Describe(s.id, ctx.context);
  } catch {
    v = null;
  }
  if (!v) return [];
  const out: HTMLElement[] = [];
  if (!v.allowed) out.push(tag("v1", "tag.not_needed"));
  if (v.requiresAdmin) out.push(tag("admin", "tag.admin"));
  if (v.requiresReboot) out.push(tag("reboot", "tag.reboot"));
  if (v.reversible) out.push(tag("undo", "tag.undo"));
  else if (v.noUndoNeeded) out.push(tag("safe", "tag.no_undo_needed"));
  else out.push(tag("reboot", "tag.no_undo"));
  return out;
}

async function showPlan(ctx: Ctx): Promise<void> {
  const chosen = ctx.steps.map(() => true);
  const items = ctx.steps.map((s, i) => {
    const box = h("input", { type: "checkbox", class: "check-box", id: "plan-" + i });
    box.checked = true;
    const tags = h("span", { class: "plan-tags" });
    const item = h("label", { class: "plan-item", for: "plan-" + i }, box,
      h("span", { class: "plan-num", text: String(i + 1).padStart(2, "0") }),
      h("span", { class: "plan-body" }, h("span", { class: "plan-title", text: stepLabel(s) }), tags));
    box.addEventListener("change", () => {
      chosen[i] = box.checked;
      item.classList.toggle("off", !box.checked);
      start.disabled = !chosen.some(Boolean);
    });
    void stepTags(s, ctx).then((list) => mount(tags, list));
    return item;
  });
  const start = h("button", { class: "btn btn-primary btn-large", type: "button", on: { click: () => {
    ctx.steps = ctx.steps.filter((_, i) => chosen[i]);
    ctx.index = 0;
    void show(ctx);
  } } }, t("wizard.start"));
  mount(ctx.host, h("div", { class: "card step" },
    h("div", { class: "step-head" },
      h("p", { class: "step-count", text: t("wizard.plan_count", { count: ctx.steps.length }) }),
      h("h2", { class: "step-title", text: t("wizard.plan_title") })),
    h("p", { class: "muted", text: t("wizard.plan_text") }),
    h("ul", { class: "plan-list" }, items.map((it) => h("li", null, it))),
    h("div", { class: "actions" }, start,
      h("button", { class: "btn btn-ghost", type: "button", on: { click: () => void resetTo(homeScreen) } }, t("common.cancel")))));
  start.focus();
}

function header(ctx: Ctx, name: string): HTMLElement {
  const total = ctx.steps.length;
  return h("div", { class: "step-head" },
    h("p", { class: "step-count", text: t("wizard.step", { n: ctx.index + 1, total }) }),
    h("div", { class: "segs", role: "progressbar", aria: { label: t("wizard.progress"), valuemin: "0", valuemax: String(total), valuenow: String(ctx.index) } },
      ctx.steps.map((_, i) => h("span", { class: "seg" + (i < ctx.index ? " done" : i === ctx.index ? " cur" : "") }))),
    h("h2", { class: "step-title", text: name }));
}

function record(ctx: Ctx, mark: Mark): void {
  const step = ctx.steps[ctx.index];
  if (!step) return;
  const i = ctx.log.findIndex((l) => l.step === step);
  if (i >= 0) ctx.log[i].mark = mark;
  else ctx.log.push({ step, mark });
}

function next(ctx: Ctx, mark?: Mark): void {
  const step = ctx.steps[ctx.index];
  if (mark || (step && !ctx.log.some((l) => l.step === step))) record(ctx, mark ?? "skipped");
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

function skipBtn(ctx: Ctx): HTMLButtonElement {
  return h("button", { class: "btn btn-ghost", type: "button", on: { click: () => next(ctx, "skipped") } }, t("wizard.skip"));
}

async function showFix(ctx: Ctx, id: string): Promise<void> {
  let view: FixView | null = null;
  try {
    view = await FixService.Describe(id, ctx.context);
  } catch {
    view = null;
  }
  if (!view || !view.allowed) {
    next(ctx, "not_needed");
    return;
  }
  const v = view;
  const undoText = v.noUndoNeeded
    ? t("wizard.undo_not_needed")
    : v.reversible ? t("wizard.undo_yes") : t("wizard.undo_no");
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
  const skip = skipBtn(ctx);
  const picked = new Set<string>();
  const routes = id === "F14" ? routesBlock(picked, doBtn) : null;
  doBtn.addEventListener("click", () => void runFix(ctx, id, v, status, [doBtn, showBtn, skip], id === "F14" ? { hosts: Array.from(picked) } : {}));
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t("fix." + id + ".title")),
    h("dl", { class: "fields" },
      field(t("wizard.what"), t("fix." + id + ".what")),
      field(t("wizard.why"), t("fix." + id + ".why")),
      field(t("wizard.know"), know.join(" ")),
      field(t("wizard.undo"), undoText)),
    v.sourceUrl ? link(t("wizard.source"), v.sourceUrl) : h("p", { class: "muted small", text: t("wizard.source_v1") }),
    routes,
    h("div", { class: "actions" }, doBtn, showBtn, skip),
    status, manual));
  doBtn.focus();
}

function routesBlock(picked: Set<string>, doBtn: HTMLButtonElement): HTMLElement {
  const box = h("div", { class: "nodes", aria: { live: "polite" } }, h("div", { class: "working" }, h("span", { class: "spinner" }), h("span", { text: t("nodes.running") })));
  doBtn.disabled = true;
  void (async () => {
    let view: NodesView | null = null;
    try {
      view = await FixService.Nodes();
    } catch {
      view = null;
    }
    if (!view) {
      mount(box, h("p", { class: "notice notice-fail", text: t("nodes.failed") }));
      return;
    }
    const need = (view.plans ?? []).filter((p) => p.ip && p.pinned !== p.ip);
    if (need.length === 0) {
      mount(box, h("p", { class: "notice notice-ok", text: view.allDown > 0 ? t("nodes.verdict.some_down") : t("nodes.verdict.ok") }),
        h("button", { class: "link link-small", type: "button", on: { click: () => void go(nodesScreen) } }, t("wizard.routes.details")));
      return;
    }
    mount(box, h("p", { class: "small", text: t("wizard.routes.pick_text") }), need.map((p) => {
      const cb = h("input", { type: "checkbox", class: "check-box", id: "rt-" + p.host });
      cb.checked = true;
      picked.add(p.host);
      cb.addEventListener("change", () => {
        if (cb.checked) picked.add(p.host);
        else picked.delete(p.host);
        doBtn.disabled = picked.size === 0;
      });
      return h("label", { class: "node-pick", for: "rt-" + p.host }, cb, h("span", { class: "mono small", text: p.host }), h("span", { class: "small", text: "→ " + t("nodes.pick", { ip: p.ip, ms: p.latencyMs }) }));
    }), h("button", { class: "link link-small", type: "button", on: { click: () => void go(nodesScreen) } }, t("wizard.routes.details")));
    doBtn.disabled = false;
  })();
  return box;
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
    out = await FixService.Apply(id, { onDemand: opts.onDemand ?? "", context: ctx.context, hosts: opts.hosts && opts.hosts.length ? opts.hosts : null });
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
  if (out.declined) {
    mount(status, h("p", { class: "notice notice-warn", text: t("fix.error.declined") }));
    return;
  }
  if (!out.ok) {
    record(ctx, "failed");
    mount(status, h("div", null, h("p", { class: "notice notice-fail", text: t(out.errorKey || "fix.error.generic") }),
      h("div", { class: "actions" }, h("button", { class: "btn btn-secondary", type: "button", on: { click: () => next(ctx, "failed") } }, t("wizard.next")))));
    return;
  }
  record(ctx, out.changed ? "done" : "already");
  await refreshUndo();
  for (const b of buttons) b.hidden = true;
  const msgs = [out.changed ? t("wizard.done") : t("wizard.already")];
  if (out.changed && has("fix." + id + ".after")) msgs.push(t("fix." + id + ".after"));
  if (out.rebootRequired) msgs.push(t("wizard.reboot_now"));
  mount(status, h("p", { class: "notice notice-ok", text: msgs.join(" ") }));
  if (out.rebootRequired) {
    status.appendChild(rebootBlock(ctx, status));
    askHelped(ctx, status, false);
    return;
  }
  if (!out.changed) {
    status.appendChild(h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "button", on: { click: () => next(ctx) } }, t("wizard.next"))));
    return;
  }
  await recheckAndAsk(ctx, status);
}

function rebootBlock(ctx: Ctx, status: HTMLElement): HTMLElement {
  const box = h("div", { class: "actions" });
  const now = h("button", { class: "btn btn-primary", type: "button", on: { click: async () => {
    now.disabled = true;
    const r = await FixService.Reboot().catch(() => null);
    if (!r?.ok) {
      now.disabled = false;
      toast(t("reboot.failed"));
      return;
    }
    if (r.simulated) {
      mount(box, h("p", { class: "notice notice-info", text: t("reboot.simulated") }));
      status.querySelector(".helped")?.remove();
      await recheckAndAsk(ctx, status);
      return;
    }
    mount(box, h("p", { class: "notice notice-warn", text: t("reboot.scheduled") }),
      h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: async () => {
        const ok = await FixService.CancelReboot().catch(() => false);
        toast(ok ? t("reboot.cancelled") : t("reboot.cancel_failed"));
        if (ok) mount(box);
      } } }, t("reboot.cancel")));
  } } }, icon("power"), t("reboot.now"));
  mount(box, now);
  return box;
}

function onDemandBlocked(ctx: Ctx, status: HTMLElement): void {
  mount(status, h("div", { class: "notice notice-info" },
    h("p", { text: t("ondemand.blocked") }),
    h("ol", { class: "steps-list" }, [1, 2, 3].map((i) => h("li", { text: t("ondemand.steps." + i) }))),
    link(t("ondemand.link"), PRECHECK),
    h("div", { class: "actions" }, h("button", { class: "btn btn-secondary", type: "button", on: { click: () => next(ctx, "skipped") } }, t("wizard.next")))));
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
    h("div", { class: "actions" },
      h("button", { class: "btn btn-primary", type: "button", on: { click: () => finish(ctx, true) } }, t("wizard.helped_yes")),
      h("button", { class: "btn btn-secondary", type: "button", on: { click: () => next(ctx) } }, t("wizard.helped_no")),
      h("button", { class: "btn btn-ghost", type: "button", on: { click: async () => {
        status.querySelector(".helped")?.remove();
        status.querySelector(".recheck")?.remove();
        await launchGame();
        await recheckAndAsk(ctx, status);
      } } }, icon("play"), t("wizard.try_game")))));
  status.querySelector<HTMLButtonElement>(".helped .btn-primary")?.focus();
}

async function showGuide(ctx: Ctx, id: string): Promise<void> {
  if (id === "G13") {
    record(ctx, "checked");
    finish(ctx, false);
    return;
  }
  let g;
  try {
    g = await FixService.Guide(id);
  } catch {
    g = null;
  }
  if (!g) {
    next(ctx, "not_needed");
    return;
  }
  const status = h("div", { class: "step-status", aria: { live: "polite" } });
  const steps: string[] = [];
  for (let i = 1; i <= g.steps; i++) steps.push(t(`guide.${id}.s${i}`));
  const opens = (g.opens ?? []).map((o) => h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: () => { FixService.Open(String(o)).catch(() => toast(t("error.open_system"))); if (state.info?.demo) toast(t("demo.opened", { name: t("open." + String(o)) })); } } }, icon("external"), t("open." + String(o))));
  const final = id === "G12";
  const done = h("button", { class: "btn btn-primary", type: "button", on: { click: async () => {
    record(ctx, "done");
    await FixService.CompleteGuide(id).catch(() => false);
    if (final) {
      finish(ctx, false);
      return;
    }
    done.disabled = true;
    await recheckAndAsk(ctx, status);
  } } }, final ? t("wizard.finish") : t("wizard.did_it"));
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t(`guide.${id}.title`)),
    h("p", { text: t(`guide.${id}.intro`) }),
    g.warn ? h("p", { class: "notice notice-warn", text: t(`guide.${id}.warn`) }) : null,
    h("ol", { class: "steps-list" }, steps.map((s) => h("li", { text: s }))),
    opens.length ? h("div", { class: "actions" }, opens) : null,
    g.sourceUrl ? link(t("common.official_article"), g.sourceUrl) : null,
    h("p", { class: "muted small", text: t("wizard.guide_note") }),
    h("div", { class: "actions" }, done, skipBtn(ctx)),
    status));
}

function showInfo(ctx: Ctx, id: string): void {
  const status = h("div", { class: "step-status", aria: { live: "polite" } });
  const isReboot = id === "reboot";
  const did = h("button", { class: isReboot ? "btn btn-secondary" : "btn btn-primary", type: "button", on: { click: async () => {
    record(ctx, "done");
    did.disabled = true;
    await FixService.CompleteGuide(id).catch(() => false);
    await recheckAndAsk(ctx, status);
  } } }, isReboot ? t("info.reboot.did") : t("wizard.did_it"));
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t(`info.${id}.title`)),
    h("p", { text: t(`info.${id}.text`) }),
    SOURCES[id] ? link(t("common.official_article"), SOURCES[id]) : null,
    isReboot ? rebootBlock(ctx, status) : null,
    h("div", { class: "actions" }, did, skipBtn(ctx)),
    status));
}

async function showCheck(ctx: Ctx, id: string): Promise<void> {
  mount(ctx.host, h("div", { class: "card step" }, header(ctx, t("check." + id)), h("div", { class: "working" }, h("span", { class: "spinner" }), h("span", { text: t("wizard.checking") }))));
  let line: HTMLElement | null = null;
  let ok = false;
  let added = 0;
  try {
    const sum = await CheckService.RunOnly([id]);
    const r = sum?.results?.find((x) => x.checkId === id);
    if (r) {
      line = h("ul", { class: "check-list" }, resultLine(r, true));
      ok = String(r.status) === "ok" || String(r.status) === "info";
      if (!ok && (r.steps ?? []).length) {
        const extra = (r.steps ?? []).filter((s) => !ctx.steps.some((x) => x.kind === s.kind && x.id === s.id));
        ctx.steps.splice(ctx.index + 1, 0, ...extra);
        added = extra.length;
      }
    }
  } catch {
    line = null;
  }
  record(ctx, "checked");
  const nextBtn = h("button", { class: "btn btn-primary", type: "button", on: { click: () => next(ctx, "checked") } }, t("wizard.next"));
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t("check." + id)),
    line ?? h("p", { class: "muted", text: t("error.check_failed") }),
    h("p", { class: "muted small", text: ok ? t("wizard.check_ok") : added > 0 ? t("wizard.check_added", { count: added }) : t("wizard.check_problem") }),
    h("div", { class: "actions" }, nextBtn)));
  nextBtn.focus();
}

function showArticle(ctx: Ctx): void {
  mount(ctx.host, h("div", { class: "card step" },
    header(ctx, t("wizard.article_title")),
    h("p", { text: t("wizard.article_text") }),
    ctx.sourceUrl ? link(t("common.official_article"), ctx.sourceUrl) : null,
    h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "button", on: { click: () => next(ctx, "checked") } }, t("wizard.next")))));
}

const MARK_CLASS: Record<Mark, string> = { done: "ok", already: "ok", checked: "ok", skipped: "skip", not_needed: "skip", failed: "fail" };

function finish(ctx: Ctx, helped: boolean): void {
  const level = String(state.last?.verdict.level ?? "");
  const log = ctx.log.filter((l) => l.step.kind !== "article");
  mount(ctx.host, h("div", { class: "card step finish" },
    h("div", { class: "step-head" },
      h("p", { class: "step-count", text: t("wizard.finish_kicker") }),
      h("h2", { class: "step-title", text: helped ? t("wizard.end_helped") : t("wizard.end_nothing") })),
    h("p", { text: helped ? t("wizard.end_helped_text") : t("wizard.end_nothing_text") }),
    level ? h("p", { class: "muted small", text: t("wizard.recheck." + level) }) : null,
    log.length ? h("div", null,
      h("h3", { class: "sub-title", text: t("wizard.summary") }),
      h("ul", { class: "done-list" }, log.map((l) => h("li", null, h("span", { class: "done-mark " + MARK_CLASS[l.mark], text: t("wizard.mark." + l.mark) }), h("span", { text: stepLabel(l.step) }))))) : null,
    h("div", { class: "actions" },
      helped ? h("button", { class: "btn btn-primary", type: "button", on: { click: () => void launchGame() } }, icon("play"), t("result.launch")) : null,
      !helped ? h("button", { class: "btn btn-primary", type: "button", on: { click: () => void go(supportScreen) } }, icon("support"), t("result.contact")) : null,
      !helped && ctx.sourceUrl ? link(t("common.official_article"), ctx.sourceUrl, "btn btn-secondary") : null,
      h("button", { class: "btn btn-ghost", type: "button", on: { click: () => void resetTo(homeScreen) } }, t("nav.home")))));
}
