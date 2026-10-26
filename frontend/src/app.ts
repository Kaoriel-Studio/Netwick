import { h, icon, mount } from "./dom";
import { t } from "./i18n";
import { AppService, FixService } from "./api";
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

async function render(): Promise<void> {
  if (!shellMain || !current) return;
  const el = await current();
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
  mount(undoSlot, h("button", { class: "btn btn-ghost btn-small", on: { click: () => onUndo?.() }, title: t("undo.hint") }, icon("undo"), t("undo.button", { count: state.undo.length })));
}

export function openURL(url: string): void {
  AppService.OpenURL(url).catch(() => toast(t("error.open_url")));
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
  tools: () => void;
  settings: () => void;
  help: () => void;
}

export function buildShell(root: HTMLElement, handlers: ShellHandlers): void {
  undoSlot = h("div", { class: "undo-slot" });
  shellMain = h("main", { class: "content", id: "content" });
  const demo = state.info?.demo
    ? h("span", { class: "demo-badge", title: t("demo.hint") }, t("demo.badge", { scenario: state.info.scenario }))
    : null;
  const header = h("header", { class: "topbar" },
    h("button", { class: "brand", type: "button", on: { click: handlers.home }, aria: { label: t("nav.home") } },
      h("img", { class: "brand-logo", attrs: { src: "/icon.png", alt: "" } }),
      h("span", { class: "brand-name", text: "Netwick" })),
    demo,
    h("div", { class: "topbar-spacer" }),
    undoSlot,
    h("nav", { class: "topbar-nav", aria: { label: t("nav.label") } },
      h("button", { class: "icon-btn", type: "button", title: t("nav.tools"), aria: { label: t("nav.tools") }, on: { click: handlers.tools } }, icon("tools")),
      h("button", { class: "icon-btn", type: "button", title: t("nav.settings"), aria: { label: t("nav.settings") }, on: { click: handlers.settings } }, icon("settings")),
      h("button", { class: "icon-btn", type: "button", title: t("nav.help"), aria: { label: t("nav.help") }, on: { click: handlers.help } }, icon("help"))));
  mount(root, h("a", { class: "skip-link", attrs: { href: "#content" }, text: t("nav.skip") }), header, shellMain);
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
