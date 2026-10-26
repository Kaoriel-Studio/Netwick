import { h, icon } from "../dom";
import { t } from "../i18n";

const TIPS = 12;

export function dailyTip(): HTMLElement {
  const d = new Date();
  const day = Math.floor(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / 86400000);
  const n = (day % TIPS) + 1;
  return h("div", { class: "tip-card" }, icon("bulb"), h("div", null, h("p", { class: "tip-label", text: t("tip.label") }), h("p", { text: t("tip." + n) })));
}
