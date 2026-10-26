import type { Step } from "../api";

const KIND: Record<string, string> = { f: "fix", g: "guide", c: "check", i: "info", a: "article" };

export function steps(spec: string): Step[] {
  return spec.split(" ").filter(Boolean).map((x) => {
    const [k, id] = x.split(":");
    return { kind: KIND[k], id };
  });
}

export type ScenarioAction = "wizard" | "check" | "nodes" | "launch" | "undo" | "support";

export interface RepairScenario {
  id: string;
  icon: string;
  group: "safe" | "repair" | "other";
  action: ScenarioAction;
  steps?: Step[];
  tags: string[];
  featured?: boolean;
}

export const SCENARIOS: RepairScenario[] = [
  { id: "auto", icon: "bolt", group: "repair", action: "wizard", featured: true, tags: ["confirm", "admin"], steps: steps("c:C22 c:C03 c:C11 c:C12 c:C19 f:F11 f:F02 f:F13 c:C10 f:F14 f:F01") },
  { id: "diagnose", icon: "radar", group: "safe", action: "check", tags: ["safe"] },
  { id: "nodes", icon: "nodes", group: "safe", action: "nodes", tags: ["safe"] },
  { id: "quick", icon: "broom", group: "repair", action: "wizard", tags: ["noreboot", "admin"], steps: steps("f:F11 f:F02 f:F13 f:F01") },
  { id: "network", icon: "reset", group: "repair", action: "wizard", tags: ["reboot", "admin"], steps: steps("f:F11 f:F12 c:C10 f:F14 f:F13 f:F05") },
  { id: "bonjour", icon: "power", group: "repair", action: "wizard", tags: ["undo", "admin"], steps: steps("c:C23 f:F15") },
  { id: "vanguard", icon: "shield", group: "repair", action: "wizard", tags: ["reboot"], steps: steps("f:F13 c:C09 g:G01") },
  { id: "routes_reset", icon: "trash", group: "repair", action: "wizard", tags: ["undo", "admin"], steps: steps("f:F10") },
  { id: "launch", icon: "play", group: "other", action: "launch", tags: ["safe"] },
  { id: "undo", icon: "undo", group: "other", action: "undo", tags: [] },
  { id: "support", icon: "support", group: "other", action: "support", tags: ["safe"] },
];

export function scenario(id: string): RepairScenario | undefined {
  return SCENARIOS.find((s) => s.id === id);
}
