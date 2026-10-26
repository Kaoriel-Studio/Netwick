import { h, icon, mount, svgEl, countUp } from "../dom";
import { t, dateTime } from "../i18n";
import { go, backButton, screenHead, modal, toast } from "../app";
import { ProfileService } from "../api";
import type { ProfileView, Point } from "../api";

const MEDAL_ICON: Record<string, string> = {
  first_check: "radar", clean_bill: "checkshield", fixer: "wrench", safe_hands: "undo", scout: "nodes",
  tester: "signal", streak3: "flame", streak7: "calendar", veteran: "trophy",
};

function gaugeClass(score: number): string {
  return score >= 85 ? "g-high" : score >= 60 ? "g-mid" : "g-low";
}

export function gauge(score: number): HTMLElement {
  const value = h("span", { class: "gauge-value", text: "0" });
  const el = h("div", { class: "gauge " + gaugeClass(score), role: "img", aria: { label: `${score} ${t("profile.score")}` } }, value);
  requestAnimationFrame(() => el.style.setProperty("--pct", String(score)));
  countUp(value, score, 1000);
  return el;
}

export function sparkline(points: Point[]): SVGElement {
  const w = 360, hgt = 46, pad = 5;
  const svg = svgEl("svg", { viewBox: `0 0 ${w} ${hgt}`, class: "spark", role: "img", "aria-label": t("profile.trend") });
  if (points.length === 0) return svg;
  const x = (i: number) => points.length === 1 ? w / 2 : pad + (i * (w - 2 * pad)) / (points.length - 1);
  const y = (s: number) => hgt - pad - (s / 100) * (hgt - 2 * pad);
  const line = points.map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)} ${y(p.score).toFixed(1)}`).join(" ");
  if (points.length > 1) {
    svg.appendChild(svgEl("path", { class: "area", d: `${line} L${x(points.length - 1).toFixed(1)} ${hgt} L${x(0).toFixed(1)} ${hgt} Z` }));
    svg.appendChild(svgEl("path", { d: line }));
  }
  svg.appendChild(svgEl("circle", { cx: x(points.length - 1).toFixed(1), cy: y(points[points.length - 1].score).toFixed(1), r: "3.5" }));
  return svg;
}

function names(ids: string[]): string {
  return ids.map((id) => t("check." + id)).join(", ");
}

function changes(v: ProfileView): HTMLElement | null {
  const fixed = v.fixed ?? [];
  const added = v.added ?? [];
  if (fixed.length === 0 && added.length === 0) return null;
  return h("div", { class: "health-changes small" },
    fixed.length ? h("p", { class: "strong", text: t("profile.fixed", { list: names(fixed) }) }) : null,
    added.length ? h("p", { class: "muted", text: t("profile.added", { list: names(added) }) }) : null);
}

function deltaText(v: ProfileView): string {
  if ((v.points ?? []).length < 2) return "";
  if (v.delta > 0) return t("profile.delta_up", { n: v.delta });
  if (v.delta < 0) return t("profile.delta_down", { n: v.delta });
  return t("profile.delta_same");
}

export function profileCard(v: ProfileView, standalone = false): HTMLElement {
  if (!v.hasRuns) {
    return h("button", { class: "card profile-empty tile", type: "button", on: { click: () => void go(profileScreen) } },
      h("span", { class: "tile-head" }, icon("heart")),
      h("span", { class: "tile-title", text: t("profile.open") }),
      h("span", { class: "tile-text", text: t("profile.card_empty") }));
  }
  const earned = (v.medals ?? []).filter((m) => m.earned).length;
  return h("div", { class: "card" },
    h("div", { class: "health" },
      gauge(v.score),
      h("div", { class: "health-body" },
        standalone ? null : h("h2", { class: "section-title", text: t("profile.title") }),
        h("p", { class: "strong", text: t("profile.level." + v.level) }),
        deltaText(v) ? h("p", { class: "muted small", text: deltaText(v) }) : null,
        sparkline(v.points ?? []),
        changes(v))),
    h("div", { class: "actions" },
      standalone ? null : h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: () => void go(profileScreen) } }, icon("medal"), t("profile.open"), " · ", t("profile.medals_count", { n: earned, total: (v.medals ?? []).length })),
      v.streak > 0 ? h("span", { class: "tag tag-v1" }, icon("flame", "icon icon-inline"), t("profile.streak", { n: v.streak })) : null));
}

export async function profileScreen(): Promise<HTMLElement> {
  let v: ProfileView | null = null;
  try {
    v = await ProfileService.Get();
  } catch {
    v = null;
  }
  const body = h("div", { class: "screen-body" });
  const head = [backButton(), ...screenHead(t("profile.kicker"), t("profile.title"), t("profile.subtitle"))];
  if (!v) return h("section", { class: "screen profile" }, ...head, h("p", { class: "notice notice-fail", text: t("error.check_failed") }));
  const medals = v.medals ?? [];
  const earned = medals.filter((m) => m.earned).length;
  const reset = h("button", { class: "btn btn-ghost btn-small", type: "button", on: { click: () => {
    modal(t("profile.reset"), [t("profile.reset_text")], [
      { label: t("profile.reset"), primary: true, onClick: async () => {
        await ProfileService.Reset().catch(() => undefined);
        toast(t("profile.reset_done"));
        mount(body, ...await profileBody());
      } },
      { label: t("common.cancel"), onClick: () => undefined },
    ]);
  } } }, icon("trash"), t("profile.reset"));
  const profileBody = async (): Promise<HTMLElement[]> => {
    const cur = (await ProfileService.Get().catch(() => null)) ?? v!;
    return [
      cur.hasRuns ? profileCard(cur, true) : h("div", { class: "card soft" }, h("p", { class: "strong", text: t("profile.empty_title") }), h("p", { class: "muted", text: t("profile.empty_text") })),
      cur.hasRuns ? h("p", { class: "muted small", text: t("profile.last", { when: dateTime(cur.lastAt) }) }) : null,
      h("div", null,
        h("h2", { class: "section-title", text: t("profile.medals") }),
        h("p", { class: "muted small", text: t("profile.medals_count", { n: (cur.medals ?? []).filter((m) => m.earned).length, total: (cur.medals ?? []).length }) }),
        h("div", { class: "medals" }, (cur.medals ?? []).map((m) => h("div", { class: "medal " + (m.earned ? "on" : "off") + (m.fresh ? " fresh" : "") },
          h("span", { class: "medal-icon" }, icon(MEDAL_ICON[m.id] ?? "star", "icon")),
          h("span", { class: "medal-name", text: t(`medal.${m.id}.name`) }),
          h("span", { class: "medal-text", text: t(`medal.${m.id}.text`) }))))),
    ].filter((x): x is HTMLElement => !!x);
  };
  void earned;
  mount(body, ...await profileBody());
  void ProfileService.Seen().catch(() => undefined);
  return h("section", { class: "screen profile" }, ...head, body, h("div", { class: "actions" }, reset), h("p", { class: "muted small", text: t("profile.privacy") }));
}
