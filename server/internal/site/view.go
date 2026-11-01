package site

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

// common is shown on every page: the head, nav and footer need it.
type common struct {
	Title, Description string
	Active             string // "dj", "host" or ""
	Version            string
	SiteURL            string
	Year               int

	StreamURL, StatusURL, ClubSLurl, SLGroupURL string
	FacebookURL, FlickrURL, YouTubeURL          string
	Hiring                                      []string
}

type eventView struct {
	Title, Summary, Badge, BadgeStyle string
	Featured                          bool
	PosterURL                         string
	PosterW, PosterH                  int
	LinkURL, LinkLabel                string
	Link2URL, Link2Label              string
}

type homePage struct {
	common
	Events             []eventView
	Road               *eventView
	Partners           []string
	DJCount, HostCount int
}

type staffView struct {
	Anchor, SearchName   string
	Name, Pre, ShortName string
	Teaser, PhotoURL     string
	Bio, Schedule        []string
	SLName               string
}

type staffPage struct {
	common
	Kind, Role, RolePlural string
	Staff                  []staffView
}

const defaultTimezone = "America/Los_Angeles"

func (s *Site) loadCommon(ctx context.Context, title, description string) (common, map[string]string, *time.Location, error) {
	set, err := s.settings.All(ctx)
	if err != nil {
		return common{}, nil, nil, err
	}
	loc, err := time.LoadLocation(set[storage.SettingTimezone])
	if err != nil || set[storage.SettingTimezone] == "" {
		loc, _ = time.LoadLocation(defaultTimezone)
		if loc == nil {
			loc = time.UTC
		}
	}
	c := common{
		Title: title, Description: description, Version: s.version, SiteURL: strings.TrimRight(s.cfg.SiteURL, "/"), Year: s.cfg.Now().In(loc).Year(),
		StreamURL: set[storage.SettingRadioStreamURL], StatusURL: set[storage.SettingRadioStatusURL],
		ClubSLurl: set[storage.SettingClubSLurl], SLGroupURL: set[storage.SettingSLGroupURL],
		FacebookURL: set[storage.SettingFacebookURL], FlickrURL: set[storage.SettingFlickrURL],
		YouTubeURL: set[storage.SettingYouTubeURL], Hiring: lines(set[storage.SettingHiringText]),
	}
	return c, set, loc, nil
}

