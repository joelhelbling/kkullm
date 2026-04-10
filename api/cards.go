package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

func (s *Server) listCards(w http.ResponseWriter, r *http.Request) {
	params := store.CardListParams{
		Project:  r.URL.Query().Get("project"),
		Assignee: r.URL.Query().Get("assignee"),
		Status:   r.URL.Query().Get("status"),
		Tag:      r.URL.Query().Get("tag"),
	}

	cards, err := s.store.ListCards(params)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if cards == nil {
		cards = []model.Card{}
	}
	writeJSON(w, 200, cards)
}

func (s *Server) createCard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title     string               `json:"title"`
		Body      string               `json:"body"`
		Status    string               `json:"status"`
		Project   string               `json:"project"`
		Assignees []string             `json:"assignees"`
		Tags      []string             `json:"tags"`
		Relations []model.CardRelation `json:"relations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeError(w, 400, "title is required")
		return
	}
	if body.Project == "" {
		writeError(w, 400, "project is required")
		return
	}

	project, err := s.store.GetProjectByName(body.Project)
	if err != nil {
		writeError(w, 404, "project not found: "+body.Project)
		return
	}

	card, err := s.store.CreateCard(store.CardCreateParams{
		Title:     body.Title,
		Body:      body.Body,
		Status:    body.Status,
		ProjectID: project.ID,
		Assignees: body.Assignees,
		Tags:      body.Tags,
		Relations: body.Relations,
	})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	s.events.Publish(Event{Type: "card_created", Data: card})
	writeJSON(w, 201, card)
}

func (s *Server) getCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	card, err := s.store.GetCard(id)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, card)
}

func (s *Server) updateCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	var body struct {
		Title     *string              `json:"title"`
		Body      *string              `json:"body"`
		Status    *string              `json:"status"`
		Assignees []string             `json:"assignees"`
		Tags      []string             `json:"tags"`
		Relations []model.CardRelation `json:"relations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}

	card, err := s.store.UpdateCard(id, store.CardUpdateParams{
		Title:     body.Title,
		Body:      body.Body,
		Status:    body.Status,
		Assignees: body.Assignees,
		Tags:      body.Tags,
		Relations: body.Relations,
	})
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}

	s.events.Publish(Event{Type: "card_updated", Data: card})
	writeJSON(w, 200, card)
}

func (s *Server) deleteCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	if err := s.store.DeleteCard(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}

	s.events.Publish(Event{Type: "card_deleted", Data: map[string]int{"id": id}})
	w.WriteHeader(204)
}
