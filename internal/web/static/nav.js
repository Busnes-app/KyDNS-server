// Phones scroll the navigation row sideways: keep the current page in view.
// Only the row scrolls; scrollIntoView would also move the page. Run again once
// the web font has loaded, since it widens the row.
(function () {
  const cur = document.querySelector('.app-nav-links [aria-current="page"]');
  if (!cur) return;
  const reveal = () => {
    const row = cur.parentElement.getBoundingClientRect(), item = cur.getBoundingClientRect();
    if (item.right > row.right) cur.parentElement.scrollLeft += item.right - row.right;
  };
  reveal();
  document.fonts.ready.then(reveal);
})();
