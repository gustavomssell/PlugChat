import { mountWidget } from "./mount";
import type { DisplayMode, WidgetPosition } from "@chatfree/shared/widget";

const params = new URLSearchParams(window.location.search);
const mode = (params.get("mode") ?? "bubble") as DisplayMode;
const position = (params.get("position") ?? "bottom-right") as WidgetPosition;
const accent = params.get("accent") ?? "4f46e5";

mountWidget({
  key: "pk_demo",
  apiUrl: import.meta.env.VITE_API_URL ?? "http://localhost:8090",
  mode,
  position,
  accent: accent.startsWith("#") ? accent : `#${accent}`,
  target: mode === "inline" ? "#inline-slot" : undefined,
});
