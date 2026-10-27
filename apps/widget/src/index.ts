import type { DisplayMode, WidgetBootstrap, WidgetPosition, WidgetTheme } from "@chatfree/shared/widget";
import { DISPLAY_MODES, WIDGET_POSITIONS, WIDGET_THEMES } from "@chatfree/shared/widget";
import { mountWidget } from "./mount";

function currentScript(): HTMLScriptElement | null {
  if (document.currentScript instanceof HTMLScriptElement) {
    return document.currentScript;
  }
  return document.querySelector<HTMLScriptElement>("script[data-key]");
}

function pick<T extends string>(value: string | undefined, allowed: readonly T[]): T | undefined {
  return allowed.includes(value as T) ? (value as T) : undefined;
}

function boot() {
  const script = currentScript();

  if (!script) {
    return;
  }

  const key = script.dataset.key;

  if (!key) {
    console.warn("[chatfree] atributo data-key ausente no script do widget");
    return;
  }

  const apiUrl = script.dataset.api ?? new URL(script.src || window.location.href).origin;

  const bootstrap: WidgetBootstrap = {
    key,
    apiUrl,
    mode: pick<DisplayMode>(script.dataset.mode, DISPLAY_MODES),
    position: pick<WidgetPosition>(script.dataset.position, WIDGET_POSITIONS),
    theme: pick<WidgetTheme>(script.dataset.theme, WIDGET_THEMES),
    accent: script.dataset.accent,
    greeting: script.dataset.greeting,
    launcherLabel: script.dataset.launcher,
    target: script.dataset.target,
  };

  mountWidget(bootstrap);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", boot, { once: true });
} else {
  boot();
}
