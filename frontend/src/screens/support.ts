import { h, icon } from "../dom";
import { t } from "../i18n";
import { go, link, backButton, screenHead, toast } from "../app";
import { ReportService } from "../api";
import { reportScreen, reportText } from "./report";
import { searchScreen } from "./search";
import { scenario } from "./scenarios";
import { runScenario } from "./repair";

const STATUS = "https://status.riotgames.com/";
const RRT = "https://support.riotgames.com/en-us/riot/performance/gathering-logs-riot-repair-tool-rrt";
const TICKET = "https://support.riotgames.com/en-us/valorant/submit-a-ticket";

async function copyReport(): Promise<void> {
  try {
    const rep = await ReportService.Build();
    const ok = await ReportService.Copy(reportText(rep));
    toast(ok ? t("report.copied") : t("report.copy_failed"));
  } catch {
    toast(t("report.copy_failed"));
  }
}

function step(title: string, text: string, ...actions: (HTMLElement | null)[]): HTMLElement {
  return h("li", null, h("div", { class: "support-body" },
    h("h2", { class: "sub-title", text: title }),
    h("p", { text }),
    h("div", { class: "actions" }, actions)));
}

export function supportScreen(): HTMLElement {
  const auto = scenario("auto");
  return h("section", { class: "screen support" },
    backButton(),
    ...screenHead(t("support.kicker"), t("support.title"), t("support.subtitle")),
    h("div", { class: "card soft" },
      h("p", { class: "strong", text: t("support.before_title") }),
      h("p", { class: "small muted", text: t("support.before_text") }),
      h("div", { class: "actions" },
        auto ? h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: () => runScenario(auto) } }, icon("bolt"), t("scenario.auto.title")) : null,
        h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: () => void go(() => searchScreen("")) } }, icon("search"), t("result.find_code")))),
    h("ol", { class: "support-steps" },
      step(t("support.s1.title"), t("support.s1.text"), link(t("support.s1.link"), STATUS, "btn btn-secondary btn-small")),
      step(t("support.s2.title"), t("support.s2.text"),
        h("button", { class: "btn btn-primary btn-small", type: "button", on: { click: () => void copyReport() } }, icon("copy"), t("report.copy")),
        h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: () => void go(reportScreen) } }, t("support.s2.open"))),
      step(t("support.s3.title"), t("support.s3.text"), link(t("support.s3.link"), RRT, "btn btn-secondary btn-small")),
      step(t("support.s4.title"), t("support.s4.text"), link(t("support.s4.link"), TICKET, "btn btn-primary btn-small"))),
    h("p", { class: "muted small", text: t("support.privacy") }));
}
