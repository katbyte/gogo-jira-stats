package j

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
)

const IssuePageSize = 50

// searchJQLRequest is the POST body for /rest/api/3/search/jql
type searchJQLRequest struct {
	Expand        string   `json:"expand,omitempty"`
	JQL           string   `json:"jql"`
	MaxResults    int      `json:"maxResults,omitempty"`
	Fields        []string `json:"fields,omitempty"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

// searchJQLResponse is the response from /rest/api/3/search/jql
type searchJQLResponse struct {
	Issues        []*models.IssueScheme `json:"issues,omitempty"`
	NextPageToken string                `json:"nextPageToken,omitempty"`
	Total         int                   `json:"total,omitempty"`
	MaxResults    int                   `json:"maxResults,omitempty"`
	StartAt       int                   `json:"startAt,omitempty"`
}

// list all issues for a jql with a callback per api request, using the new /rest/api/3/search/jql endpoint
func (i Instance) ListAllIssues(jql string, fields, expand *[]string, cb func(*models.IssueSearchScheme, *models.ResponseScheme) error) error {
	nextPageToken := ""

	for {
		reqBody := searchJQLRequest{
			JQL:           jql,
			MaxResults:    IssuePageSize,
			Expand:        "changelog",
			Fields:        []string{"summary", "status", "issuetype", "resolution", "labels", "creator", "created", "updated"},
			NextPageToken: nextPageToken,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal search request: %w", err)
		}

		searchURL := strings.TrimRight(i.URL, "/") + "/rest/api/3/search/jql"
		req, err := http.NewRequest(http.MethodPost, searchURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth(i.User, i.Token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("jira search request failed: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("jira search failed (status %d): %s", resp.StatusCode, string(respBody))
		}

		var searchResp searchJQLResponse
		if err := json.Unmarshal(respBody, &searchResp); err != nil {
			return fmt.Errorf("failed to parse search response: %w", err)
		}

		// convert to models.IssueSearchScheme for compatibility with callers
		result := &models.IssueSearchScheme{
			StartAt:    searchResp.StartAt,
			MaxResults: searchResp.MaxResults,
			Total:      searchResp.Total,
			Issues:     searchResp.Issues,
		}

		if err = cb(result, nil); err != nil {
			return fmt.Errorf("callback failed for %s @ %s: %w", i.URL, jql, err)
		}

		// if no next page token, we're done
		if searchResp.NextPageToken == "" {
			break
		}
		nextPageToken = searchResp.NextPageToken
	}

	return nil
}

// make multiple calls to get all issues for jql
func (i Instance) GetAllIssues(jql string, fields, expand *[]string) (*[]models.IssueScheme, error) {
	var allIssues []models.IssueScheme

	err := i.ListAllIssues(jql, fields, expand, func(results *models.IssueSearchScheme, resp *models.ResponseScheme) error {
		for _, i := range results.Issues {
			allIssues = append(allIssues, *i)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all issues for %s @ %s: %w", i.URL, jql, err)
	}

	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].Key < allIssues[j].Key
	})

	return &allIssues, nil
}
