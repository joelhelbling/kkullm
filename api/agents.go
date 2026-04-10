package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/joelhelbling/kkullm/model"
)

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("project")

	agents, err := s.store.ListAgents(projectName)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if agents == nil {
		agents = []model.Agent{}
	}
	writeJSON(w, 200, agents)
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Project string `json:"project"`
		Bio     string `json:"bio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if body.Name == "" {
		writeError(w, 400, "name is required")
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

	agent, err := s.store.CreateAgent(body.Name, project.ID, body.Bio)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, agent)
}

func (s *Server) getAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	agent, err := s.store.GetAgent(id)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, agent)
}