func (s *Site) serveHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, _, loc, err := s.loadCommon(ctx, "Muddy's Music Cafe",
		"Muddy's Music Cafe — Second Life's award-winning music venue since 2010. Live DJs 24/7, 365 days a year, plus Muddy's Radio streaming across the grid.")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	images, err := s.images.List(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	now := s.cfg.Now()
	page := homePage{common: c}

	evs, err := s.events.ListPublic(ctx, storage.EventKindEvent, now)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, e := range evs {
		page.Events = append(page.Events, toEventView(e, images, loc))
	}
	road, err := s.events.ListPublic(ctx, storage.EventKindRoad, now)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if len(road) > 0 {
		v := toEventView(road[0], images, loc)
		page.Road = &v
	}

	partners, err := s.partners.List(ctx, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, p := range partners {
		page.Partners = append(page.Partners, p.Name)
	}
	djs, err := s.staff.List(ctx, storage.StaffKindDJ, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	hosts, err := s.staff.List(ctx, storage.StaffKindHost, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page.DJCount, page.HostCount = len(djs), len(hosts)

	s.render(w, r, s.home, "home.html", page)
}

func (s *Site) serveStaff(kind string) http.HandlerFunc {
	role, plural, title, desc := "DJ", "DJs", "Our DJs · Muddy's Music Cafe",
		"Meet the hand-picked DJs of Muddy's Music Cafe, spinning live in Second Life 24/7 since 2010."
	if kind == storage.StaffKindHost {
		role, plural, title, desc = "Host", "hosts", "Our Hosts · Muddy's Music Cafe",
			"Meet the friendly hosts of Muddy's Music Cafe, welcoming you in Second Life 24/7 since 2010."
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, _, _, err := s.loadCommon(ctx, title, desc)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		c.Active = kind
		images, err := s.images.List(ctx)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		members, err := s.staff.List(ctx, kind, false)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		page := staffPage{common: c, Kind: kind, Role: role, RolePlural: plural}
		for _, m := range members {
			page.Staff = append(page.Staff, toStaffView(m, images))
		}
		s.render(w, r, s.staffPage, "staff.html", page)
	}
}

func toEventView(e storage.Event, images map[int64]storage.Image, loc *time.Location) eventView {
	v := eventView{
		Title: e.Title, Summary: e.Summary, BadgeStyle: e.BadgeStyle, Featured: e.Featured,
		Badge:   e.BadgeText,
		LinkURL: e.LinkURL, LinkLabel: e.LinkLabel, Link2URL: e.Link2URL, Link2Label: e.Link2Label,
	}
	if v.Badge == "" {
		v.Badge = dateRange(e.StartsAt, e.EndsAt, loc)
	}
	if e.PosterImageID != nil {
		if img, ok := images[*e.PosterImageID]; ok {
			v.PosterURL, v.PosterW, v.PosterH = "/uploads/"+img.Filename, img.Width, img.Height
		}
	}
	if v.LinkURL != "" && v.LinkLabel == "" {
		v.LinkLabel = "Learn more"
	}
	if v.Link2URL != "" && v.Link2Label == "" {
		v.Link2Label = "Learn more"
	}
	return v
}

func toStaffView(m storage.StaffMember, images map[int64]storage.Image) staffView {
	pre, short := splitName(m.Name)
	v := staffView{
		Anchor: m.Kind + "-" + m.Slug, SearchName: strings.ToLower(m.Name),
		Name: m.Name, Pre: pre, ShortName: short,
		Bio: paragraphs(m.Bio), Schedule: lines(m.Schedule), SLName: m.SLName,
		PhotoURL: "/static/images/logo.png",
	}
	v.Teaser = teaser(v.Bio)
	if m.PhotoImageID != nil {
		if img, ok := images[*m.PhotoImageID]; ok {
			v.PhotoURL = "/uploads/" + img.Filename
		}
	}
	return v
}

// splitName turns "DJ Andy" into ("DJ", "Andy").
func splitName(n string) (pre, name string) {
	if len(n) > 3 && strings.EqualFold(n[:2], "dj") && (n[2] == ' ' || n[2] == '\t') {
		return "DJ", strings.TrimSpace(n[3:])
	}
	return "", n
}

func teaser(bio []string) string {
	if len(bio) == 0 {
		return "Bio coming soon."
	}
	t := strings.TrimSpace(bio[0])
	if utf8.RuneCountInString(t) <= 120 {
		return t
	}
	r := []rune(t)
	cut := strings.LastIndex(string(r[:118]), " ")
	if cut < 0 {
		return string(r[:118]) + "…"
	}
	return string(r[:118])[:cut] + "…"
}

// paragraphs splits text on blank lines.
func paragraphs(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out []string
	for _, p := range strings.Split(s, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// lines splits text on newlines, dropping blanks.
func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// dateRange formats an event's dates for the badge: "Oct 29", "Oct 29 – 31"
// or "Oct 29 – Nov 1". Empty when there is no start date.
func dateRange(start, end *time.Time, loc *time.Location) string {
	if start == nil {
		return ""
	}
	a := start.In(loc)
	if end == nil {
		return a.Format("Jan 2")
	}
	b := end.In(loc)
	switch {
	case a.Year() == b.Year() && a.YearDay() == b.YearDay():
		return a.Format("Jan 2")
	case a.Month() == b.Month() && a.Year() == b.Year():
		return a.Format("Jan 2") + " – " + b.Format("2")
	default:
		return a.Format("Jan 2") + " – " + b.Format("Jan 2")
	}
}
