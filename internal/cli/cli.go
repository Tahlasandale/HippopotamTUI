// Package cli assemble la ligne de commande de hippo.
//
// La surface est volontairement étroite : le TUI quand on ne dit rien, et de
// quoi l'entretenir — la configuration, le cache, les contacts, une page de
// manuel.
package cli

import (
	"fmt"
	"os"

	"github.com/Tahlasandale/HippopotamTUI/internal/config"
	"github.com/Tahlasandale/HippopotamTUI/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// Version est inscrite à la compilation.
var Version = "0.1.0"

type options struct {
	configPath string
	démo       bool
	version    bool
}

// Execute construit la commande racine et l'exécute.
func Execute() error {
	return racine().Execute()
}

func racine() *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "hippo",
		Short: "Un client de messagerie en ligne de commande, au-dessus de himalaya",
		Long: `hippo est un TUI léger pour lire et écrire ses courriels.

Il s'appuie entièrement sur la CLI de himalaya : il ne lit jamais le fichier de
configuration de himalaya, qui contient vos mots de passe, et ne lui demande que
des listes et des actions.

Lancement :
  hippo                ouvre la boîte de réception
  hippo --demo         ouvre une démonstration, sans aucun compte

Entretien :
  hippo config path    où sont la configuration, le cache, les journaux
  hippo contacts       reconstruit le carnet d'adresses
  hippo cache purge    vide le cache local`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return lancer(opts)
		},
	}

	cmd.Flags().StringVar(&opts.configPath, "config", "", "chemin du fichier de configuration de hippo")
	cmd.Flags().BoolVar(&opts.démo, "demo", false, "démonstration hors ligne, sans toucher au réseau")
	cmd.Flags().BoolVar(&opts.version, "version", false, "affiche la version et sort")

	cmd.AddCommand(versionCmd(&opts))
	cmd.AddCommand(configCmd(&opts))
	cmd.AddCommand(contactsCmd(&opts))
	cmd.AddCommand(cacheCmd(&opts))
	cmd.AddCommand(manCmd())
	cmd.AddCommand(captureCmd())

	return cmd
}

// chargerConfig lit la configuration demandée, ou celle par défaut.
func chargerConfig(opts options) (config.Config, config.Emplacements, error) {
	chemins := config.Paths()
	chemin := opts.configPath
	if chemin == "" {
		chemin = chemins.Config
	}

	cfg, err := config.Load(chemin)
	if err != nil {
		return cfg, chemins, err
	}
	if opts.configPath != "" {
		chemins.Config = opts.configPath
	}
	return cfg, chemins, nil
}

// lancer démarre le TUI.
func lancer(opts options) error {
	cfg, chemins, err := chargerConfig(opts)
	if err != nil {
		return err
	}

	if !opts.démo {
		if err := config.EnsureStateDir(chemins); err != nil {
			return err
		}
	}

	client, store := clientEtCache(cfg, chemins, opts.démo)
	if store != nil {
		defer store.Fermer()
	}

	programme := tea.NewProgram(
		ui.Nouveau(client, store, cfg, opts.démo),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err = programme.Run()
	if err != nil && opts.démo && !isTerminalAbsente(err) {
		// Hors terminal, le mode démonstration doit rester utilisable pour les
		// captures d'écran automatisées.
		fmt.Fprintln(os.Stderr, "hippo :", err)
	}
	return nil
}

func isTerminalAbsente(err error) bool {
	return err != nil && (os.Getenv("TERM") == "" || os.Getenv("TERM") == "dumb")
}
