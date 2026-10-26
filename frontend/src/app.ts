import { h, icon, mount } from "./dom";
import { t, has } from "./i18n";
import { AppService, FixService, DemoService } from "./api";
import type { AppInfo, RunSummary, Settings, UndoItem } from "./api";

export type Screen = () => HTMLElement | Promise<HTMLElement>;

export const state: {
  info: AppInfo | null;
  settings: Settings | null;
  last: RunSummary | null;
  undo: UndoItem[];
  focus: string;
} = { info: null, settings: null, last: null, undo: [], focus: "" };

const stack: Screen[] = [];
let current: Screen | null = null;
let shellMain: HTMLElement | null = null;
let undoSlot: HTMLElement | null = null;
let onUndo: (() => void) | null = null;

export function setUndoHandler(fn: () => void): void {
  onUndo = fn;
}

export function runUndo(): void {
  if (state.undo.length === 0) {
    toast(t("undo.nothing"));
    return;
  }
  onUndo?.();
}

export async function go(screen: Screen, push = true): Promise<void> {
  if (push && current) stack.push(current);
  current = screen;
  await render();
}

export async function replace(screen: Screen): Promise<void> {
  current = screen;
  await render();
}

export async function back(): Promise<void> {
  const prev = stack.pop();
  if (prev) {
    current = prev;
    await render();
  }
}

export function resetTo(screen: Screen): Promise<void> {
  stack.length = 0;
  current = screen;
  return render();
}

export function canGoBack(): boolean {
  return stack.length > 0;
}

export async function rerender(): Promise<void> {
  await render();
}

let renderSeq = 0;

async function render(): Promise<void> {
  if (!shellMain || !current) return;
  const seq = ++renderSeq;
  const el = await current();
  if (seq !== renderSeq) return;
  mount(shellMain, el);
  const heading = shellMain.querySelector<HTMLElement>("h1, h2");
  if (heading) {
    heading.tabIndex = -1;
    heading.focus({ preventScroll: true });
  }
  window.scrollTo(0, 0);
  await refreshUndo();
}

export async function refreshUndo(): Promise<void> {
  try {
    state.undo = (await FixService.UndoList()) ?? [];
  } catch {
    state.undo = [];
  }
  if (!undoSlot) return;
  if (state.undo.length === 0) {
    mount(undoSlot);
    return;
  }
  mount(undoSlot, h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: () => onUndo?.() }, title: t("undo.hint") }, icon("undo"), t("undo.button", { count: state.undo.length })));
}

export function openURL(url: string): void {
  AppService.OpenURL(url).catch(() => toast(t("error.open_url")));
}

export function screenHead(kicker: string, title: string, sub?: string): HTMLElement[] {
  const out: HTMLElement[] = [h("p", { class: "kicker", text: kicker }), h("h1", { class: "screen-title", text: title })];
  if (sub) out.push(h("p", { class: "screen-sub", text: sub }));
  return out;
}

export function link(label: string, url: string, cls = "link"): HTMLElement {
  return h("button", { class: cls + " link-external", type: "button", on: { click: () => openURL(url) } }, label, icon("external", "icon icon-inline"));
}

export function toast(message: string): void {
  const el = h("div", { class: "toast", role: "status", text: message });
  document.body.appendChild(el);
  setTimeout(() => el.classList.add("toast-show"), 10);
  setTimeout(() => {
    el.classList.remove("toast-show");
    setTimeout(() => el.remove(), 300);
  }, 3500);
}

export function applyTheme(theme: string): void {
  const root = document.documentElement;
  if (theme === "light" || theme === "dark") root.dataset.theme = theme;
  else delete root.dataset.theme;
}

export interface ShellHandlers {
  home: () => void;
  repair: () => void;
  tools: () => void;
  settings: () => void;
  help: () => void;
  demoChanged: () => void;
}

const navButtons: Record<string, HTMLButtonElement> = {};

export function markNav(id: string): void {
  for (const [k, b] of Object.entries(navButtons)) {
    if (k === id) b.setAttribute("aria-current", "page");
    else b.removeAttribute("aria-current");
  }
}

function navLink(id: string, iconName: string, label: string, onClick: () => void): HTMLButtonElement {
  const b = h("button", { class: "nav-link", type: "button", title: label, aria: { label }, on: { click: () => { markNav(id); onClick(); } } }, icon(iconName), h("span", { text: label }));
  navButtons[id] = b;
  return b;
}

