package recipe

// ScrubSecrets removes known secret fields from a map[string]any that was
// parsed from opencode.json. It operates on provider option maps, deleting
// apiKey, api_key, token, and secret keys from every provider's options block.
func ScrubSecrets(cfg map[string]any) map[string]any {
	providers, ok := cfg["provider"].(map[string]any)
	if !ok {
		return cfg
	}
	for _, v := range providers {
		prov, ok := v.(map[string]any)
		if !ok {
			continue
		}
		opts, ok := prov["options"].(map[string]any)
		if !ok {
			continue
		}
		delete(opts, "apiKey")
		delete(opts, "api_key")
		delete(opts, "token")
		delete(opts, "secret")
	}
	return cfg
}
