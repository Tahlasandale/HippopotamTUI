package cli

import (
	"os"
	"path/filepath"

	"github.com/Tahlasandale/HippopotamTUI/internal/demo"
	"github.com/Tahlasandale/HippopotamTUI/internal/ui"
	"github.com/spf13/cobra"
)

// captureCmd écrit les écrans du mode démonstration dans un fichier, pour la
// documentation. La doc montre ainsi exactement ce que le programme affiche,
// sans capture d'écran datée.
func captureCmd() *cobra.Command {
	var (
		destination string
		largeur     int
		hauteur     int
		surTerminal bool
	)

	cmd := &cobra.Command{
		Use:   "capture",
		Short: "écrit les écrans de démonstration dans docs/captures/",
		Long: `Rend les écrans principaux à partir du jeu de démonstration et les écrit
dans docs/captures/. Les couleurs sont retirées : le résultat reste lisible
dans n'importe quel fichier texte.

À relancer quand l'affichage change, plutôt que d'entretenir des captures
d'écran qui vieillissent mal.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if destination == "" {
				destination = filepath.Join("docs", "captures")
			}
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}

			captures := ui.Capturer(demo.Nouveau(), largeur, hauteur)
			for _, c := range captures {
				texte := ui.EnTeur(c.Intitulé, c.Texte)
				chemin := filepath.Join(destination, c.Nom+".txt")
				if err := os.WriteFile(chemin, []byte(texte), 0o644); err != nil {
					return err
				}
				if surTerminal {
					cmd.Print(texte)
					continue
				}
				cmd.Printf("écrit %s\n", chemin)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&destination, "dans", "", "répertoire de destination")
	cmd.Flags().IntVar(&largeur, "largeur", 96, "largeur du terminal simulé")
	cmd.Flags().IntVar(&hauteur, "hauteur", 30, "hauteur du terminal simulé")
	cmd.Flags().BoolVar(&surTerminal, "stdout", false, "écrit sur la sortie standard au lieu des fichiers")
	return cmd
}
