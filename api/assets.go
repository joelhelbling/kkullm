package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

func (s *Server) listAssets(w http.ResponseWriter, r *http.Request) {
	params := store.AssetListParams{
		Project:  r.URL.Query().Get("project"),
		NameGlob: r.URL.Query().Get("name"),
		URLGlob:  r.URL.Query().Get("url"),
	}

	assets, err := s.store.ListAssets(params)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if assets == nil {
		assets = []model.ProjectAsset{}
	}
	writeJSON(w, 200, assets)
}

func (s *Server) createAsset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Project     string `json:"project"`
		Description string `json:"description"`
		URL         string `json:"url"`
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

	asset, err := s.store.CreateAsset(project.ID, body.Name, body.Description, body.URL)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, asset)
}

func (s *Server) getAsset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	asset, err := s.store.GetAsset(id)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, asset)
}
