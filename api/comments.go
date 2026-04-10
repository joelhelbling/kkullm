package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/joelhelbling/kkullm/model"
)

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	cardID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid card id")
		return
	}

	comments, err := s.store.ListComments(cardID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if comments == nil {
		comments = []model.Comment{}
	}
	writeJSON(w, 200, comments)
}

func (s *Server) createComment(w http.ResponseWriter, r *http.Request) {
	cardID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid card id")
		return
	}

	var body struct {
		Agent string `json:"agent"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if body.Agent == "" {
		writeError(w, 400, "agent is required")
		return
	}
	if body.Body == "" {
		writeError(w, 400, "body is required")
		return
	}

	agent, err := s.store.GetAgentByName(body.Agent)
	if err != nil {
		writeError(w, 404, "agent not found: "+body.Agent)
		return
	}

	comment, err := s.store.CreateComment(cardID, agent.ID, body.Body)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	s.events.Publish(Event{Type: "comment_created", Data: comment})
	writeJSON(w, 201, comment)
}
