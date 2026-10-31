(function () {
  // Phones scroll the navigation row sideways: keep the current page in view.
  // Only the row scrolls; scrollIntoView would also move the page. Run again once
  // the web font has loaded, since it widens the row.
  const cur = document.querySelector('.ky-nav [aria-current="page"]');
  if (cur) {
    const reveal = () => {
      const row = cur.parentElement.getBoundingClientRect(), item = cur.getBoundingClientRect();
      if (item.right > row.right) cur.parentElement.scrollLeft += item.right - row.right;
    };
    reveal();
    document.fonts.ready.then(reveal);
  }

  // commandfor/command open and close dialogs without script where the browser
  // supports invoker commands; this covers the ones that do not yet.
  if (!("command" in HTMLButtonElement.prototype)) {
    document.addEventListener("click", (event) => {
      const button = event.target.closest("button[commandfor]");
      const dialog = button && document.getElementById(button.getAttribute("commandfor"));
      if (!dialog || button.disabled) return;
      if (button.getAttribute("command") === "show-modal") dialog.showModal();
      else if (button.getAttribute("command") === "close") dialog.close();
    });
  }

  // A form that came back with an error reopens with what was typed.
  for (const dialog of document.querySelectorAll("dialog[data-open]")) dialog.showModal();

  // Row menus: one open at a time; Escape, a click elsewhere or a scroll closes them.
  // The list is position: fixed beside its trigger, so a card that scrolls a wide
  // table sideways cannot clip it; it opens upward near the bottom of the window.
  const menus = () => document.querySelectorAll("details.row-menu[open]");
  document.addEventListener("toggle", (event) => {
    const menu = event.target;
    if (!(menu instanceof HTMLDetailsElement) || !menu.matches(".row-menu") || !menu.open) return;
    const at = menu.querySelector("summary").getBoundingClientRect();
    const list = menu.querySelector(".row-menu-list");
    const below = at.bottom + 4 + list.offsetHeight <= innerHeight - 8;
    list.style.top = (below ? at.bottom + 4 : at.top - 4 - list.offsetHeight) + "px";
    list.style.left = Math.max(8, at.right - list.offsetWidth) + "px";
  }, true);
  addEventListener("scroll", () => { for (const menu of menus()) menu.open = false; }, { capture: true, passive: true });
  addEventListener("resize", () => { for (const menu of menus()) menu.open = false; });
  document.addEventListener("click", (event) => {
    for (const menu of menus()) if (!menu.contains(event.target)) menu.open = false;
  });
  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    for (const menu of menus()) { menu.open = false; menu.querySelector("summary").focus(); }
  });

  // Table filters hide rows whose text does not contain the query.
  for (const input of document.querySelectorAll("input[data-filter]")) {
    const table = document.getElementById(input.dataset.filter);
    const rows = [...table.querySelectorAll("tr:not(.column-headings):not(.filter-empty)")].map((row) => ({
      row, text: [...row.cells].filter((cell) => !cell.matches(".actions")).map((cell) => cell.textContent).join(" ").toLowerCase(),
    }));
    const none = table.querySelector(".filter-empty");
    input.addEventListener("input", () => {
      const query = input.value.trim().toLowerCase();
      let shown = 0;
      for (const { row, text } of rows) {
        row.hidden = !text.includes(query);
        if (!row.hidden) shown++;
      }
      if (none) none.hidden = shown > 0;
    });
  }
})();
