// Package config lit la configuration du TUI et résout les chemins de stockage.
//
// Le TUI ne lit jamais la configuration de himalaya : celle-ci contient des
// mots de passe en clair. Tout passe par la CLI. Ce fichier ne contient que des
// préférences d'affichage et d'ergonomie, et chaque clé a un défaut qui rend le
// programme utilisable sans configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config regroupe les préférences du TUI.
type Config struct {
	// Himalaya est le chemin du binaire himalaya. Vidé, il vaut "himalaya",
	// que l'oncherche dans le PATH.
	Himalaya string `toml:"himalaya"`

	// Editor est la commande utilisée pour rédiger le corps d'un message.
	// Vide, elle est déduite de $EDITOR puis de micro, nvim, vim et vi.
	Editor string `toml:"editor"`

	// PageSize est le nombre d'enveloppes demandées à himalaya par page.
	PageSize int `toml:"page_size"`

	// RefreshMinutes est le délai au-delà duquel la liste affichée est
	// rechargée depuis le serveur sans qu'on le demande.
	RefreshMinutes int `toml:"refresh_minutes"`

	// ContactsRefreshDays est l'âge maximal du moisson des contacts avant
	// qu'il soit relance au démarrage.
	ContactsRefreshDays int `toml:"contacts_refresh_days"`

	// Keys surcharge ponctuellement une touche. Clé : nom de l'action,
	// valeur : touche ou séquence ("ctrl-s", "f2"...).
	Keys map[string]string `toml:"keys"`
}

// Default rend une configuration entièrement fonctionnelle, sans que l'utilisateur
// ait jamais eu à écrire le moindre fichier.
func Default() Config {
	return Config{
		Himalaya:            "himalaya",
		Editor:              "",
		PageSize:            25,
		RefreshMinutes:      5,
		ContactsRefreshDays: 7,
	}
}

// Touche renvoie la touche associée à une action, ou le nom de l'action si
// celle-ci n'a pas été surchargée.
func (c Config) Touche(action string) string {
	if touche, ok := c.Keys[action]; ok && touche != "" {
		return touche
	}
	return action
}

// Emplacements regroupe les emplacements de fichiers manipulés par le programme.
type Emplacements struct {
	Config string
	State  string
	Cache  string
	Log    string
}

// Paths calcule les chemins en suivant l'arbitrage XDG : la configuration dans
// XDG_CONFIG_HOME, le cache et les journaux dans XDG_STATE_HOME, avec le repli
// standard sur $HOME quand la variable est absente.
func Paths() Emplacements {
	state := filepath.Join(repertoire("XDG_STATE_HOME"), "hippo")
	return Emplacements{
		Config: filepath.Join(repertoire("XDG_CONFIG_HOME"), "hippo", "config.toml"),
		State:  state,
		Cache:  filepath.Join(state, "cache.db"),
		Log:    filepath.Join(state, "hippo.log"),
	}
}

func repertoire(varEnv string) string {
	if v := os.Getenv(varEnv); filepath.IsAbs(v) {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	if varEnv == "XDG_CONFIG_HOME" {
		return filepath.Join(home, ".config")
	}
	return filepath.Join(home, ".local", "state")
}

// EnsureStateDir crée le répertoire d'état s'il n'existe pas encore.
func EnsureStateDir(p Emplacements) error {
	if err := os.MkdirAll(p.State, 0o700); err != nil {
		return fmt.Errorf("création du répertoire d'état %s : %w", p.State, err)
	}
	return nil
}

// DefaultPath est le chemin de configuration retenu quand aucun n'est fourni en
// ligne de commande.
func DefaultPath() string { return Paths().Config }

// Load part des défauts puis applique le fichier donné. Un fichier absent est
// normal ; un fichier illisible ne l'est pas.
func Load(path string) (Config, error) {
	c := Default()

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, fmt.Errorf("lecture de la configuration %s : %w", path, err)
	}

	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Default(), fmt.Errorf("analyse de la configuration %s : %w", path, err)
	}

	c.normalise()
	return c, nil
}

func (c *Config) normalise() {
	if c.Himalaya == "" {
		c.Himalaya = "himalaya"
	}
	if c.PageSize <= 0 {
		c.PageSize = 25
	}
	if c.RefreshMinutes <= 0 {
		c.RefreshMinutes = 5
	}
	if c.ContactsRefreshDays <= 0 {
		c.ContactsRefreshDays = 7
	}
}