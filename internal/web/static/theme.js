// The suite theme catalog from ky-ui, applied by a classic <head> script before first
// paint (after ky-ui/themes-browser.js). The saved value is a pair ID (that pair follows
// the OS) or a half ID (a fixed mode); Match system for Busnes removes the key. Older saved
// catalog names and aliases ("Ocean") resolve unchanged; removed palettes fall back to system.
(() => {
"use strict";
const { SYSTEM_THEME, THEME_PAIRS, applyThemeVariables, findPair, findTheme } = window.kyThemes;
const KEY = "kydns-theme";
const media = window.matchMedia("(prefers-color-scheme: dark)");
// KyCalendar's reference: per pair, Match system / Light / Dark. Busnes's Match system is "system".
const CHOICES = THEME_PAIRS.map(pair => ({
  pair,
  modes: [
    { label: "Match system", value: pair.id === "busnes-pair" ? SYSTEM_THEME.id : pair.id },
    { label: "Light", value: pair.light },
    { label: "Dark", value: pair.dark },
  ],
}));

function stored() {
  try {
    const value = localStorage.getItem(KEY);
    return findPair(value)?.id ?? findTheme(value)?.id ?? SYSTEM_THEME.id;
  } catch { return SYSTEM_THEME.id; }
}

function apply(choice) {
  const root = document.documentElement;
  const resolved = choice === SYSTEM_THEME.id ? (media.matches ? "busnes-dark" : "busnes-light") : findTheme(choice, media.matches).id;
  // Named palettes set their colours inline; Busnes clears them so the shared tokens apply.
  const scheme = applyThemeVariables(resolved, root) ?? (resolved === "busnes-dark" ? "dark" : "light");
  if (resolved.startsWith("busnes-")) root.dataset.theme = resolved;
  else delete root.dataset.theme;
  root.style.colorScheme = scheme;
  root.dataset.scheme = scheme;
}

function sync() {
  const choice = stored();
  apply(choice);
  for (const radio of document.querySelectorAll("[data-theme-picker] input")) radio.checked = radio.value === choice;
  for (const select of document.querySelectorAll("[data-theme-shortcut]")) select.value = choice;
}

function choose(value) {
  try {
    if (value === SYSTEM_THEME.id) localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, value);
  } catch { /* storage off: applies for this page */ }
  apply(value);
}

// DOM calls only, so no name is parsed as markup.
function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text) node.textContent = text;
  return node;
}

const CHECK = "M20 6 9 17l-5-5";

// The Appearance page: one card per pair, one native radio group across them.
function picker(grid) {
  for (const { pair, modes } of CHOICES) {
    const card = el("div", "appearance-pair");
    card.setAttribute("role", "group");
    card.setAttribute("aria-labelledby", pair.id + "-name");
    const head = el("div", "appearance-pair-head");
    const name = el("h2", "appearance-pair-name", pair.name);
    name.id = pair.id + "-name";
    head.append(name, el("span", "appearance-pair-halves", findTheme(pair.light).name + " · " + findTheme(pair.dark).name));
    const preview = el("span", "ky-appearance-preview");
    preview.setAttribute("aria-hidden", "true");
    preview.dataset.kyPreview = pair.id;
    const row = el("div", "appearance-modes");
    for (const mode of modes) {
      const option = el("label", "appearance-mode");
      const radio = el("input");
      radio.type = "radio";
      radio.name = KEY;
      radio.value = mode.value;
      radio.addEventListener("change", () => choose(mode.value));
      const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
      svg.setAttribute("viewBox", "0 0 24 24");
      svg.setAttribute("aria-hidden", "true");
      svg.setAttribute("class", "appearance-check");
      const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
      path.setAttribute("d", CHECK);
      svg.append(path);
      option.append(radio, svg, el("span", "", mode.label));
      row.append(option);
    }
    card.append(head, preview, row);
    grid.append(card);
  }
}

// The sign-in switcher lists the same choices, grouped by pair.
function shortcut(select) {
  select.replaceChildren(...CHOICES.map(({ pair, modes }) => {
    const group = el("optgroup");
    group.label = pair.name;
    group.append(...modes.map(mode => new Option(pair.name + " · " + mode.label, mode.value)));
    return group;
  }));
  select.addEventListener("change", () => choose(select.value));
}

apply(stored());
media.addEventListener("change", () => apply(stored()));
window.addEventListener("storage", event => { if (event.key === KEY || event.key === null) sync(); });
document.addEventListener("DOMContentLoaded", () => {
  const grid = document.querySelector("[data-theme-picker] .ky-appearance-grid");
  if (grid) picker(grid);
  for (const select of document.querySelectorAll("[data-theme-shortcut]")) shortcut(select);
  sync();
});
})();
