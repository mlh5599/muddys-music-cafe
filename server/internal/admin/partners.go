package admin

import (
	"net/http"

	"github.com/mlh5599/muddys-music-cafe/server/internal/storage"
)

type partnerBody struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Active    bool   `json:"active"`
}

func partnerOut(p storage.Partner) partnerBody {
	return partnerBody{ID: p.ID, Name: p.Name, SortOrder: p.SortOrder, Active: p.Active}
}

func partnerIn(b partnerBody) (storage.Partner, validator) {
	v := validator{}
	p := storage.Partner{Name: v.line("name", "the name", b.Name, 120, true), Active: b.Active}
	return p, v
}

func (a *api) listPartners(w http.ResponseWriter, r *http.Request) {
	list, err := a.partners.List(r.Context(), true)
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]partnerBody, 0, len(list))
	for _, p := range list {
		out = append(out, partnerOut(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) createPartner(w http.ResponseWriter, r *http.Request) {
	var b partnerBody
	if !decode(w, r, &b) {
		return
	}
	p, v := partnerIn(b)
	if len(v) > 0 {
		writeFieldErrors(w, v)
		return
	}
	p, err := a.partners.Create(r.Context(), p)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, partnerOut(p))
}

func (a *api) updatePartner(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b partnerBody
	if !decode(w, r, &b) {
		return
	}
	p, v := partnerIn(b)
	if len(v) > 0 {
		writeFieldErrors(w, v)
		return
	}
	p.ID = id
	if err := a.partners.Update(r.Context(), p); err != nil {
		storeError(w, err)
		return
	}
	got, err := a.partners.Get(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, partnerOut(got))
}

func (a *api) deletePartner(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.partners.Delete(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) reorderPartners(w http.ResponseWriter, r *http.Request) {
	var b orderBody
	if !decode(w, r, &b) {
		return
	}
	if len(b.IDs) == 0 || len(b.IDs) > 1000 {
		writeError(w, http.StatusBadRequest, "We couldn't read that. Please reload the page and try again.")
		return
	}
	if err := a.partners.Reorder(r.Context(), b.IDs); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
