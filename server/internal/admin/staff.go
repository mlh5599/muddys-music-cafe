package admin

import (
	"net/http"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

type staffBody struct {
	ID           int64  `json:"id"`
	Kind         string `json:"kind"`
	Slug         string `json:"slug,omitempty"`
	Name         string `json:"name"`
	SLName       string `json:"slName"`
	Bio          string `json:"bio"`
	Schedule     string `json:"schedule"`
	PhotoImageID *int64 `json:"photoImageId"`
	PhotoURL     string `json:"photoUrl,omitempty"`
	SortOrder    int    `json:"sortOrder"`
	Active       bool   `json:"active"`
}

type orderBody struct {
	Kind string  `json:"kind"`
	IDs  []int64 `json:"ids"`
}

func (a *api) staffOut(r *http.Request, m storage.StaffMember) staffBody {
	return staffBody{
		ID: m.ID, Kind: m.Kind, Slug: m.Slug, Name: m.Name, SLName: m.SLName, Bio: m.Bio,
		Schedule: m.Schedule, PhotoImageID: m.PhotoImageID, PhotoURL: a.imageURL(r, m.PhotoImageID),
		SortOrder: m.SortOrder, Active: m.Active,
	}
}

func validKind(k string) bool { return k == storage.StaffKindDJ || k == storage.StaffKindHost }

func (a *api) staffIn(r *http.Request, b staffBody) (storage.StaffMember, validator) {
	v := validator{}
	m := storage.StaffMember{Kind: b.Kind, PhotoImageID: b.PhotoImageID, Active: b.Active}
	if !validKind(m.Kind) {
		v["kind"] = "Please choose whether this is a DJ or a host."
	}
	m.Name = v.line("name", "the name", b.Name, 80, true)
	m.SLName = v.line("slName", "the Second Life name", b.SLName, 80, false)
	m.Bio = v.text("bio", "the bio", b.Bio, 5000, false)
	m.Schedule = v.text("schedule", "the schedule", b.Schedule, 1000, false)
	a.checkImage(r, v, "photoImageId", m.PhotoImageID)
	return m, v
}

func (a *api) listStaff(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if !validKind(kind) {
		writeError(w, http.StatusBadRequest, "Please choose whether you want DJs or hosts.")
		return
	}
	list, err := a.staff.List(r.Context(), kind, true)
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]staffBody, 0, len(list))
	for _, m := range list {
		out = append(out, a.staffOut(r, m))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) getStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	m, err := a.staff.Get(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.staffOut(r, m))
}

func (a *api) createStaff(w http.ResponseWriter, r *http.Request) {
	var b staffBody
	if !decode(w, r, &b) {
		return
	}
	m, v := a.staffIn(r, b)
	if len(v) > 0 {
		writeFieldErrors(w, v)
		return
	}
	m, err := a.staff.Create(r.Context(), m)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a.staffOut(r, m))
}

func (a *api) updateStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b staffBody
	if !decode(w, r, &b) {
		return
	}
	cur, err := a.staff.Get(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	b.Kind = cur.Kind // a DJ can't be turned into a host by editing
	m, v := a.staffIn(r, b)
	if len(v) > 0 {
		writeFieldErrors(w, v)
		return
	}
	m.ID = id
	if err := a.staff.Update(r.Context(), m); err != nil {
		storeError(w, err)
		return
	}
	got, err := a.staff.Get(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.staffOut(r, got))
}

func (a *api) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.staff.Delete(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) reorderStaff(w http.ResponseWriter, r *http.Request) {
	var b orderBody
	if !decode(w, r, &b) {
		return
	}
	if !validKind(b.Kind) || len(b.IDs) == 0 || len(b.IDs) > 1000 {
		writeError(w, http.StatusBadRequest, "We couldn't read that. Please reload the page and try again.")
		return
	}
	if err := a.staff.Reorder(r.Context(), b.Kind, b.IDs); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