function demoBar(onChange: () => void): HTMLElement | null {
  if (!state.info?.demo) return null;
  const sel = h("select", { class: "input", id: "demo-scenario", aria: { label: t("demo.scenario_label") } });
  for (const sc of state.info.scenarios ?? []) sel.appendChild(h("option", { value: sc, text: has("demo.scenario." + sc) ? t("demo.scenario." + sc) : sc }));
  sel.value = state.info.scenario;
  sel.addEventListener("change", async () => {
    try {
      const info = await DemoService.SetScenario(sel.value);
      if (state.info) state.info.scenario = info.scenario;
      state.last = null;
      toast(t("demo.switched", { scenario: sel.selectedOptions[0]?.textContent ?? sel.value }));
      onChange();
    } catch {
      sel.value = state.info?.scenario ?? "";
      toast(t("demo.switch_failed"));
    }
  });
  const reboot = h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: async () => {
    reboot.disabled = true;
    await FixService.Reboot().catch(() => null);
    reboot.disabled = false;
    toast(t("demo.rebooted"));
  } } }, icon("power"), t("demo.reboot"));
  return h("div", { class: "demo-bar", role: "region", aria: { label: t("demo.badge_short") } },
    h("span", { class: "demo-tag", text: t("demo.badge_short") }),
    h("label", { class: "label small", for: "demo-scenario", text: t("demo.scenario_label") }),
    sel,
    reboot,
    h("span", { class: "muted small", text: t("demo.hint") }));
}

export function buildShell(root: HTMLElement, handlers: ShellHandlers): void {
  undoSlot = h("div", { class: "undo-slot" });
  shellMain = h("main", { class: "content", id: "content" });
  const header = h("header", { class: "topbar" },
    h("button", { class: "brand", type: "button", on: { click: () => { markNav("home"); handlers.home(); } }, aria: { label: t("nav.home") } },
      h("img", { class: "brand-logo", attrs: { src: "/icon.png", alt: "" } }),
      h("span", { class: "brand-name", text: "Netwick" })),
    h("div", { class: "topbar-spacer" }),
    undoSlot,
    h("nav", { class: "topbar-nav", aria: { label: t("nav.label") } },
      navLink("home", "home", t("nav.home"), handlers.home),
      navLink("repair", "wrench", t("nav.repair"), handlers.repair),
      navLink("tools", "tools", t("nav.tools"), handlers.tools),
      navLink("settings", "settings", t("nav.settings"), handlers.settings),
      navLink("help", "help", t("nav.help"), handlers.help)));
  mount(root, h("a", { class: "skip-link", attrs: { href: "#content" }, text: t("nav.skip") }), header, demoBar(handlers.demoChanged), shellMain);
  markNav("home");
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !document.querySelector(".modal") && canGoBack()) {
      e.preventDefault();
      void back();
    }
  });
}

export function backButton(): HTMLElement | null {
  if (!canGoBack()) return null;
  return h("button", { class: "btn btn-ghost btn-small back", type: "button", on: { click: () => void back() } }, icon("back"), t("nav.back"));
}

export function modal(title: string, body: (HTMLElement | string)[], actions: { label: string; primary?: boolean; onClick: () => void }[]): () => void {
  const prev = document.activeElement as HTMLElement | null;
  const close = () => {
    overlay.remove();
    prev?.focus();
  };
  const buttons = actions.map((a) => h("button", { class: a.primary ? "btn btn-primary" : "btn btn-secondary", type: "button", on: { click: () => { close(); a.onClick(); } } }, a.label));
  const dialog = h("div", { class: "modal", role: "dialog", aria: { modal: "true", labelledby: "modal-title" } },
    h("h2", { id: "modal-title", class: "modal-title", text: title }),
    h("div", { class: "modal-body" }, body.map((b) => typeof b === "string" ? h("p", { text: b }) : b)),
    h("div", { class: "modal-actions" }, buttons));
  const overlay = h("div", { class: "overlay", on: { keydown: (e) => {
    if (e.key === "Escape") { e.stopPropagation(); close(); }
    if (e.key === "Tab") {
      const f = Array.from(dialog.querySelectorAll<HTMLElement>("button"));
      if (f.length === 0) return;
      const first = f[0], last = f[f.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    }
  } } }, dialog);
  document.body.appendChild(overlay);
  buttons[0]?.focus();
  return close;
}
