// Phones scroll the navigation row sideways: keep the current page in view.
// Only the row scrolls; scrollIntoView would also move the page.
(function () {
  const cur = document.querySelector('.nav-links [aria-current="page"]');
  if (!cur) return;
  const row = cur.parentElement.getBoundingClientRect(), item = cur.getBoundingClientRect();
  if (item.right > row.right) cur.parentElement.scrollLeft += item.right - row.right;
})();
