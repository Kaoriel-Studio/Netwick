import { h } from "../dom";
import { t, setLang } from "../i18n";
import { state, link, toast, backButton, applyTheme, rerender, openURL, screenHead } from "../app";
import { AppService, ProfileService } from "../api";
import type { Settings } from "../api";

function toggle(id: string, label: string, hint: string, checked: boolean, onChange: (v: boolean) => void): HTMLElement {
  const input = h("input", { id, type: "checkbox", class: "switch-input", aria: { describedby: id + "-hint" } });
  input.checked = checked;
  input.addEventListener("change", () => onChange(input.checked));
  return h("div", { class: "setting" },
    h("div", { class: "setting-text" }, h("label", { for: id, class: "label", text: label }), h("p", { class: "muted small", id: id + "-hint", text: hint })),
    input);
}

function select(id: string, label: string, value: string, options: [string, string][], onChange: (v: string) => void): HTMLElement {
  const sel = h("select", { class: "input", id });
  for (const [v, l] of options) sel.appendChild(h("option", { value: v, text: l }));
  sel.value = value;
  sel.addEventListener("change", () => onChange(sel.value));
  return h("div", { class: "setting" }, h("label", { for: id, class: "label", text: label }), sel);
}

async function save(patch: Partial<Settings>): Promise<void> {
  if (!state.settings) return;
  const next = { ...state.settings, ...patch } as Settings;
  try {
    state.settings = await AppService.SaveSettings(next);
  } catch {
    toast(t("settings.save_failed"));
  }
}

export function settingsScreen(): HTMLElement {
  const s = state.settings!;
  const info = state.info;
  const updateOut = h("p", { class: "muted small", aria: { live: "polite" } });
  return h("section", { class: "screen settings" },
    backButton(),
    ...screenHead(t("settings.kicker"), t("settings.title")),
    h("div", { class: "card" },
      select("set-lang", t("settings.language"), s.language, [["", t("settings.lang.auto")], ["ru", "Русский"], ["en", "English"]], async (v) => {
        await save({ language: v });
        setLang(v);
        await rerender();
      }),
      select("set-theme", t("settings.theme"), s.theme, [["system", t("settings.theme.system")], ["light", t("settings.theme.light")], ["dark", t("settings.theme.dark")]], async (v) => {
        await save({ theme: v });
        applyTheme(v);
      }),
      select("set-region", t("settings.region"), s.region || "eu", ["eu", "na", "ap", "kr", "br", "latam"].map((r) => [r, t("region." + r)] as [string, string]), (v) => void save({ region: v })),
      toggle("set-upd", t("settings.update"), t("settings.update_hint"), s.updateCheck, (v) => void save({ updateCheck: v })),
      toggle("set-tray", t("settings.tray"), t("settings.tray_hint"), s.tray, (v) => void save({ tray: v })),
      toggle("set-watch", t("settings.watch"), t("settings.watch_hint"), s.watchClient, (v) => void save({ watchClient: v })),
      h("div", { class: "setting" },
        h("div", { class: "setting-text" }, h("p", { class: "label", text: t("settings.clear_history") }), h("p", { class: "muted small", text: t("settings.clear_history_hint") })),
        h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: async () => {
          await ProfileService.Reset().catch(() => undefined);
          toast(t("profile.reset_done"));
        } } }, t("settings.clear_history"))),
      h("div", { class: "row" },
        h("button", { class: "btn btn-secondary btn-small", type: "button", on: { click: async () => {
          const u = await AppService.CheckUpdate();
          if (u.errorKey) updateOut.textContent = t(u.errorKey);
          else if (u.newer) {
            updateOut.textContent = t("update.newer", { version: u.latest });
            updateOut.appendChild(document.createTextNode(" "));
            updateOut.appendChild(h("button", { class: "link", type: "button", text: t("update.open"), on: { click: () => openURL(u.url) } }));
          } else updateOut.textContent = t("update.latest");
        } } }, t("settings.check_update")), updateOut)),
    aboutBlock(info?.version ?? "", info?.repository ?? ""));
}

export function aboutBlock(version: string, repo: string): HTMLElement {
  const does = [1, 2, 3, 4, 5].map((i) => h("li", { text: t("about.does." + i) }));
  const doesnt = [1, 2, 3, 4, 5, 6].map((i) => h("li", { text: t("about.doesnt." + i) }));
  return h("div", { class: "card about", id: "about" },
    h("h2", { class: "section-title", text: t("about.title") }),
    h("p", null, h("span", { class: "strong", text: "Netwick " + version }), " · ", t("about.license")),
    h("p", { class: "notice notice-info strong", text: t("disclaimer.ru_note") }),
    h("div", { class: "cols" },
      h("div", null, h("h3", { class: "sub-title", text: t("about.does_title") }), h("ul", { class: "plain-list ok-list" }, does)),
      h("div", null, h("h3", { class: "sub-title", text: t("about.doesnt_title") }), h("ul", { class: "plain-list no-list" }, doesnt))),
    h("p", { class: "muted small", text: t("about.privacy") }),
    h("blockquote", { class: "legal", text: "Netwick isn't endorsed by Riot Games and doesn't reflect the views or opinions of Riot Games or anyone officially involved in producing or managing Riot Games properties. Riot Games, and all associated properties are trademarks or registered trademarks of Riot Games, Inc." }),
    repo ? link(t("about.source"), repo) : null);
}
