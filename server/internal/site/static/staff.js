// Behaviour for the DJs and Hosts pages: search, and a profile dialog that
// shows the details the server already rendered inside each card.
(function () {
  const grid = document.querySelector(".js-staff-grid");
  const dlg = document.getElementById("profile");
  if (!grid || !dlg) return;

  const search = document.querySelector(".js-staff-search");
  const empty = document.querySelector(".js-staff-empty");
  const cards = [...grid.querySelectorAll(".staff-card")];
  const photoBox = dlg.querySelector(".profile__photo");
  const textBox = dlg.querySelector(".profile__text");
  let current = null;

  // ===== Search =====
  if (search) {
    search.addEventListener("input", () => {
      const q = search.value.trim().toLowerCase();
      let shown = 0;
      cards.forEach(li => {
        const hit = !q || li.dataset.name.includes(q);
        li.hidden = !hit;
        if (hit) shown++;
      });
      empty.hidden = shown > 0;
    });
  }

  // ===== Profile dialog =====
  const visible = () => cards.filter(li => !li.hidden);

  function open(li, push = true) {
    current = li;
    const img = li.querySelector(".staff-card__img img").cloneNode(true);
    img.alt = li.querySelector(".staff-card__name").textContent;
    img.removeAttribute("loading");
    photoBox.replaceChildren(img);
    const data = li.querySelector(".profile-data");
    textBox.replaceChildren(...[...data.children].map(n => n.cloneNode(true)));
    textBox.scrollTop = 0;
    if (!dlg.open) dlg.showModal();
    if (push) history.replaceState(null, "", "#" + li.id);
  }

  dlg.addEventListener("close", () => {
    history.replaceState(null, "", location.pathname + location.search);
    const btn = current && current.querySelector("button");
    if (btn) btn.focus();
  });

  function step(dir) {
    const v = visible();
    if (!v.length) return;
    const at = v.indexOf(current);
    open(v[(at + dir + v.length) % v.length]);
  }

  grid.addEventListener("click", e => {
    const li = e.target.closest(".staff-card");
    if (li) open(li);
  });
  dlg.querySelector(".profile__close").addEventListener("click", () => dlg.close());
  dlg.querySelector(".profile__prev").addEventListener("click", () => step(-1));
  dlg.querySelector(".profile__next").addEventListener("click", () => step(1));
  dlg.addEventListener("click", e => { if (e.target === dlg) dlg.close(); });
  dlg.addEventListener("keydown", e => {
    if (e.key === "ArrowLeft") step(-1);
    if (e.key === "ArrowRight") step(1);
  });

  // Open a profile straight from a shared link, e.g. /djs#dj-andy
  if (location.hash.length > 1) {
    const li = cards.find(c => "#" + c.id === location.hash);
    if (li) open(li, false);
  }
})();
