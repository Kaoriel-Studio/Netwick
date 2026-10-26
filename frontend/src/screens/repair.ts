import { h, icon, mount, statusBadge } from "../dom";
import { t } from "../i18n";
import { state, go, link, backButton, screenHead, runUndo, toast } from "../app";
import { AppService, CheckService } from "../api";
import type { Settings } from "../api";
import { SCENARIOS, type RepairScenario } from "./scenarios";
import { wizardScreen } from "./wizard";
import { startCheck, launchGame, resultText } from "./check";
import { nodesScreen } from "./nodes";
import { supportScreen } from "./support";

const REGIONS = ["eu", "na", "ap", "kr", "br", "latam"];
const STATUS_PAGE = "https://status.riotgames.com/";

export function runScenario(sc: RepairScenario): void {
  switch (sc.action) {
    case "wizard":
      void go(() => wizardScreen(sc.steps ?? [], "", t(`scenario.${sc.id}.title`)));
      return;
    case "check":
      void startCheck("full", "");
      return;
    case "nodes":
      void go(nodesScreen);
      return;
    case "launch":
      void launchGame();
      return;
    case "undo":
      runUndo();
      return;
    case "support":
      void go(supportScreen);
  }
}

export function tile(sc: RepairScenario, index: number, compact = false): HTMLElement {
  return h("button", { class: "tile" + (sc.featured ? " featured" : ""), type: "button", on: { click: () => runScenario(sc) } },
    h("span", { class: "tile-head" }, icon(sc.icon), h("span", { class: "tile-num", text: String(index + 1).padStart(2, "0") })),
    h("span", { class: "tile-title", text: t(`scenario.${sc.id}.title`) }),
    h("span", { class: "tile-text", text: t(`scenario.${sc.id}.text`) }),
    compact || sc.tags.length === 0 ? null : h("span", { class: "tile-tags" }, sc.tags.map((tg) => h("span", { class: "tag tag-" + tagClass(tg), text: t("tag." + tg) }))));
}

function tagClass(tg: string): string {
  switch (tg) {
    case "safe": return "safe";
    case "admin": return "admin";
    case "reboot": return "reboot";
    case "undo": return "undo";
    default: return "v1";
  }
}

function statusStrip(): HTMLElement {
  const out = h("div", { class: "row grow", aria: { live: "polite" } }, h("span", { class: "spinner" }), h("span", { class: "muted", text: t("repair.status_loading") }));
  const load = async () => {
    mount(out, h("span", { class: "spinner" }), h("span", { class: "muted", text: t("repair.status_loading") }));
    try {
      const sum = await CheckService.RunOnly(["C22"]);
      const r = sum?.results?.find((x) => x.checkId === "C22");
      if (!r) throw new Error("no result");
      mount(out, statusBadge(String(r.status), t("status." + String(r.status))), h("span", { text: resultText(r) }));
    } catch {
      mount(out, h("span", { class: "muted", text: t("repair.status_failed") }));
    }
  };
  const sel = h("select", { class: "input", id: "region", aria: { label: t("settings.region") } });
  for (const r of REGIONS) sel.appendChild(h("option", { value: r, text: t("region." + r) }));
  sel.value = state.settings?.region || "eu";
  sel.addEventListener("change", async () => {
    if (!state.settings) return;
    try {
      state.settings = await AppService.SaveSettings({ ...state.settings, region: sel.value } as Settings);
      await load();
    } catch {
      toast(t("settings.save_failed"));
    }
  });
  void load();
  return h("div", { class: "status-strip" },
    icon("globe"),
    h("label", { class: "label small", for: "region", text: t("settings.region") }),
    sel, out, link(t("repair.status_page"), STATUS_PAGE, "link link-small"));
}

export function repairScreen(): HTMLElement {
  const groups: [string, RepairScenario[]][] = [
    ["repair", SCENARIOS.filter((s) => s.group === "repair")],
    ["safe", SCENARIOS.filter((s) => s.group === "safe")],
    ["other", SCENARIOS.filter((s) => s.group === "other")],
  ];
  let n = 0;
  return h("section", { class: "screen repair" },
    backButton(),
    ...screenHead(t("repair.kicker"), t("repair.title"), t("repair.subtitle")),
    statusStrip(),
    groups.map(([g, list]) => h("div", null,
      h("p", { class: "tile-group-title", text: t("repair.group." + g) }),
      h("div", { class: "tiles" }, list.map((sc) => tile(sc, n++))))),
    h("p", { class: "notice small", text: t("repair.note") }));
}
