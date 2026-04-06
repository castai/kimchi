package config

type Config struct {
	APIKey      string `json:"api_key"`
	GitHubToken string `json:"github_token,omitempty"`
}
