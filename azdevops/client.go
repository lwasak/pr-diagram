package azdevops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client calls the Azure DevOps Git REST API.
type Client struct {
	token       string
	projectBase string // https://dev.azure.com/{org}/{project}/_apis/git
	repoBase    string // https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}
	http        *http.Client
}

// NewClient constructs a Client for the given organisation and project.
// The repository is not required upfront — it is resolved from the PR details.
func NewClient(org, project, token string) *Client {
	return &Client{
		token:       token,
		projectBase: fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git", org, project),
		http:        &http.Client{Timeout: 30 * time.Second},
	}
}

// SetRepo sets the repository-scoped base URL once the repo name is known.
func (c *Client) SetRepo(repo string) {
	c.repoBase = c.projectBase + "/repositories/" + repo
}

// PRDetails is the partial response from GET pullRequests/{prId}.
type PRDetails struct {
	Repository struct {
		Name string `json:"name"`
	} `json:"repository"`
	LastMergeSourceCommit struct {
		CommitID string `json:"commitId"`
	} `json:"lastMergeSourceCommit"`
}

// ChangeEntry represents one file change within a PR iteration.
type ChangeEntry struct {
	ChangeType string `json:"changeType"` // "add", "edit", "delete", "rename", …
	Item       struct {
		Path string `json:"path"` // e.g. "/src/Domain/Order.cs"
	} `json:"item"`
}

type iterationsResponse struct {
	Value []struct {
		ID int `json:"id"`
	} `json:"value"`
}

type changesResponse struct {
	ChangeEntries []ChangeEntry `json:"changeEntries"`
}

// GetPRDetails fetches the PR's repository name and source commit ID.
// Uses the project-level endpoint — no repository name required.
func (c *Client) GetPRDetails(ctx context.Context, prID int) (PRDetails, error) {
	u := fmt.Sprintf("%s/pullRequests/%d?api-version=7.1", c.projectBase, prID)
	var out PRDetails
	if err := c.get(ctx, u, &out); err != nil {
		return PRDetails{}, err
	}
	return out, nil
}

// GetLatestIterationID returns the ID of the last (most recent) iteration of the PR.
func (c *Client) GetLatestIterationID(ctx context.Context, prID int) (int, error) {
	u := fmt.Sprintf("%s/pullRequests/%d/iterations?api-version=7.1", c.repoBase, prID)
	var out iterationsResponse
	if err := c.get(ctx, u, &out); err != nil {
		return 0, err
	}
	if len(out.Value) == 0 {
		return 0, fmt.Errorf("PR %d has no iterations (may still be a draft with no pushed commits)", prID)
	}
	return out.Value[len(out.Value)-1].ID, nil
}

// GetChangedFiles returns all change entries for the given PR iteration.
func (c *Client) GetChangedFiles(ctx context.Context, prID, iterationID int) ([]ChangeEntry, error) {
	u := fmt.Sprintf("%s/pullRequests/%d/iterations/%d/changes?api-version=7.1", c.repoBase, prID, iterationID)
	var out changesResponse
	if err := c.get(ctx, u, &out); err != nil {
		return nil, err
	}
	return out.ChangeEntries, nil
}

// DownloadFile fetches the raw content of a file at the given commit.
func (c *Client) DownloadFile(ctx context.Context, path, commitID string) ([]byte, error) {
	u := fmt.Sprintf(
		"%s/items?path=%s&versionDescriptor.version=%s&versionDescriptor.versionType=commit&$format=text&api-version=7.1",
		c.repoBase, url.QueryEscape(path), commitID,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := body
		if len(preview) > 4096 {
			preview = preview[:4096]
		}
		return nil, fmt.Errorf("azure devops GET %s returned %d: %s", path, resp.StatusCode, preview)
	}
	return body, nil
}

// get performs a GET request and JSON-decodes the response into out.
func (c *Client) get(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := body
		if len(preview) > 4096 {
			preview = preview[:4096]
		}
		return fmt.Errorf("azure devops returned %d: %s", resp.StatusCode, preview)
	}
	return json.Unmarshal(body, out)
}

func (c *Client) authHeader() string {
	encoded := base64.StdEncoding.EncodeToString([]byte(":" + c.token))
	return "Basic " + encoded
}
