# Muddy's Music Cafe website

New static website for Muddy's Music Cafe in Second Life.

**To view it:** double-click `index.html`. No install needed.

**To edit it with Claude Code:** open a terminal in this folder and run `claude`. It will read `CLAUDE.md` automatically, which has the full project history, design decisions, links, and the to-do list.

**Pages**
- `index.html`: home page
- `djs.html`: the DJs
- `hosts.html`: the hosts

**Common edits**
- Add, remove, or update a DJ or host: edit `staff-data.js` and put a square photo in `images/staff/`.
- Change the radio stream: `STREAM_URL` at the top of `script.js`.
- Change events: the "EVENTS" section of `index.html`, with posters in `images/`.

**To put it online:** upload the whole folder to any static host (GitHub Pages, Netlify, etc.). See the note in `CLAUDE.md` about the radio stream needing an `https` address once it's online.
