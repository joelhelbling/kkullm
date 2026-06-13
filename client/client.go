package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/joelhelbling/kkullm/model"
)

// agentHeader carries the acting-agent identity (the CLI's --as / KKULLM_AGENT)
// to the API on every request, so mutations can be attributed in the audit trail.
const agentHeader = "X-Kkullm-Agent"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	// Agent is the acting-agent identity sent on every request via the
	// X-Kkullm-Agent header. Empty for read-only/anonymous use.
	Agent string
}

func New(baseURL string, agent ...string) *Client {
	c := &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
	if len(agent) > 0 {
		c.Agent = agent[0]
	}
	return c
}

type apiError struct {
	Error string `json:"error"`
}

func (c *Client) do(method, path string, body any, result any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Agent != "" {
		req.Header.Set(agentHeader, c.Agent)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr apiError
		if json.Unmarshal(respData, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, apiErr.Error)
		}
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respData))
	}

	if result != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// --- Projects ---

func (c *Client) ListProjects() ([]model.Project, error) {
	var projects []model.Project
	err := c.do("GET", "/api/projects", nil, &projects)
	return projects, err
}

func (c *Client) CreateProject(name, description string) (*model.Project, error) {
	body := map[string]string{"name": name, "description": description}
	var project model.Project
	err := c.do("POST", "/api/projects", body, &project)
	return &project, err
}

// --- Agents ---

func (c *Client) ListAgents(project string) ([]model.Agent, error) {
	path := "/api/agents"
	if project != "" {
		path += "?project=" + url.QueryEscape(project)
	}
	var agents []model.Agent
	err := c.do("GET", path, nil, &agents)
	return agents, err
}

func (c *Client) CreateAgent(name, project, bio string) (*model.Agent, error) {
	body := map[string]string{"name": name, "project": project, "bio": bio}
	var agent model.Agent
	err := c.do("POST", "/api/agents", body, &agent)
	return &agent, err
}

func (c *Client) GetAgent(id int) (*model.Agent, error) {
	var agent model.Agent
	err := c.do("GET", "/api/agents/"+strconv.Itoa(id), nil, &agent)
	return &agent, err
}

// --- Cards ---

type CardCreateRequest struct {
	Title     string               `json:"title"`
	Body      string               `json:"body,omitempty"`
	Status    string               `json:"status,omitempty"`
	Project   string               `json:"project"`
	Assignees []string             `json:"assignees,omitempty"`
	Tags      []string             `json:"tags,omitempty"`
	Relations []model.CardRelation `json:"relations,omitempty"`
}

type CardUpdateRequest struct {
	Title     *string              `json:"title,omitempty"`
	Body      *string              `json:"body,omitempty"`
	Status    *string              `json:"status,omitempty"`
	Blocked   *bool                `json:"blocked,omitempty"`
	Assignees []string             `json:"assignees,omitempty"`
	Tags      []string             `json:"tags,omitempty"`
	Relations []model.CardRelation `json:"relations,omitempty"`
	// Force bypasses the status-transition matrix (#35). Status validity is
	// still enforced server-side; only the transition rule is skipped.
	Force bool `json:"force,omitempty"`
}

type CardListOptions struct {
	Project      string
	Assignee     string
	Status       string
	Tag          string
	ArchiveLimit int
	ArchiveView  string
}

func (c *Client) ListCards(opts CardListOptions) ([]model.Card, error) {
	params := url.Values{}
	if opts.Project != "" {
		params.Set("project", opts.Project)
	}
	if opts.Assignee != "" {
		params.Set("assignee", opts.Assignee)
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}
	if opts.Tag != "" {
		params.Set("tag", opts.Tag)
	}
	if opts.ArchiveLimit > 0 {
		params.Set("archive_limit", strconv.Itoa(opts.ArchiveLimit))
	}
	if opts.ArchiveView != "" {
		params.Set("archive_view", opts.ArchiveView)
	}
	path := "/api/cards"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var cards []model.Card
	err := c.do("GET", path, nil, &cards)
	return cards, err
}

func (c *Client) GetCard(id int) (*model.Card, error) {
	var card model.Card
	err := c.do("GET", "/api/cards/"+strconv.Itoa(id), nil, &card)
	return &card, err
}

func (c *Client) CreateCard(req CardCreateRequest) (*model.Card, error) {
	var card model.Card
	err := c.do("POST", "/api/cards", req, &card)
	return &card, err
}

func (c *Client) UpdateCard(id int, req CardUpdateRequest) (*model.Card, error) {
	var card model.Card
	err := c.do("PATCH", "/api/cards/"+strconv.Itoa(id), req, &card)
	return &card, err
}

func (c *Client) ListCardEvents(cardID int) ([]model.CardEvent, error) {
	var events []model.CardEvent
	err := c.do("GET", "/api/cards/"+strconv.Itoa(cardID)+"/events", nil, &events)
	return events, err
}

// --- Comments ---

func (c *Client) ListComments(cardID int) ([]model.Comment, error) {
	var comments []model.Comment
	err := c.do("GET", "/api/cards/"+strconv.Itoa(cardID)+"/comments", nil, &comments)
	return comments, err
}

func (c *Client) CreateComment(cardID int, agent, body, kind string) (*model.Comment, error) {
	reqBody := map[string]string{"agent": agent, "body": body}
	if kind != "" {
		reqBody["kind"] = kind
	}
	var comment model.Comment
	err := c.do("POST", "/api/cards/"+strconv.Itoa(cardID)+"/comments", reqBody, &comment)
	return &comment, err
}

// --- Assets ---

func (c *Client) ListAssets(project, nameGlob, urlGlob string) ([]model.ProjectAsset, error) {
	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	if nameGlob != "" {
		params.Set("name", nameGlob)
	}
	if urlGlob != "" {
		params.Set("url", urlGlob)
	}
	path := "/api/assets"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var assets []model.ProjectAsset
	err := c.do("GET", path, nil, &assets)
	return assets, err
}

func (c *Client) CreateAsset(project, name, description, assetURL string) (*model.ProjectAsset, error) {
	body := map[string]string{
		"project":     project,
		"name":        name,
		"description": description,
		"url":         assetURL,
	}
	var asset model.ProjectAsset
	err := c.do("POST", "/api/assets", body, &asset)
	return &asset, err
}

func (c *Client) GetAsset(id int) (*model.ProjectAsset, error) {
	var asset model.ProjectAsset
	err := c.do("GET", "/api/assets/"+strconv.Itoa(id), nil, &asset)
	return &asset, err
}
