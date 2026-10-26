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
  home: ["M3 11l9-7 9 7", "M5 10v10h14V10", "M10 20v-6h4v6"],
  wrench: ["M14.7 6.3a4 4 0 0 0 5 5L22 14l-8 8-2.3-2.3a4 4 0 0 0-5-5L4 12l8-8z", "M2 22l6-6"],
  power: ["M12 2v9", "M5.6 6.6a8 8 0 1 0 12.8 0"],
  bolt: ["M13 2L4 14h7l-1 8 9-12h-7z"],
  nodes: ["M5 12h14", "M12 5v14", "M5 5h3v3H5z", "M16 5h3v3h-3z", "M5 16h3v3H5z", "M16 16h3v3h-3z"],
  pin: ["M12 22v-7", "M7 15h10l-2-4V5h1V3H8v2h1v6z"],
  trash: ["M4 7h16", "M10 11v6", "M14 11v6", "M6 7l1 13h10l1-13", "M9 7V4h6v3"],
  broom: ["M14 3l-4 9", "M6 12h10l2 9H4z", "M9 16v5", "M13 16v5"],
  session: ["M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z", "M4 21c0-4 4-6 8-6", "M16 17l4 4", "M20 17l-4 4"],
  radar: ["M12 21a9 9 0 1 1 9-9", "M12 16a4 4 0 1 1 4-4", "M12 12l7-7"],
  reset: ["M3 12a9 9 0 1 0 3-6.7L3 8", "M3 3v5h5"],
  support: ["M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z", "M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8z", "M5 5l4 4", "M15 15l4 4", "M19 5l-4 4", "M9 15l-4 4"],
  globe: ["M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z", "M2 12h20", "M12 2c3 3 4 6.5 4 10s-1 7-4 10c-3-3-4-6.5-4-10s1-7 4-10z"],
  list: ["M9 6h12", "M9 12h12", "M9 18h12", "M4 6h.01", "M4 12h.01", "M4 18h.01"],
  star: ["M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1L3.2 9.5l6.1-.9z"],
  medal: ["M12 15a6 6 0 1 0 0-12 6 6 0 0 0 0 12z", "M8.5 14L7 21l5-3 5 3-1.5-7"],
  flame: ["M12 22c4 0 7-2.8 7-7 0-3-2-5-3.5-7-.4 1.6-1.4 2.6-2.5 3 .5-3-1-6-4-8 .3 3-1.5 4.6-2.6 6.2C5.3 10.7 5 12.8 5 15c0 4.2 3 7 7 7z"],
  trophy: ["M8 4h8v6a4 4 0 0 1-8 0z", "M8 6H4v1a4 4 0 0 0 4 4", "M16 6h4v1a4 4 0 0 1-4 4", "M12 14v4", "M8 21h8", "M10 18h4"],
  signal: ["M4 20v-4", "M9 20v-8", "M14 20V8", "M19 20V4"],
  calendar: ["M4 6h16v14H4z", "M4 10h16", "M8 3v4", "M16 3v4"],
  bulb: ["M9 18h6", "M10 21h4", "M12 3a6 6 0 0 0-3.5 10.9c.6.5 1 1.2 1 2.1h5c0-.9.4-1.6 1-2.1A6 6 0 0 0 12 3z"],
  checkshield: ["M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z", "M8.5 12l2.5 2.5L16 9.5"],
  eye: ["M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12z", "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"],
  heart: ["M12 21s-8-5.2-8-11a4.5 4.5 0 0 1 8-2.8A4.5 4.5 0 0 1 20 10c0 5.8-8 11-8 11z"],
};

const SVG_NS = "http://www.w3.org/2000/svg";

export function svgEl<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Record<string, string> = {}, ...children: SVGElement[]): SVGElementTagNameMap[K] {
  const el = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  for (const c of children) el.appendChild(c);
  return el;
}

export function countUp(el: HTMLElement, to: number, ms = 900): void {
  if (matchMedia("(prefers-reduced-motion: reduce)").matches || to <= 0) {
    el.textContent = String(to);
    return;
  }
  const start = performance.now();
  const tick = (now: number) => {
    const p = Math.min(1, (now - start) / ms);
    const eased = 1 - Math.pow(1 - p, 3);
    el.textContent = String(Math.round(to * eased));
    if (p < 1) requestAnimationFrame(tick);
  };
  requestAnimationFrame(tick);
}

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
