// ===== Radio player =====
const STREAM_URL = "http://muddys.digistream.info:20398/stream";
const audio = new Audio();
audio.preload = "none";
audio.volume = 0.8;

const playButtons = document.querySelectorAll(".js-play");
const playerCard = document.querySelector(".player");
const statusEl = document.querySelector(".js-status");
const labels = document.querySelectorAll(".js-play-label");
let playing = false;

function setUI(state, message) {
  playing = state;
  [...playButtons, playerCard].forEach(el => el && el.classList.toggle("is-playing", state));
  labels.forEach(l => (l.textContent = state ? "Pause" : "Listen Live"));
  playButtons.forEach(b => b.setAttribute("aria-label", state ? "Pause Muddy's Radio" : "Play Muddy's Radio"));
  if (statusEl) statusEl.textContent = message;
}

function play() {
  // Reload the live stream each time so it starts at "now", not where it paused.
  audio.src = STREAM_URL + "?t=" + Date.now();
  setUI(true, "Connecting…");
  audio.play()
    .then(() => setUI(true, "Now streaming live from Muddy's"))
    .catch(() => setUI(false, "Couldn't start the stream — try the link below."));
}

function stop() {
  audio.pause();
  audio.removeAttribute("src");
  audio.load(); // drop the connection so it stops downloading
  setUI(false, "Press play to tune in");
}

playButtons.forEach(b => b.addEventListener("click", () => (playing ? stop() : play())));
audio.addEventListener("error", () => { if (playing) setUI(false, "Stream unavailable right now — try again shortly."); });

const vol = document.querySelector(".js-volume");
if (vol) vol.addEventListener("input", () => (audio.volume = +vol.value));

// ===== Mobile nav =====
const nav = document.querySelector(".nav");
const toggle = document.getElementById("navToggle");
toggle.addEventListener("click", () => {
  const open = nav.classList.toggle("is-open");
  toggle.setAttribute("aria-expanded", open);
});
document.querySelectorAll(".nav__links a").forEach(a =>
  a.addEventListener("click", () => { nav.classList.remove("is-open"); toggle.setAttribute("aria-expanded", false); })
);

// ===== Lightbox =====
const box = document.getElementById("lightbox");
if (box) {
  const boxImg = box.querySelector("img");
  document.querySelectorAll(".js-lightbox").forEach(btn =>
    btn.addEventListener("click", () => {
      boxImg.src = btn.dataset.src;
      boxImg.alt = btn.dataset.alt || "";
      box.showModal();
    })
  );
  box.querySelector(".lightbox__close").addEventListener("click", () => box.close());
  box.addEventListener("click", e => { if (e.target === box) box.close(); });
}

// ===== Partners =====
const partners = [
  "Gacha Shack", "Snuggles", "Graffitiwear", "Paragon", "Legends of Ludo", "Social VR", "Wyld Deal",
  "Sycamore Decor", "Max Magic", "Canarun", "Royal Estates", "Blue Moon", "Next Door Designs",
  "Carol's Store", "Thingamabobs", "Vista Animations", "Younique", "Good Vibes", "Simple Terms",
  "Telsiope's Couture", "Blender School", "Tea Garden", "Freebird", "Cloud Nine", "Digistream",
  "Cuddles", "Sew n Sew", "Three Graces", "EchoVets", "KK's Designs", "Active Urban", "Hyperdrive",
  "Prima", "Shelbyfied", "Athenia", "AMG", "StreamLife Art Gallery", "Rise", "Glamo Nails",
  "Premier Wrestling", "Crossroads", "Cosmopolitan", "Raggs", "Sweet Surrender", "KC Couture",
  "Cornwell Designs", "6th Element", "Buggy Photography", "GamePrizes", "Violet Skies", "Olympus",
  "Mula Rouge", "Firelight", "Provoka", "Lutra", "Gecko", "Madcat Creations", "Mustard Seed",
  "Travesty", "Mystic Market", "Yalla", "Starzz", "Dabble", "Oahu", "Moon Kiss Bay",
  "Teleport Streams", "Imperium Estates", "Central Park", "Timber Wilds", "Delicatt Couture",
  "Studz", "Wild Side Mall", "Enlight", "Doux Hair", "Sugar and Spice", "Kira Style",
  "Wicked Designs", "PlantPets", "Connectables", "Veda's Closet", "Lil Birdy", "Sibz Estates",
  "Excalibur Rentals", "No Stone Unturned"
];
const wrap = document.querySelector(".js-partners");
if (wrap) wrap.innerHTML = partners.map(p => `<span>${p}</span>`).join("");

// ===== Reveal on scroll =====
const revealEls = document.querySelectorAll(".section__head, .event, .radio > *, .road__inner > *, .story > *, .stats, .muddy, .giving > *, .team > *, .partners__cta");
revealEls.forEach(el => el.classList.add("reveal"));
const io = new IntersectionObserver(entries => {
  entries.forEach(e => { if (e.isIntersecting) { e.target.classList.add("is-in"); io.unobserve(e.target); } });
}, { threshold: 0.12 });
revealEls.forEach(el => io.observe(el));

document.querySelectorAll(".js-year").forEach(el => (el.textContent = new Date().getFullYear()));
