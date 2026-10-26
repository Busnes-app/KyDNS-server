// The suite theme catalog from ky-ui, applied by a classic <head> script before first
// paint (after ky-ui/themes-browser.js). Saved values are catalog names ("Ocean"); older
// saved names resolve unchanged. With nothing saved, tokens.css follows the OS.
(() => {
"use strict";
const { SYSTEM_THEME, THEME_CATALOG, applyThemeVariables, findTheme } = window.kyThemes;
const KEY = "kydns-theme";
const media = window.matchMedia("(prefers-color-scheme: dark)");

function stored() {
  try { return findTheme(localStorage.getItem(KEY)) ?? SYSTEM_THEME; } catch { return SYSTEM_THEME; }
}

function apply(theme) {
  const root = document.documentElement;
  const resolved = theme.id === "system" ? (media.matches ? "busnes-dark" : "busnes-light") : theme.id;
  // Named palettes set their colours inline; Busnes clears them so the shared tokens apply.
  const scheme = applyThemeVariables(resolved, root);
  if (scheme) delete root.dataset.theme;
  else root.dataset.theme = resolved;
  root.style.colorScheme = scheme ?? (resolved === "busnes-dark" ? "dark" : "light");
}

function sync() {
  const theme = stored();
  apply(theme);
  for (const radio of document.querySelectorAll("[data-theme-picker] input")) radio.checked = radio.value === theme.id;
}

function choose(theme) {
  try {
    if (theme.id === "system") localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, theme.name);
  } catch { /* storage off: applies for this page */ }
  apply(theme);
}

// The Appearance page: one native radio card per theme, built with DOM calls so no
// name is parsed as markup. Preview colours come from ky-ui/appearance-themes.css.
function el(tag, className, text) {
  const node = document.createElement(tag);
  node.className = className;
  if (text) node.textContent = text;
  return node;
}

function picker(grid) {
  for (const theme of [SYSTEM_THEME, ...THEME_CATALOG]) {
    const option = el("label", "ky-appearance-option");
    const radio = el("input", "ky-appearance-radio");
    radio.type = "radio";
    radio.name = "kydns-theme";
    radio.value = theme.id;
    radio.addEventListener("change", () => choose(theme));
    const preview = el("span", "ky-appearance-preview");
    preview.setAttribute("aria-hidden", "true");
    preview.dataset.kyPreview = theme.id;
    const mode = theme.scheme === "system" ? "Follows device" : theme.scheme === "light" ? "Light" : "Dark";
    option.append(radio, preview, el("span", "ky-appearance-name", theme.name), el("small", "ky-appearance-mode", mode));
    grid.append(option);
  }
}

apply(stored());
media.addEventListener("change", () => { if (stored().id === "system") apply(stored()); });
// Another tab, or the license iframe's parent, changed it.
window.addEventListener("storage", event => { if (event.key === KEY || event.key === null) sync(); });
document.addEventListener("DOMContentLoaded", () => {
  const grid = document.querySelector("[data-theme-picker] .ky-appearance-grid");
  if (grid) picker(grid);
  sync();
});
})();
