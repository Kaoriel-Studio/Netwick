type Child = Node | string | number | null | undefined | false | Child[];

export interface Props {
  class?: string;
  text?: string;
  id?: string;
  role?: string;
  type?: string;
  title?: string;
  for?: string;
  value?: string;
  placeholder?: string;
  disabled?: boolean;
  hidden?: boolean;
  tabindex?: number;
  attrs?: Record<string, string>;
  aria?: Record<string, string>;
  data?: Record<string, string>;
  on?: Partial<{ [K in keyof HTMLElementEventMap]: (e: HTMLElementEventMap[K]) => void }>;
}

function append(el: Node, child: Child): void {
  if (child === null || child === undefined || child === false) return;
  if (Array.isArray(child)) {
    for (const c of child) append(el, c);
    return;
  }
  if (typeof child === "string" || typeof child === "number") {
    el.appendChild(document.createTextNode(String(child)));
    return;
  }
  el.appendChild(child);
}

export function h<K extends keyof HTMLElementTagNameMap>(tag: K, props: Props | null = null, ...children: Child[]): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  if (props) {
    if (props.class) el.className = props.class;
    if (props.text !== undefined) el.textContent = props.text;
    if (props.id) el.id = props.id;
    if (props.role) el.setAttribute("role", props.role);
    if (props.title) el.title = props.title;
    if (props.type) el.setAttribute("type", props.type);
    if (props.for) el.setAttribute("for", props.for);
    if (props.placeholder) el.setAttribute("placeholder", props.placeholder);
    if (props.value !== undefined && "value" in el) (el as HTMLInputElement).value = props.value;
    if (props.disabled) el.setAttribute("disabled", "");
    if (props.hidden) el.hidden = true;
    if (props.tabindex !== undefined) el.tabIndex = props.tabindex;
    for (const [k, v] of Object.entries(props.attrs ?? {})) el.setAttribute(k, v);
    for (const [k, v] of Object.entries(props.aria ?? {})) el.setAttribute("aria-" + k, v);
    for (const [k, v] of Object.entries(props.data ?? {})) el.dataset[k] = v;
    for (const [k, fn] of Object.entries(props.on ?? {})) {
      if (fn) el.addEventListener(k, fn as EventListener);
    }
  }
  append(el, children);
  return el;
}

export function clear(el: Element): void {
  while (el.firstChild) el.removeChild(el.firstChild);
}

export function mount(el: Element, ...children: Child[]): void {
  clear(el);
  append(el, children);
}

const SVG = "http://www.w3.org/2000/svg";

const paths: Record<string, string[]> = {
  ok: ["M5 12.5l4.2 4.2L19 7"],
  warn: ["M12 8v5", "M12 16.5v.5"],
  fail: ["M8 8l8 8", "M16 8l-8 8"],
  info: ["M12 11v6", "M12 7.5v.5"],
  unknown: ["M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.6v.4", "M12 17v.5"],
  settings: ["M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z", "M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"],
  help: ["M9.1 9a3 3 0 0 1 5.8 1c0 2-3 3-3 3", "M12 17h.01", "M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z"],
  tools: ["M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.8-3.8a6 6 0 0 1-7.9 7.9l-6.9 6.9a2.1 2.1 0 0 1-3-3l6.9-6.9a6 6 0 0 1 7.9-7.9l-3.8 3.8z"],
  back: ["M15 18l-6-6 6-6"],
  external: ["M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6", "M15 3h6v6", "M10 14L21 3"],
  undo: ["M3 7v6h6", "M21 17a9 9 0 0 0-15-6.7L3 13"],
  play: ["M7 4l13 8-13 8z"],
  copy: ["M9 9h10v12H9z", "M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1"],
  search: ["M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16z", "M21 21l-4.3-4.3"],
  chevron: ["M9 18l6-6-6-6"],
  shield: ["M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"],
};

export function icon(name: string, cls = "icon"): SVGSVGElement {
  const svg = document.createElementNS(SVG, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("class", cls);
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("focusable", "false");
  for (const d of paths[name] ?? []) {
    const p = document.createElementNS(SVG, "path");
    p.setAttribute("d", d);
    svg.appendChild(p);
  }
  return svg;
}

export function statusBadge(status: string, label: string): HTMLElement {
  return h("span", { class: `badge badge-${status}` }, icon(statusIcon(status), "badge-icon"), h("span", { class: "sr-only", text: label }));
}

export function statusIcon(status: string): string {
  switch (status) {
    case "ok": return "ok";
    case "warn": return "warn";
    case "fail": return "fail";
    case "info": return "info";
    default: return "unknown";
  }
}
