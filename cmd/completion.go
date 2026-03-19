package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `To load completions:

Bash:
  $ source <(kimchi completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ kimchi completion bash > /etc/bash_completion.d/kimchi
  # macOS:
  $ kimchi completion bash > $(brew --prefix)/etc/bash_completion.d/kimchi

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ kimchi completion zsh > "${fpath[1]}/_kimchi"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ kimchi completion fish | source

  # To load completions for each session, execute once:
  $ kimchi completion fish > ~/.config/fish/completions/kimchi.fish

PowerShell:
  PS> kimchi completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> kimchi completion powershell > kimchi.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			switch args[0] {
			case "bash":
				err = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				err = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				err = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				err = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}
}
