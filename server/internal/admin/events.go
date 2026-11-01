package admin

import (
	"net/http"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

type eventBody struct {
	ID            int64   `json:"id"`
	Kind          string  `json:"kind"`
	Title         string  `json:"title"`
	Summary       string  `json:"summary"`
	BadgeText     string  `json:"badgeText"`
	BadgeStyle    string  `json:"badgeStyle"`
	StartsAt      *string `json:"startsAt"`
	EndsAt        *string `json:"endsAt"`
	PosterImageID *int64  `json:"posterImageId"`
	PosterURL     string  `json:"posterUrl,omitempty"`
	LinkURL       string  `json:"linkUrl"`
	LinkLabel     string  `json:"linkLabel"`
	Link2URL      string  `json:"link2Url"`
	Link2Label    string  `json:"link2Label"`
	Featured      bool    `json:"featured"`
	Published     bool    `json:"published"`
}

func (a *api) eventOut(r *http.Request, e storage.Event) eventBody {
	return eventBody{
		ID: e.ID, Kind: e.Kind, Title: e.Title, Summary: e.Summary,
		BadgeText: e.BadgeText, BadgeStyle: e.BadgeStyle,
		StartsAt: formatTimePtr(e.StartsAt), EndsAt: formatTimePtr(e.EndsAt),
		PosterImageID: e.PosterImageID, PosterURL: a.imageURL(r, e.PosterImageID),
		LinkURL: e.LinkURL, LinkLabel: e.LinkLabel, Link2URL: e.Link2URL, Link2Label: e.Link2Label,
		Featured: e.Featured, Published: e.Published,
	}
}

// eventIn validates the request body and converts it to a storage event.
func (a *api) eventIn(r *http.Request, b eventBody) (storage.Event, validator) {
	v := validator{}
	e := storage.Event{Kind: b.Kind, PosterImageID: b.PosterImageID, Featured: b.Featured, Published: b.Published}
	if e.Kind == "" {
		e.Kind = storage.EventKindEvent
	}
	if e.Kind != storage.EventKindEvent && e.Kind != storage.EventKindRoad {
		v["kind"] = "Please choose what kind of listing this is."
	}
	e.Title = v.line("title", "the name", b.Title, 120, true)
	e.Summary = v.text("summary", "the description", b.Summary, 2000, false)
	e.BadgeText = v.line("badgeText", "the label", b.BadgeText, 40, false)
	e.BadgeStyle = b.BadgeStyle
	if e.BadgeStyle == "" {
		e.BadgeStyle = "orange"
	}
	switch e.BadgeStyle {
	case "orange", "purple", "blue", "pink":
	default:
		v["badgeStyle"] = "Please pick one of the label colours."
	}
	e.StartsAt = parseTime(v, "startsAt", "the start", b.StartsAt)
	e.EndsAt = parseTime(v, "endsAt", "the end", b.EndsAt)
	if e.StartsAt != nil && e.EndsAt != nil && e.EndsAt.Before(*e.StartsAt) {
		v["endsAt"] = "The end needs to be after the start."
	}
	e.LinkURL = v.link("linkUrl", "The first link", b.LinkURL)
	e.LinkLabel = v.line("linkLabel", "the first link's button text", b.LinkLabel, 60, e.LinkURL != "")
	e.Link2URL = v.link("link2Url", "The second link", b.Link2URL)
	e.Link2Label = v.line("link2Label", "the second link's button text", b.Link2Label, 60, e.Link2URL != "")
	a.checkImage(r, v, "posterImageId", e.PosterImageID)
	return e, v
}

func (a *api) listEvents(w http.ResponseWriter, r *http.Request) {
	list, err := a.events.List(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]eventBody, 0, len(list))
	for _, e := range list {
		out = append(out, a.eventOut(r, e))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) getEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	e, err := a.events.Get(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.eventOut(r, e))
}

func (a *api) createEvent(w http.ResponseWriter, r *http.Request) {
	var b eventBody
	if !decode(w, r, &b) {
		return
	}
	e, v := a.eventIn(r, b)
	if len(v) > 0 {
		writeFieldErrors(w, v)
		return
	}
	e, err := a.events.Create(r.Context(), e)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a.eventOut(r, e))
}

func (a *api) updateEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b eventBody
	if !decode(w, r, &b) {
		return
	}
	e, v := a.eventIn(r, b)
	if len(v) > 0 {
		writeFieldErrors(w, v)
		return
	}
	e.ID = id
	if err := a.events.Update(r.Context(), e); err != nil {
		storeError(w, err)
		return
	}
	got, err := a.events.Get(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.eventOut(r, got))
}

func (a *api) deleteEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.events.Delete(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
