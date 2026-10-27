import { render } from "preact";
import type { WidgetBootstrap } from "@chatfree/shared/widget";
import styles from "./styles.css?inline";
import { Widget } from "./widget";

export function mountWidget(bootstrap: WidgetBootstrap) {
  if (document.querySelector(`[data-chatfree="${bootstrap.key}"]`)) {
    return;
  }

  const host = document.createElement("div");
  host.dataset.chatfree = bootstrap.key;
  host.style.setProperty("--cf-accent", bootstrap.accent ?? "#4f46e5");

  const shadow = host.attachShadow({ mode: "open" });
  const styleElement = document.createElement("style");
  styleElement.textContent = styles;

  const root = document.createElement("div");
  root.className = "cf-root";

  shadow.append(styleElement, root);

  if (bootstrap.target) {
    const target = document.querySelector(bootstrap.target);
    host.style.display = "block";
    host.style.width = "100%";
    (target ?? document.body).appendChild(host);
  } else if (bootstrap.mode === "inline") {
    host.style.display = "block";
    host.style.width = "100%";
    document.body.appendChild(host);
  } else if (bootstrap.mode === "fullscreen") {
    host.style.position = "fixed";
    host.style.inset = "0";
    host.style.zIndex = "2147483000";
    document.body.appendChild(host);
  } else {
    document.body.appendChild(host);
  }

  render(<Widget bootstrap={bootstrap} />, root);
}
