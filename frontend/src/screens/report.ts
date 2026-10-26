import { h, icon } from "../dom";
import { t, dateTime } from "../i18n";
import { link, toast, backButton } from "../app";
import { ReportService } from "../api";
import type { Report } from "../api";
import { resultParams } from "./check";

const MARK: Record<string, string> = { ok: "[OK]  ", info: "[i]   ", warn: "[!]   ", fail: "[X]   ", unknown: "[?]   " };

export function reportText(r: Report): string {
  const lines: string[] = [];
  lines.push(t("report.header", { app: r.app, version: r.version, date: dateTime(r.generatedAt) }));
  lines.push(t("report.mode", { mode: r.mode }));
  lines.push("");
  lines.push(t("report.verdict", { verdict: t("verdict.title." + String(r.verdict.level)) }));
  for (const p of r.verdict.problems ?? []) lines.push("  - " + t(p.messageKey, resultParams(p)));
  lines.push("");
  lines.push(t("report.checks"));
  for (const c of r.results ?? []) {
    lines.push(`${MARK[String(c.status)] ?? "      "}${c.checkId} ${t("check." + c.checkId)}: ${t(c.messageKey, resultParams(c))}`);
    for (const d of c.details ?? []) lines.push("        · " + t(d.key, resultParams(d)));
  }
  lines.push("");
  lines.push(t("report.fixes"));
  const fixes = r.fixes ?? [];
  if (fixes.length === 0) lines.push("  " + t("report.no_fixes"));
  for (const f of fixes) lines.push(`  ${dateTime(f.at)}  ${f.fixId} ${t("fix." + f.fixId + ".title")} — ${f.ok ? t("report.fix_ok") : t("report.fix_failed")}${f.undone ? " (" + t("report.undone") + ")" : ""}`);
  lines.push("");
  lines.push(t("report.privacy"));
  return lines.join("\n");
}

export async function reportScreen(): Promise<HTMLElement> {
  let rep: Report | null = null;
  try {
    rep = await ReportService.Build();
  } catch {
    rep = null;
  }
  const text = rep ? reportText(rep) : t("report.empty");
  const pre = h("pre", { class: "report-preview", tabindex: 0, aria: { label: t("report.preview") }, text });
  return h("section", { class: "screen report" },
    backButton(),
    h("h1", { class: "screen-title", text: t("report.title") }),
    h("p", { class: "muted", text: t("report.subtitle") }),
    pre,
    h("div", { class: "actions row wrap" },
      h("button", { class: "btn btn-primary", type: "button", on: { click: async () => {
        const ok = await ReportService.Copy(text).catch(() => false);
        toast(ok ? t("report.copied") : t("report.copy_failed"));
      } } }, icon("copy"), t("report.copy")),
      h("button", { class: "btn btn-secondary", type: "button", on: { click: async () => {
        const name = await ReportService.Save(text, "txt").catch(() => "");
        if (name) toast(t("report.saved", { name }));
      } } }, t("report.save")),
      h("button", { class: "btn btn-ghost", type: "button", on: { click: async () => {
        const name = await ReportService.Save("", "json").catch(() => "");
        if (name) toast(t("report.saved", { name }));
      } } }, t("report.save_json"))),
    h("div", { class: "card soft" },
      h("p", { text: t("report.rrt_hint") }),
      link(t("report.rrt_link"), "https://support.riotgames.com/en-us/riot/performance/gathering-logs-riot-repair-tool-rrt")));
}
