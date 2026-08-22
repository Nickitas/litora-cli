package cobra

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Execute запускает корневую команду Lito с русскоязычной справкой.
func Execute() {
	prepareRussianHelp()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

const russianUsageTemplate = `Использование:{{if .Runnable}}
  {{russianUseLine .UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [команда]{{end}}{{if gt (len .Aliases) 0}}

Псевдонимы:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Примеры:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Доступные команды:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Дополнительные команды:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Флаги:
{{russianFlagUsages .LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Глобальные флаги:
{{russianFlagUsages .InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Дополнительные разделы справки:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Используйте «{{.CommandPath}} [команда] --help» для справки по команде.{{end}}
`

// prepareRussianHelp удаляет англоязычные подписи стандартной справки Cobra
// для корневой команды и всех подкоманд Lito.
func prepareRussianHelp() {
	cobra.AddTemplateFunc("russianUseLine", func(value string) string {
		return strings.ReplaceAll(value, "[flags]", "[флаги]")
	})
	cobra.AddTemplateFunc("russianFlagUsages", func(value string) string {
		return strings.ReplaceAll(value, "(default ", "(по умолчанию ")
	})
	rootCmd.SetUsageTemplate(russianUsageTemplate)
	rootCmd.InitDefaultHelpCmd()
	localizeCommandHelp(rootCmd)
}

func localizeCommandHelp(command *cobra.Command) {
	command.InitDefaultHelpFlag()
	if helpFlag := command.Flags().Lookup("help"); helpFlag != nil {
		helpFlag.Usage = "показать справку"
	}
	if command.Name() == "help" {
		command.Short = "Справка по любой команде"
		command.Long = "Показать справку по указанной команде Lito."
	}
	for _, child := range command.Commands() {
		localizeCommandHelp(child)
	}
}

var rootCmd = &cobra.Command{
	Use:   "lito",
	Short: "Моделирование береговой геометрии и дна Чёрного моря",
	Long: `Lito - это инструмент командной строки для моделирования геометрии
Чёрного моря: анализ наблюдаемой береговой линии методом box-counting,
моделирование волновой эрозии и визуализация рельефа дна. Входные данные
других акваторий не поддерживаются. Учебная Кох-подобная генерация вынесена
в отдельную команду koch-demo.`,
	SilenceUsage: true,
}

var (
	quiet bool
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "тихий режим (подавить вывод)")
}
