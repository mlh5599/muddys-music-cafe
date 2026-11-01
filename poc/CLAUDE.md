# Muddy's Music Cafe website: project context

Static website for **Muddy's Music Cafe**, a Second Life music club (est. 2010, owner Bridget Hammill, "Beth").
It replaces the old Wix site at https://muddys2010.wixsite.com/muddys, which the owner felt was poorly designed.
Everything here is plain HTML/CSS/JS: no build step, no framework. Open `index.html` in a browser to view it.

## Files

| File | What it is |
|---|---|
| `index.html` | Home page: hero, radio player, events, on the road, story, Muddy the dog, giving back, hiring, partners |
| `djs.html`, `hosts.html` | Staff pages. Same template; `<body data-staff="djs|hosts">` picks the list |
| `staff-data.js` | All DJ/host names, photos, bios (`window.MUDDYS_STAFF = { djs: [...], hosts: [...] }`). Optional fields: `sl` (Second Life name), `schedule` (list of strings, shown with "SLT") |
| `staff.js` | Renders the staff grid, search, and profile dialog. Profiles deep-link via `#dj-andy`, `#host-belona` (slug = photo filename) |
| `script.js` | Shared: radio player, mobile nav, poster lightbox, partner list, scroll reveal. Null-safe so it runs on every page |
| `styles.css` | All styles. Color tokens on `:root` |
| `images/` | Logo, posters (resized JPGs), `images/staff/` = 600px square staff photos |
| `tools/` | PowerShell scripts used to scrape staff from the old Wix site (see below) |

## Design decisions

- Built around the logo: black background, distressed off-white slab serif, script "Est. 2010".
- Fonts (Google Fonts): **Alfa Slab One** for headings (matches the logo), **Yellowtail** for script accents, **Inter** for body text.
- Colors: `--ink #f4eee2` (off-white), `--amber #ffc24b` (script accents, like the gold "Est. 2010"), `--blue #49c6ff` and `--red #ff5a3c` (neon, from the club's "OPEN 24 HOURS" sign).
- The site is dark only, on purpose (nightclub feel). There's a subtle film-grain overlay to echo the distressed logo.
- Headings are uppercase slab with one script-word accent (`<span class="script">`).
- Muddy is the chocolate lab mascot (`images/home-photo.jpg`), featured on the home page.
- Avoid the tagline "Where everyone knows your name" from one of their posters: it's the *Cheers* tagline. Use their own line, "a little club in Second Life with a big heart".

## Key facts and links (from the old site)

- Live DJs 24/7/365. Music: soft rock to pop, 70s to today. "All hits all the time."
- Muddy's Radio started in 2015. Stream: `http://muddys.digistream.info:20398/stream` (tested OK). Status page: `http://muddys.digistream.info:20398/index.html?sid=1`
- Official radio station of the American Cancer Society of SL (Relay For Life, Strides).
- Sponsor of Premier Wrestling's "Full Circle". Roadshow SLurl: https://maps.secondlife.com/secondlife/Premier%20Virtual%20Entertainment/157/153/23
- On the Road / Cloud Nine (Oct 25, DJ John + host Amfie): https://maps.secondlife.com/secondlife/GoodVibes/125/127/21 · SoundCloud: https://soundcloud.com/user-680512645/muddys-radio-cloud-9
- Facebook: https://www.facebook.com/Muddysmusiccafe (the old site had a typo, "wwww.")
- SL group: http://world.secondlife.com/group/6ffdf9ce-7581-3731-e601-81ee5f9b6842
- Flickr: https://www.flickr.com/groups/muddys/
- 4th Wall Podcast (Bridget interview): https://www.youtube.com/watch?v=D2rIlQpxip8
- Hiring: DJs and hosts, must have experience, busy 24/7 club, PG venue, keep all tips, contact Bridget Hammill in SL.
- Partner names in `script.js` were taken from the old site's Business Partners page link slugs. Some capitalization is a best guess.

## Staff data notes

- 22 DJs and 28 hosts were scraped from the old site's `/team-1` (DJs) and `/team` (hosts) pages and their detail pages, in the original order.
- Bios are kept verbatim except: Kelly and JayMarie (all-caps fragments rewritten into sentences), Life Suki (one-line fragments joined into paragraphs), and Belona (schedule pulled into the `schedule` field, one intro line reworded).
- **Mairi** has a photo but no bio (her old Wix detail page is empty), so the page shows "bio coming soon".
- Running `tools/scrape-staff.ps1` and then `tools/build-staff-data.ps1` regenerates `staff-data.js` from Wix and **overwrites** the hand edits above. To add or change one person, edit `staff-data.js` directly instead.

## Open items / next steps

1. **No SLurl for the club itself.** The old site never listed one. "Join our SL Group" stands in for it. Once the owner provides the SLurl, add a "Teleport to Muddy's" button (hero + footer).
2. **The radio stream is `http`.** It plays when the file is opened locally, but browsers block it on an `https` site, and nearly every host serves `https`. Ask Digistream for an `https` stream URL, then update `STREAM_URL` in `script.js` and the link in `index.html`.
3. **The events are the Halloween/October 2026 ones** (Block Party Oct 29–31, Monster Mash-Up Oct 29–Nov 1, Road to Full Circle, 4th Wall Podcast, Cloud Nine Oct 25). They'll need swapping as new events come up. Consider moving events into a data file like the staff list.
4. **Not built yet:** "Muddy's in the Media" page (`/latest-news` on the old site, never scraped), individual business-partner pages (about 100 on the old site), the Cloud Nine page.
5. **Hosting:** not deployed yet. GitHub Pages or Netlify (free) were suggested. All paths are relative, so the folder can be uploaded as is.

## Checking layout

On the original machine, pages were checked with headless Edge screenshots, e.g.
`msedge --headless=new --window-size=1440,4000 --screenshot=out.png file:///.../index.html`.
For a true phone width, load the page in a 390px-wide `<iframe>` inside a wrapper page (headless Edge won't shrink the window that far).
