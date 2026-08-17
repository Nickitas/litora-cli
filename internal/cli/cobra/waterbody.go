package cobra

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/waterbody"

	"github.com/spf13/cobra"
)

var waterbodyOutput string

// waterbodyCmd объединяет команды просмотра доступных в Lito водоёмов РФ.
var waterbodyCmd = &cobra.Command{
	Use:   "waterbody",
	Short: "Каталог водоёмов Российской Федерации и применимость моделей",
}

var waterbodyListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать доступные водоёмы Российской Федерации",
	Long: `Показывает только водоёмы и участки водоёмов Российской Федерации.
Статус «требуется другая модель» означает, что волновая CERC-модель не будет
использована для такого объекта. Каталог также сохраняется в output/waterbody/.`,
	RunE: runWaterbodyList,
}

func init() {
	rootCmd.AddCommand(waterbodyCmd)
	waterbodyCmd.AddCommand(waterbodyListCmd)
	waterbodyListCmd.Flags().StringVar(&waterbodyOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")
}

func runWaterbodyList(cmd *cobra.Command, args []string) error {
	bodies := waterbody.List()
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Идентификатор\tВодоём\tРегион РФ\tТип\tМодель\tДоступность")
	fmt.Fprintln(table, "-------------\t-------\t---------\t---\t------\t-----------")
	for _, body := range bodies {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n", body.ID, body.Name, body.Region, body.Type, body.Model, body.Availability)
	}
	table.Flush()

	output := cli.NewOutputPathManager(waterbodyOutput)
	if err := output.EnsureDirectories(); err != nil {
		return fmt.Errorf("подготовка каталога вывода: %w", err)
	}
	if err := os.MkdirAll(output.WaterbodyDir(), 0o755); err != nil {
		return fmt.Errorf("создание каталога водоёмов: %w", err)
	}
	data, err := json.MarshalIndent(bodies, "", "  ")
	if err != nil {
		return fmt.Errorf("сериализация каталога: %w", err)
	}
	path := output.WaterbodyPath("catalog.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение каталога: %w", err)
	}
	fmt.Printf("\n✓ Каталог сохранён: %s\n", path)
	return nil
}
