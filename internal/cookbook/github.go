package cookbook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// githubClientID is the GitHub OAuth App client ID compiled into the binary.
// Override at build time: -ldflags "-X github.com/castai/kimchi/internal/cookbook.githubClientID=<id>"
// or set KIMCHI_GITHUB_CLIENT_ID at runtime.
// TODO: this is a temporary Github APP - replace it with official one before release
const githubClientID = "Ov23liNx4u53Pg5wkjmg"

// DeviceCodeResponse is returned by RequestDeviceCode.
type DeviceCodeResponse struct {
	DeviceCode      string
	UserCode        string
	VerificationURL string
	ExpiresIn       int
	Interval        int // seconds between poll attempts
}

// RequestDeviceCode starts a GitHub device auth flow and returns the codes to show the user.
// Set KIMCHI_GITHUB_CLIENT_ID or compile in githubClientID before calling.
func RequestDeviceCode() (DeviceCodeResponse, error) {
	cid := os.Getenv("KIMCHI_GITHUB_CLIENT_ID")
	if cid == "" {
		cid = githubClientID
	}
	if cid == "" {
		return DeviceCodeResponse{}, fmt.Errorf(
			"GitHub OAuth app not configured — set KIMCHI_GITHUB_CLIENT_ID",
		)
	}

	resp, err := http.PostForm("https://github.com/login/device/code", url.Values{
		"client_id": {cid},
		"scope":     {"repo"},
	})
	if err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("request device code: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	vals, _ := url.ParseQuery(string(body))
	dcr := DeviceCodeResponse{
		DeviceCode:      vals.Get("device_code"),
		UserCode:        vals.Get("user_code"),
		VerificationURL: vals.Get("verification_uri"),
		ExpiresIn:       900,
		Interval:        5,
	}
	fmt.Sscanf(vals.Get("expires_in"), "%d", &dcr.ExpiresIn)
	fmt.Sscanf(vals.Get("interval"), "%d", &dcr.Interval)
	return dcr, nil
}

// PollForToken attempts a single token exchange. Returns ("", nil) when still pending.
func PollForToken(deviceCode string) (token string, err error) {
	cid := os.Getenv("KIMCHI_GITHUB_CLIENT_ID")
	if cid == "" {
		cid = githubClientID
	}

	resp, err := http.PostForm("https://github.com/login/oauth/access_token", url.Values{
		"client_id":   {cid},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	vals, _ := url.ParseQuery(string(body))

	if token = vals.Get("access_token"); token != "" {
		return token, nil
	}
	switch vals.Get("error") {
	case "authorization_pending", "slow_down":
		return "", nil // still waiting
	case "":
		return "", nil
	default:
		return "", fmt.Errorf("%s: %s", vals.Get("error"), vals.Get("error_description"))
	}
}

// ValidateToken checks that the token is accepted by the GitHub API and returns
// the authenticated user's login.
func ValidateToken(token string) (string, error) {
	username, err := GetUsername(token)
	if err != nil {
		return "", fmt.Errorf("invalid GitHub token: %w", err)
	}
	return username, nil
}

// GetUsername returns the authenticated user's login.
func GetUsername(token string) (string, error) {
	var result struct {
		Login string `json:"login"`
	}
	if err := githubGet(token, "https://api.github.com/user", &result); err != nil {
		return "", err
	}
	return result.Login, nil
}

// ForkRepo forks owner/repo on behalf of the authenticated user.
// Returns the HTTPS clone URL of the fork.
func ForkRepo(token, owner, repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/forks", owner, repo)
	var result struct {
		CloneURL string `json:"clone_url"`
		HTMLURL  string `json:"html_url"`
	}
	if err := githubPost(token, apiURL, nil, &result); err != nil {
		return "", fmt.Errorf("fork %s/%s: %w", owner, repo, err)
	}
	// GitHub may return 202 (fork is being created); wait briefly
	if result.CloneURL == "" {
		time.Sleep(3 * time.Second)
		forkURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", githubUsername(token), repo)
		if err := githubGet(token, forkURL, &result); err != nil {
			return "", fmt.Errorf("get fork: %w", err)
		}
	}
	return result.CloneURL, nil
}

// CreatePR opens a pull request. head should be "user:branch".
// Returns the PR URL.
func CreatePR(token, owner, repo, head, base, title, body string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)
	payload := map[string]string{
		"title": title,
		"head":  head,
		"base":  base,
		"body":  body,
	}
	var result struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	if err := githubPost(token, apiURL, payload, &result); err != nil {
		return "", fmt.Errorf("create PR: %w", err)
	}
	return result.HTMLURL, nil
}

// FindOpenPR returns the number and URL of an open PR whose head matches, or 0/"" if none.
func FindOpenPR(token, owner, repo, head string) (int, string, error) {
	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls?state=open&head=%s",
		owner, repo, url.QueryEscape(head),
	)
	var results []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := githubGet(token, apiURL, &results); err != nil {
		return 0, "", err
	}
	if len(results) > 0 {
		return results[0].Number, results[0].HTMLURL, nil
	}
	return 0, "", nil
}

// ParseGitHubURL extracts owner and repo from a GitHub URL (HTTPS or SSH).
func ParseGitHubURL(rawURL string) (owner, repo string, err error) {
	rawURL = strings.TrimSuffix(rawURL, ".git")
	// SSH: git@github.com:owner/repo
	if strings.HasPrefix(rawURL, "git@github.com:") {
		parts := strings.SplitN(strings.TrimPrefix(rawURL, "git@github.com:"), "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}
	// HTTPS: https://github.com/owner/repo
	u, err2 := url.Parse(rawURL)
	if err2 != nil || u.Host != "github.com" {
		return "", "", fmt.Errorf("not a GitHub URL: %s", rawURL)
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("cannot parse GitHub URL path: %s", rawURL)
	}
	return parts[0], parts[1], nil
}

// TokenCloneURL inserts a GitHub token into an HTTPS clone URL.
func TokenCloneURL(token, rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, ".git")
	if strings.HasPrefix(rawURL, "https://") {
		return "https://oauth2:" + token + "@" + strings.TrimPrefix(rawURL, "https://") + ".git"
	}
	return rawURL
}

// --- helpers ---

func githubUsername(token string) string {
	u, _ := GetUsername(token)
	return u
}

func githubGet(token, apiURL string, dest any) error {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API %s: %s", resp.Status, body)
	}
	return json.Unmarshal(body, dest)
}

func githubPost(token, apiURL string, payload any, dest any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API %s: %s", resp.Status, respBody)
	}
	if dest != nil {
		return json.Unmarshal(respBody, dest)
	}
	return nil
}
