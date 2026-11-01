// Renders the DJs or Hosts page from staff-data.js.
// The page's <body data-staff="djs|hosts"> picks which list to show.
(function () {
  const kind = document.body.dataset.staff;
  const people = (window.MUDDYS_STAFF || {})[kind] || [];
  const role = kind === "djs" ? "DJ" : "Host";

  const grid = document.querySelector(".js-staff-grid");
  const search = document.querySelector(".js-staff-search");
  const empty = document.querySelector(".js-staff-empty");
  const count = document.querySelector(".js-staff-count");
  const dlg = document.getElementById("profile");

  const slug = p => p.photo.split("/").pop().replace(/\.jpg$/, "");
  const esc = s => String(s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  // "DJ Andy" -> { pre: "DJ", name: "Andy" }
  const split = n => (/^DJ\s+/i.test(n) ? { pre: "DJ", name: n.replace(/^DJ\s+/i, "") } : { pre: "", name: n });
  const teaser = p => {
    const t = (p.bio[0] || "").trim();
    if (!t) return "Bio coming soon.";
    return t.length > 120 ? t.slice(0, t.lastIndexOf(" ", 118)) + "…" : t;
  };

  if (count) count.textContent = people.length;

  grid.innerHTML = people.map((p, i) => {
    const n = split(p.name);
    return `
      <li class="staff-card" data-i="${i}" data-name="${esc(p.name.toLowerCase())}">
        <button type="button" class="staff-card__btn" aria-label="Read ${esc(p.name)}'s bio">
          <span class="staff-card__img"><img src="${esc(p.photo)}" alt="" loading="lazy" width="600" height="600"></span>
          <span class="staff-card__body">
            ${n.pre ? `<span class="staff-card__pre">${n.pre}</span>` : ""}
            <span class="staff-card__name">${esc(n.name)}</span>
            <span class="staff-card__teaser">${esc(teaser(p))}</span>
            <span class="staff-card__more">Read bio <b>→</b></span>
          </span>
        </button>
      </li>`;
  }).join("");

  // ===== Search =====
  if (search) {
    search.addEventListener("input", () => {
      const q = search.value.trim().toLowerCase();
      let shown = 0;
      grid.querySelectorAll(".staff-card").forEach(li => {
        const hit = !q || li.dataset.name.includes(q);
        li.hidden = !hit;
        if (hit) shown++;
      });
      empty.hidden = shown > 0;
    });
  }

  // ===== Profile dialog =====
  let current = -1;
  const visible = () => [...grid.querySelectorAll(".staff-card:not([hidden])")].map(li => +li.dataset.i);

  function open(i, push = true) {
    const p = people[i];
    if (!p) return;
    current = i;
    const n = split(p.name);
    const bio = p.bio.length
      ? p.bio.map(t => `<p>${esc(t)}</p>`).join("")
      : `<p class="muted">${esc(n.name)}'s bio is coming soon. Say hi next time you see them at Muddy's!</p>`;
    const sched = p.schedule
      ? `<h3 class="profile__sub">Catch ${esc(n.name)} hosting</h3><ul class="profile__sched">${p.schedule.map(s => `<li>${esc(s)} <small>SLT</small></li>`).join("")}</ul>`
      : "";
    const sl = p.sl ? `<p class="profile__sl"><span>Second Life name</span> ${esc(p.sl)}</p>` : "";

    dlg.querySelector(".profile__photo").innerHTML = `<img src="${esc(p.photo)}" alt="${esc(p.name)}" width="600" height="600">`;
    dlg.querySelector(".profile__text").innerHTML = `
      <p class="eyebrow">${role} at Muddy's</p>
      <h2 class="profile__name">${n.pre ? `<span>${n.pre}</span> ` : ""}${esc(n.name)}</h2>
      <div class="profile__bio">${bio}</div>
      ${sched}${sl}`;
    dlg.querySelector(".profile__text").scrollTop = 0;
    if (!dlg.open) dlg.showModal();
    if (push) history.replaceState(null, "", "#" + slug(p));
  }

  function close() {
    dlg.close();
  }
  dlg.addEventListener("close", () => {
    history.replaceState(null, "", location.pathname + location.search);
    const li = grid.querySelector(`.staff-card[data-i="${current}"] button`);
    if (li) li.focus();
  });

  function step(dir) {
    const v = visible();
    const at = v.indexOf(current);
    open(v[(at + dir + v.length) % v.length]);
  }

  grid.addEventListener("click", e => {
    const li = e.target.closest(".staff-card");
    if (li) open(+li.dataset.i);
  });
  dlg.querySelector(".profile__close").addEventListener("click", close);
  dlg.querySelector(".profile__prev").addEventListener("click", () => step(-1));
  dlg.querySelector(".profile__next").addEventListener("click", () => step(1));
  dlg.addEventListener("click", e => { if (e.target === dlg) close(); });
  dlg.addEventListener("keydown", e => {
    if (e.key === "ArrowLeft") step(-1);
    if (e.key === "ArrowRight") step(1);
  });

  // Open a profile straight from a shared link, e.g. djs.html#dj-andy
  const fromHash = people.findIndex(p => "#" + slug(p) === location.hash);
  if (fromHash >= 0) open(fromHash, false);
})();
