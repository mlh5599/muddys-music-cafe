// Package admin implements the JSON API behind the admin screens.
//
// Every route requires a valid identity assertion from the access gate
// (see Authenticate), and state-changing requests also pass a CSRF check.
// Error messages are written for the people using the admin, not for
// developers.
package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
	"github.com/mlh5599/muddys-music-cafe/server/internal/uploads"
)

// CSRFHeader must be present, with CSRFValue, on every state-changing request.
// Browsers won't add a custom header to a cross-site form post, and a
// cross-site fetch carrying it triggers a CORS preflight we never approve.
const (
	CSRFHeader = "X-Muddys-Admin"
	CSRFValue  = "1"
)

const maxJSONBody = 1 << 20

// Config configures the admin API.
type Config struct {
	DB         *sql.DB
	UploadsDir string
	Verifier   *Verifier // nil only with DevNoAuth
	DevNoAuth  bool      // local development only; never enable on a public listener
}

type api struct {
	events   *storage.EventStore
	staff    *storage.StaffStore
	partners *storage.PartnerStore
	settings *storage.SettingsStore
	images   *storage.ImageStore
	dir      string
}

// New returns the handler for /api/admin/ (mount it without stripping the prefix).
func New(c Config) http.Handler {
	a := &api{
		events:   storage.NewEventStore(c.DB),
		staff:    storage.NewStaffStore(c.DB),
		partners: storage.NewPartnerStore(c.DB),
		settings: storage.NewSettingsStore(c.DB),
		images:   storage.NewImageStore(c.DB),
		dir:      c.UploadsDir,
	}
	mux := http.NewServeMux()
	const p = "/api/admin"
	mux.HandleFunc("GET "+p+"/me", a.me)

	mux.HandleFunc("GET "+p+"/events", a.listEvents)
	mux.HandleFunc("POST "+p+"/events", a.createEvent)
	mux.HandleFunc("GET "+p+"/events/{id}", a.getEvent)
	mux.HandleFunc("PUT "+p+"/events/{id}", a.updateEvent)
	mux.HandleFunc("DELETE "+p+"/events/{id}", a.deleteEvent)

	mux.HandleFunc("GET "+p+"/staff", a.listStaff)
	mux.HandleFunc("POST "+p+"/staff", a.createStaff)
	mux.HandleFunc("PUT "+p+"/staff/order", a.reorderStaff)
	mux.HandleFunc("GET "+p+"/staff/{id}", a.getStaff)
	mux.HandleFunc("PUT "+p+"/staff/{id}", a.updateStaff)
	mux.HandleFunc("DELETE "+p+"/staff/{id}", a.deleteStaff)

	mux.HandleFunc("GET "+p+"/partners", a.listPartners)
	mux.HandleFunc("POST "+p+"/partners", a.createPartner)
	mux.HandleFunc("PUT "+p+"/partners/order", a.reorderPartners)
	mux.HandleFunc("PUT "+p+"/partners/{id}", a.updatePartner)
	mux.HandleFunc("DELETE "+p+"/partners/{id}", a.deletePartner)

	mux.HandleFunc("GET "+p+"/settings", a.getSettings)
	mux.HandleFunc("PUT "+p+"/settings", a.putSettings)

	mux.HandleFunc("POST "+p+"/images", a.uploadImage)

	// Unknown API paths get a JSON 404 rather than falling through to a page.
	mux.HandleFunc(p+"/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "That doesn't exist.")
	})

	var h http.Handler = mux
	h = csrf(h)
	h = Authenticate(c.Verifier, c.DevNoAuth)(h)
	return noStore(h)
}

// csrf rejects state-changing requests that lack the custom header or come
// from a different origin.
func csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get(CSRFHeader) != CSRFValue || !sameOrigin(r) {
				writeError(w, http.StatusForbidden, "Something went wrong with that request. Please reload the page and try again.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true // non-browser client; the custom header already gates it
	}
	u, err := url.Parse(o)
	return err == nil && u.Host == r.Host
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// ---- helpers ----

type errorBody struct {
	Error string            `json:"error"`
	Field map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

func writeFieldErrors(w http.ResponseWriter, f map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity, errorBody{
		Error: "Please check the highlighted items and try again.",
		Field: f,
	})
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("admin: %v", err)
	writeError(w, http.StatusInternalServerError, "Sorry, something went wrong on our end. Please try again in a moment.")
}

// storeError maps storage errors to responses.
func storeError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "We couldn't find that. It may have been removed already.")
		return
	}
	serverError(w, err)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "That's too much text to save at once.")
		} else {
			writeError(w, http.StatusBadRequest, "We couldn't read that. Please reload the page and try again.")
		}
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		writeError(w, http.StatusBadRequest, "We couldn't read that. Please reload the page and try again.")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "We couldn't find that.")
		return 0, false
	}
	return id, true
}

