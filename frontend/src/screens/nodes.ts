import { h, icon, mount } from "../dom";
import { t } from "../i18n";
import { backButton, screenHead, refreshUndo, toast } from "../app";
import { FixService } from "../api";
import type { NodesView, RoutePlan, ProbeResult, Outcome } from "../api";

type NodeState = "ok" | "mixed" | "down" | "routed";

function nodeState(p: RoutePlan): NodeState {
  if (p.good === 0) return "down";
  const pinnedOK = (p.probes ?? []).some((x) => x.pinned && String(x.status) === "Ok");
  if (pinnedOK && p.pinned === p.ip) return "routed";
  if (p.good < p.total || (p.pinned && !pinnedOK)) return "mixed";
  return "ok";
}

function probeDot(r: ProbeResult): string {
  switch (String(r.status)) {
    case "Ok": return "ok";
    case "Stalled":
    case "CertError":
    case "Empty": return "warn";
    default: return "fail";
  }
}

function probeRow(r: ProbeResult): HTMLElement {
  const cause = r.cause ? " · " + t("probe.cause." + String(r.cause)) : "";
  return h("tr", null,
    h("td", { class: "ip" }, r.ip, r.pinned ? h("span", { class: "pin-mark", text: t("nodes.pinned") }) : null),
    h("td", null, h("span", { class: "ip-state" }, h("span", { class: "dot dot-" + probeDot(r) }), h("span", { text: t("probe." + String(r.status)) + cause }))),
    h("td", { class: "lat", text: String(r.status) === "Ok" ? r.latencyMs + " ms" : "—" }));
}

function nodeCard(p: RoutePlan, picked: Set<string>, onPick: () => void): HTMLElement {
  const st = nodeState(p);
  const pick = h("input", { type: "checkbox", class: "check-box", id: "pick-" + p.host });
  pick.checked = picked.has(p.host);
  pick.addEventListener("change", () => {
    if (pick.checked) picked.add(p.host);
    else picked.delete(p.host);
    onPick();
  });
  const probes = (p.probes ?? []).slice().sort((a, b) => (probeDot(a) === "ok" ? 0 : 1) - (probeDot(b) === "ok" ? 0 : 1) || a.latencyMs - b.latencyMs);
  return h("div", { class: "node node-" + st },
    h("div", { class: "node-head" },
      h("span", { class: "node-host", text: p.host }),
      p.core ? h("span", { class: "tag tag-v1", text: t("nodes.core") }) : h("span", { class: "tag", text: t("nodes.extra") }),
      h("span", { class: "node-verdict", text: t("nodes.state." + st, { good: p.good, total: p.total }) })),
    h("div", { class: "table-wrap" }, h("table", { class: "ip-table" }, h("tbody", null, probes.map(probeRow)))),
    p.ip && p.pinned !== p.ip
      ? h("label", { class: "node-pick", for: "pick-" + p.host }, pick, h("span", { text: t("nodes.pick", { ip: p.ip, ms: p.latencyMs }) }))
      : null);
}

function outcomeNotice(out: Outcome | null): HTMLElement {
  if (!out) return h("p", { class: "notice notice-fail", text: t("fix.error.generic") });
  if (out.declined) return h("p", { class: "notice notice-warn", text: t("fix.error.declined") });
  if (!out.ok) return h("p", { class: "notice notice-fail", text: t(out.errorKey || "fix.error.generic") });
  return h("p", { class: "notice notice-ok", text: out.changed ? t("wizard.done") : t("wizard.already") });
}

export function nodesScreen(): HTMLElement {
  const list = h("div", { class: "nodes", aria: { live: "polite" } });
  const summary = h("div", { class: "stats" });
  const status = h("div", { class: "step-status", aria: { live: "polite" } });
  const actions = h("div", { class: "actions" });
  const picked = new Set<string>();
  let view: NodesView | null = null;

  const drawActions = () => {
    const pinBtn = h("button", { class: "btn btn-primary", type: "button", disabled: picked.size === 0, on: { click: () => void apply("F14", Array.from(picked)) } }, icon("pin"), t("nodes.apply", { count: picked.size }));
    const clearBtn = (view?.pinned ?? 0) > 0
      ? h("button", { class: "btn btn-secondary", type: "button", on: { click: () => void apply("F10", []) } }, icon("trash"), t("nodes.clear"))
      : null;
    const again = h("button", { class: "btn btn-ghost", type: "button", on: { click: () => void run() } }, icon("reset"), t("nodes.again"));
    mount(actions, (view?.needed ?? 0) > 0 ? pinBtn : null, clearBtn, again);
  };

  const run = async () => {
    mount(list, h("div", { class: "working" }, h("span", { class: "spinner" }), h("span", { text: t("nodes.running") })));
    mount(summary);
    mount(actions);
    try {
      view = await FixService.Nodes();
    } catch {
      view = null;
    }
    if (!view) {
      mount(list, h("p", { class: "notice notice-fail", text: t("nodes.failed") }));
      drawActions();
      return;
    }
    const plans = view.plans ?? [];
    picked.clear();
    for (const p of plans) if (p.ip && p.pinned !== p.ip) picked.add(p.host);
    const ok = plans.filter((p) => nodeState(p) === "ok" || nodeState(p) === "routed").length;
    mount(summary,
      h("div", { class: "stat" }, h("span", { class: "stat-value", text: `${ok}/${plans.length}` }), h("span", { class: "stat-label", text: t("nodes.stat_ok") })),
      h("div", { class: "stat" }, h("span", { class: "stat-value", text: String(view.needed) }), h("span", { class: "stat-label", text: t("nodes.stat_needed") })),
      h("div", { class: "stat" }, h("span", { class: "stat-value", text: String(view.pinned) }), h("span", { class: "stat-label", text: t("nodes.stat_pinned") })));
    const verdict = view.allDown === plans.length && plans.length > 0
      ? h("p", { class: "notice notice-fail", text: t("nodes.verdict.all_down") })
      : view.needed > 0
        ? h("p", { class: "notice notice-warn", text: t("nodes.verdict.needed", { count: view.needed }) })
        : view.allDown > 0
          ? h("p", { class: "notice notice-warn", text: t("nodes.verdict.some_down") })
          : h("p", { class: "notice notice-ok", text: t("nodes.verdict.ok") });
    mount(list, verdict, plans.map((p) => nodeCard(p, picked, drawActions)));
    drawActions();
  };

  const apply = async (id: string, hosts: string[]) => {
    mount(status, h("div", { class: "working" }, h("span", { class: "spinner" }), h("span", { text: t("wizard.waiting_uac") })));
    let out: Outcome | null = null;
    try {
      out = await FixService.Apply(id, { onDemand: "", context: "", hosts: id === "F14" ? hosts : null });
    } catch {
      out = null;
    }
    mount(status, outcomeNotice(out));
    if (out?.ok) {
      toast(out.changed ? t("wizard.done") : t("wizard.already"));
      await refreshUndo();
      await run();
    }
  };

  void run();
  return h("section", { class: "screen nodes-screen" },
    backButton(),
    ...screenHead(t("nodes.kicker"), t("nodes.title"), t("nodes.subtitle")),
    summary,
    actions,
    status,
    list,
    h("div", { class: "card soft" },
      h("h2", { class: "section-title", text: t("nodes.how_title") }),
      h("p", { class: "small", text: t("nodes.how_1") }),
      h("p", { class: "small", text: t("nodes.how_2") }),
      h("p", { class: "small muted", text: t("nodes.how_3") })));
}
