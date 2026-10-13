package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/cache"
	"github.com/Tahlasandale/HippopotamTUI/internal/config"
	"github.com/Tahlasandale/HippopotamTUI/internal/contacts"
	"github.com/Tahlasandale/HippopotamTUI/internal/demo"
	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	"github.com/spf13/cobra"
)

// clientEtCache fabrique le client himalaya et ouvre le cache local.
//
// Le cache est ouvert même en démonstration : le bandeau d'accueil se décide sur ce
// que le cache contient déjà, et un fichier vide ne coûte rien.
func clientEtCache(cfg config.Config, chemins config.Emplacements, démo bool) (himalaya.Client, *cache.Store) {
	store, err := cache.Ouvrir(chemins.Cache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hippo : cache indisponible, le TUI fonctionnera sans cache (%v)\n", err)
		store = nil
	}

	if démo {
		return demo.Nouveau(), store
	}
	return &himalaya.CLI{Bin: cfg.Himalaya, Délai: time.Duration(cfg.Délai) * time.Second}, store
}

func versionCmd(_ *options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "affiche la version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Printf("hippo %s\n", Version)
			return nil
		},
	}
}

func configCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "la configuration de hippo",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "affiche l'emplacement de chaque fichier",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, chemins, err := chargerConfig(*opts)
			if err != nil {
				return err
			}
			cmd.Printf("configuration  %s\n", chemins.Config)
			cmd.Printf("cache          %s\n", chemins.Cache)
			cmd.Printf("journal        %s\n", chemins.Log)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "afficher",
		Short: "affiche la configuration en vigueur",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := chargerConfig(*opts)
			if err != nil {
				return err
			}
			cmd.Println(config.Expliquer(cfg))
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "écrit un fichier de configuration commenté",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, chemins, err := chargerConfig(*opts)
			if err != nil {
				return err
			}
			if _, err := os.Stat(chemins.Config); err == nil {
				return fmt.Errorf("%s existe déjà : rien n'a été écrit", chemins.Config)
			}
			if err := os.MkdirAll(filepath.Dir(chemins.Config), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(chemins.Config, []byte(config.Exemple(cfg)), 0o600); err != nil {
				return err
			}
			cmd.Printf("écrit %s\n", chemins.Config)
			return nil
		},
	})

	return cmd
}

func contactsCmd(opts *options) *cobra.Command {
	var forcer bool
	var maximum int

	cmd := &cobra.Command{
		Use:   "contacts",
		Short: "reconstruit le carnet d'adresses",
		Long: `Le carnet est reconstruit en balayant les boîtes Envoyés et Réception de
chaque compte. Rien ne quitte la machine : les adresses sont enregistrées dans
le cache local, et himalaya n'a aucun carnet d'adresses à consulter.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, chemins, err := chargerConfig(*opts)
			if err != nil {
				return err
			}
			if err := config.EnsureStateDir(chemins); err != nil {
				return err
			}

			client, store := clientEtCache(cfg, chemins, false)
			if store == nil {
				return fmt.Errorf("cache indisponible, rien à faire")
			}
			defer store.Fermer()

			comptes, err := client.Comptes(cmd.Context())
			if err != nil {
				return fmt.Errorf("lecture des comptes : %w", err)
			}
			if len(comptes) == 0 {
				return fmt.Errorf("aucun compte dans la configuration de himalaya")
			}

			moissonneur := &contacts.Moissonneur{Client: client, Cache: store, ParPage: 500, Maximum: maximum}
			for _, compte := range comptes {
				if !forcer && !moissonneur.ÀRafraîchir(compte.Nom, 0) {
					cmd.Printf("%s : carnet à jour, rien à faire (--force pour le refaire)\n", compte.Nom)
					continue
				}
				cmd.Printf("moisson de %s…\n", compte.Nom)
				trouvés, err := moissonneur.Récolter(cmd.Context(), compte.Nom, nil, func(faits, attendus int) {
					cmd.Printf("  %d sur %d\n", faits, attendus)
				})
				if err != nil {
					cmd.PrintErrf("  %s : %v\n", compte.Nom, err)
					continue
				}
				cmd.Printf("  %d contacts\n", len(trouvés))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&forcer, "force", false, "moissonne même si le carnet est récent")
	cmd.Flags().IntVar(&maximum, "maximum", 2000, "nombre maximal de messages parcourus par compte")
	return cmd
}

func cacheCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "le cache local",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "purge",
		Short: "vide le cache",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, chemins, err := chargerConfig(*opts)
			if err != nil {
				return err
			}
			_, store := clientEtCache(cfg, chemins, false)
			if store == nil {
				return fmt.Errorf("cache indisponible")
			}
			defer store.Fermer()

			if err := store.Purger(); err != nil {
				return err
			}
			cmd.Printf("cache vidé : %s\n", chemins.Cache)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "statistiques",
		Short: "montre ce que contient le cache",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, chemins, err := chargerConfig(*opts)
			if err != nil {
				return err
			}
			_, store := clientEtCache(cfg, chemins, false)
			if store == nil {
				return fmt.Errorf("cache indisponible")
			}
			defer store.Fermer()

			comptes, err := cfgComptes(clientHimalaya(cfg))
			if err != nil {
				return err
			}
			for _, compte := range comptes {
				liste, quand, ok := store.Contacts(compte.Nom)
				if ok {
					cmd.Printf("  %-10s %5d contacts, moissonnés le %s\n",
						compte.Nom, len(liste), quand.Format("02/01/2006 15:04"))
				} else {
					cmd.Printf("  %-10s jamais moissonné\n", compte.Nom)
				}
			}
			return nil
		},
	})

	return cmd
}

func clientHimalaya(cfg config.Config) himalaya.Client {
	return &himalaya.CLI{Bin: cfg.Himalaya, Délai: time.Duration(cfg.Délai) * time.Second}
}

func cfgComptes(client himalaya.Client) ([]himalaya.Compte, error) {
	return client.Comptes(context.Background())
}
