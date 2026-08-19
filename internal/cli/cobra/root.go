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
побережья, включая анализ наблюдаемой береговой линии методом box-counting,
моделирование волновой эрозии и визуализацию на основе батиметрии. Учебная
Кох-подобная генерация вынесена в отдельную команду koch-demo.`,
	SilenceUsage: true,
}

var (
	quiet bool
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "тихий режим (подавить вывод)")
}