// validator collects plain-language problems keyed by field name.
type validator map[string]string

func (v validator) text(field, label, val string, max int, required bool) string {
	val = strings.TrimSpace(strings.ReplaceAll(val, "\r\n", "\n"))
	switch {
	case required && val == "":
		v[field] = "Please fill in " + label + "."
	case utf8.RuneCountInString(val) > max:
		v[field] = label + " is too long. Please keep it under " + strconv.Itoa(max) + " characters."
	case strings.ContainsRune(val, 0):
		v[field] = label + " has characters we can't use."
	}
	return val
}

func (v validator) line(field, label, val string, max int, required bool) string {
	val = v.text(field, label, val, max, required)
	if strings.ContainsAny(val, "\r\n") {
		v[field] = label + " should be on a single line."
	}
	return val
}

func (v validator) link(field, label, val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	if utf8.RuneCountInString(val) > 500 {
		v[field] = label + " is too long."
		return val
	}
	u, err := url.Parse(val)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		v[field] = label + " should be a web address starting with http:// or https://"
	}
	return val
}

func parseTime(v validator, field, label string, s *string) *time.Time {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(*s))
	if err != nil {
		v[field] = "Please pick a valid date and time for " + label + "."
		return nil
	}
	t = t.UTC()
	return &t
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func (a *api) imageURL(r *http.Request, id *int64) string {
	if id == nil {
		return ""
	}
	img, err := a.images.Get(r.Context(), *id)
	if err != nil {
		return ""
	}
	return "/uploads/" + img.Filename
}

// checkImage verifies a referenced picture exists.
func (a *api) checkImage(r *http.Request, v validator, field string, id *int64) {
	if id == nil {
		return
	}
	if _, err := a.images.Get(r.Context(), *id); err != nil {
		v[field] = "That picture is no longer available. Please choose it again."
	}
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	id, _ := IdentityFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"email": id.Email})
}

// ---- images ----

type imageBody struct {
	ID     int64  `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (a *api) uploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, uploads.MaxBytes+(1<<20))
	f, _, err := r.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, uploads.ErrTooLarge.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "Please choose a picture to upload.")
		return
	}
	defer f.Close()
	res, err := uploads.Process(f)
	switch {
	case errors.Is(err, uploads.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
		return
	case errors.Is(err, uploads.ErrNotImage), errors.Is(err, uploads.ErrTooMany):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case err != nil:
		serverError(w, err)
		return
	}
	name, err := uploads.Store(a.dir, res)
	if err != nil {
		serverError(w, err)
		return
	}
	img, err := a.images.Create(r.Context(), storage.Image{
		Filename: name, ContentType: res.ContentType, SizeBytes: int64(len(res.Data)),
		Width: res.Width, Height: res.Height,
	})
	if err != nil {
		_ = uploads.Remove(a.dir, name)
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, imageBody{ID: img.ID, URL: "/uploads/" + name, Width: img.Width, Height: img.Height})
}

// ---- settings ----

func (a *api) getSettings(w http.ResponseWriter, r *http.Request) {
	all, err := a.settings.All(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	out := map[string]string{}
	for _, k := range storage.SettingKeys {
		out[k] = all[k]
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) putSettings(w http.ResponseWriter, r *http.Request) {
	var in map[string]string
	if !decode(w, r, &in) {
		return
	}
	v := validator{}
	clean := map[string]string{}
	for k, val := range in {
		switch k {
		case storage.SettingRadioStreamURL, storage.SettingRadioStatusURL, storage.SettingClubSLurl,
			storage.SettingSLGroupURL, storage.SettingFacebookURL, storage.SettingFlickrURL, storage.SettingYouTubeURL:
			clean[k] = v.link(k, "That link", val)
		case storage.SettingHiringText:
			clean[k] = v.text(k, "That text", val, 2000, false)
		case storage.SettingTimezone:
			val = strings.TrimSpace(val)
			if val != "" {
				if _, err := time.LoadLocation(val); err != nil {
					v[k] = "That time zone isn't recognised."
				}
			}
			clean[k] = val
		default:
			writeError(w, http.StatusBadRequest, "We couldn't read that. Please reload the page and try again.")
			return
		}
	}
	if len(v) > 0 {
		writeFieldErrors(w, v)
		return
	}
	if err := a.settings.SetMany(r.Context(), clean); err != nil {
		serverError(w, err)
		return
	}
	a.getSettings(w, r)
}
