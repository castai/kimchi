package cookbook

// Cookbook represents a registered recipe repository.
type Cookbook struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Path string `json:"path"` // absolute path to local clone
}

// RecipePath returns the path where a recipe yaml lives inside the cookbook.
func (c *Cookbook) RecipePath(recipeName string) string {
	return c.Path + "/recipes/" + recipeName + "/recipe.yaml"
}
