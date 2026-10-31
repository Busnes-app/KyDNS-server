(function () {
  // Phones scroll the navigation row sideways: keep the current page in view.
  // Only the row scrolls; scrollIntoView would also move the page. Run again once
  // the web font has loaded, since it widens the row.
  const cur = document.querySelector('.app-nav-links [aria-current="page"]');
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

  // Row menus: one open at a time; Escape or a click elsewhere closes them.
  const menus = () => document.querySelectorAll("details.row-menu[open]");
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
