package cobra

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	completionShell string
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Сгенерировать скрипт автодополнения оболочки",
	Long: `Сгенерировать скрипт автодополнения оболочки для lito.

Для загрузки автодополнений в текущей сессии оболочки:
  bash:       source <(lito completion bash)
  zsh:        source <(lito completion zsh)
  fish:       lito completion fish | source
  powershell: lito completion powershell | Out-String | Invoke-Expression

Для загрузки автодополнений для каждой новой сессии, выполните один раз:
  Linux/bash:
    lito completion bash > /etc/bash_completion.d/lito
    lito completion bash > ~/.local/share/bash-completion/completions/lito
  macOS/bash:
    lito completion bash > /usr/local/etc/bash_completion.d/lito
  zsh:
    # Если автодополнение оболочки еще не включено в вашей среде,
    # включите его, выполнив:
    #   echo "autoload -U compinit; compinit" >> ~/.zshrc
    # Для загрузки автодополнений для каждой сессии, выполните один раз:
    lito completion zsh > "${fpath[1]}/_lito"
    # Вам потребуется запустить новую оболочку, чтобы настройки вступили в силу.
  fish:
    lito completion fish > ~/.config/fish/completions/lito.fish`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return cmd.Usage()
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
