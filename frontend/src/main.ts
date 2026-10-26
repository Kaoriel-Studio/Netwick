import { h } from "./dom";
import { t, setLang } from "./i18n";
import { state, buildShell, resetTo, go, applyTheme, setUndoHandler, modal, refreshUndo, toast, backButton, link } from "./app";
import { AppService, FixService, Events, AppNavigate } from "./api";
import { homeScreen, startCheck } from "./screens/check";
import { toolsScreen } from "./screens/tools";
import { settingsScreen, aboutBlock } from "./screens/settings";

function helpScreen(): HTMLElement {
  return h("section", { class: "screen help" },
    backButton(),
    h("h1", { class: "screen-title", text: t("help.title") }),
    h("div", { class: "card" },
      h("ol", { class: "steps-list" }, [1, 2, 3, 4].map((i) => h("li", { text: t("help.step." + i) }))),
      h("p", { class: "muted", text: t("help.safe") }),
      link(t("links.support"), "https://support.riotgames.com/en-us/valorant")),
    aboutBlock(state.info?.version ?? "", state.info?.repository ?? ""));
}

function confirmUndo(): void {
  const items = state.undo;
  modal(t("undo.title"), [
    t("undo.text"),
    h("ul", { class: "plain-list" }, items.map((i) => h("li", { text: t("fix." + i.fixId + ".title") }))),
    t("undo.uac"),
  ], [
    { label: t("undo.confirm"), primary: true, onClick: async () => {
      const out = await FixService.UndoAll().catch(() => null);
      if (!out) toast(t("fix.error.worker"));
      else if (out.declined) toast(t("fix.error.declined"));
      else if (out.ok) toast(t("undo.done", { count: out.undone }));
      else toast(t("undo.partial", { count: out.undone }));
      await refreshUndo();
    } },
    { label: t("common.cancel"), onClick: () => undefined },
  ]);
}

async function boot(): Promise<void> {
  const root = document.getElementById("app")!;
  try {
    state.info = await AppService.Info();
    state.settings = await AppService.Settings();
  } catch {
    state.info = null;
  }
  setLang(state.settings?.language ?? "");
  applyTheme(state.settings?.theme ?? "system");
  document.title = "Netwick";
  buildShell(root, {
    home: () => void resetTo(homeScreen),
    tools: () => void go(() => toolsScreen()),
    settings: () => void go(settingsScreen),
    help: () => void go(helpScreen),
  });
  setUndoHandler(confirmUndo);
  Events.On(AppNavigate, (ev: { data: string }) => {
    if (ev.data === "check") void startCheck("full", "");
    else if (ev.data === "quick") void go(() => toolsScreen("ready"));
    else if (ev.data === "watch-issue") void startCheck("quick", "");
  });
  await resetTo(homeScreen);
  if (state.settings?.updateCheck) {
    AppService.CheckUpdate().then((u) => {
      if (u.newer) toast(t("update.newer", { version: u.latest }));
    }).catch(() => undefined);
  }
}

void boot();
