-- images tracks every uploaded file. filename is server-generated (never the
-- uploader's name) and unique; the bytes live under <data-dir>/uploads.
CREATE TABLE images (
    id INTEGER PRIMARY KEY,
    filename TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

-- events covers both regular "What's on" events (kind = 'event') and the
-- "Muddy's on the road" block (kind = 'road'). Dates are optional: an event
-- with no ends_at never expires on its own (e.g. a podcast feature) and is
-- hidden only by unpublishing it. Times are RFC3339 UTC.
CREATE TABLE events (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL DEFAULT 'event' CHECK (kind IN ('event', 'road')),
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    badge_text TEXT NOT NULL DEFAULT '', -- optional label; empty means show the date range
    badge_style TEXT NOT NULL DEFAULT 'blue' CHECK (badge_style IN ('orange', 'purple', 'blue', 'pink')),
    starts_at TEXT,
    ends_at TEXT,
    poster_image_id INTEGER REFERENCES images (id) ON DELETE SET NULL,
    link_url TEXT NOT NULL DEFAULT '',
    link_label TEXT NOT NULL DEFAULT '',
    link2_url TEXT NOT NULL DEFAULT '',
    link2_label TEXT NOT NULL DEFAULT '',
    featured INTEGER NOT NULL DEFAULT 0,
    published INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX events_public_idx ON events (published, kind, ends_at);

-- staff_members holds both DJs and hosts. bio and schedule are plain text:
-- bio paragraphs are separated by a blank line, schedule lines by a newline.
-- slug is the stable deep-link id (e.g. #dj-andy) and unique per kind.
CREATE TABLE staff_members (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('dj', 'host')),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    sl_name TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    schedule TEXT NOT NULL DEFAULT '',
    photo_image_id INTEGER REFERENCES images (id) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (kind, slug)
);
CREATE INDEX staff_members_list_idx ON staff_members (kind, active, sort_order);

CREATE TABLE partners (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- settings is a small key/value table for site-wide text and links. The set of
-- valid keys is enforced in Go (see SettingsStore), not here.
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
