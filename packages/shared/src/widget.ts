export const DISPLAY_MODES = ["bubble", "inline", "popup", "fullscreen"] as const;
export type DisplayMode = (typeof DISPLAY_MODES)[number];

export const WIDGET_POSITIONS = [
  "bottom-right",
  "bottom-left",
  "top-right",
  "top-left",
] as const;
export type WidgetPosition = (typeof WIDGET_POSITIONS)[number];

export const WIDGET_THEMES = ["light", "dark", "auto"] as const;
export type WidgetTheme = (typeof WIDGET_THEMES)[number];

export interface WidgetAppearance {
  mode: DisplayMode;
  position: WidgetPosition;
  theme: WidgetTheme;
  accent: string;
  greeting: string;
  launcherLabel: string;
  showBranding: boolean;
}

export interface WidgetConfig {
  key: string;
  name: string;
  appearance: WidgetAppearance;
}

export interface WidgetBootstrap extends Partial<WidgetAppearance> {
  key: string;
  apiUrl: string;
  target?: string;
}

export const DEFAULT_APPEARANCE: WidgetAppearance = {
  mode: "bubble",
  position: "bottom-right",
  theme: "light",
  accent: "#4f46e5",
  greeting: "Olá! Como podemos ajudar você hoje?",
  launcherLabel: "Fale com a gente",
  showBranding: true,
};
