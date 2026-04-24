package tools

func init() {
	register(Tool{
		ID:          ToolGeneric,
		Name:        "Generic",
		Description: "Export env vars (stdout)",
		ConfigPath:  "",
		BinaryName:  "",
		IsInstalled: func() bool { return false },
		// No Write handler: the Generic tool has no on-disk config. The
		// configure step renders the env-var instructions inside the TUI.
	})
}
