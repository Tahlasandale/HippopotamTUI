package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tahlasandale/HippopotamTUI/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// manCmd génère la page de manuel. La documentation est une exigence du projet :
// autant la produire depuis les commentaires des commandes plutôt que de la
// garder en double, où elle finit forcément par mentir.
func manCmd() *cobra.Command {
	var répertoire string

	cmd := &cobra.Command{
		Use:    "man",
		Short:  "génère la page de manuel",
		Long:   `Génère les pages de manuel à partir de l'aide des commandes.`,
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if répertoire == "" {
				répertoire = filepath.Join("docs", "man")
			}
			if err := os.MkdirAll(répertoire, 0o755); err != nil {
				return err
			}
			entête := &doc.GenManHeader{
				Title:   "HIPPO",
				Section: "1",
				Source:  "HippopotamTUI " + Version,
				Manual:  "HippopotamTUI Manual",
			}
			if err := doc.GenManTree(racine(), entête, répertoire); err != nil {
				return fmt.Errorf("génération des pages de manuel : %w", err)
			}
			cmd.Printf("pages écrites dans %s\n", répertoire)
			return nil
		},
	}

	cmd.Flags().StringVar(&répertoire, "dans", "", "répertoire de destination")
	return cmd
}

// exempleConfig est la source de vérité du fichier d'exemple.
var _ = config.Default
