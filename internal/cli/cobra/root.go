package cobra

import (
	"os"

	"github.com/spf13/cobra"
)

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "lito",
	Short: "Инструмент для моделирования и визуализации береговой геометрии",
	Long: `Lito - это инструмент командной строки для моделирования геометрии
побережья, включая создание фрактальной береговой линии, моделирование волновой
эрозии и визуализацию на основе батиметрии.`,
	SilenceUsage: true,
}

var (
	quiet bool
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "тихий режим (подавить вывод)")
}
