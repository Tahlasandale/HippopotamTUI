package config

import (
	"os"
	"path/filepath"
	"testing"
)

// cycle 1 : les valeurs par défaut doivent suffire à faire tourner le TUI
// sans aucun fichier de configuration présent.

func TestDefaultConfig(t *testing.T) {
	c := Default()

	if c.Himalaya != "himalaya" {
		t.Errorf("Himalaya = %q, veut %q", c.Himalaya, "himalaya")
	}
	if c.Editor != "" {
		t.Errorf("Editor = %q, veut vide (détection automatique)", c.Editor)
	}
	if c.PageSize != 25 {
		t.Errorf("PageSize = %d, veut 25", c.PageSize)
	}
	if c.RefreshMinutes != 5 {
		t.Errorf("RefreshMinutes = %d, veut 5", c.RefreshMinutes)
	}
	if c.ContactsRefreshDays != 7 {
		t.Errorf("ContactsRefreshDays = %d, veut 7", c.ContactsRefreshDays)
	}
	if c.Keys != nil {
		t.Errorf("Keys = %v, veut nil tant qu'aucune surcharge n'est demandée", c.Keys)
	}
}

// un fichier absent n'est pas une erreur : tout part des défauts.

func TestLoadSansFichier(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	c, err := Load(filepath.Join(dir, "absent.toml"))
	if err != nil {
		t.Fatalf("Load : %v", err)
	}
	if c.PageSize != 25 {
		t.Errorf("PageSize = %d, veut le défaut 25", c.PageSize)
	}
}

// les valeurs absentes d'un fichier partiel doivent retomber sur le défaut.

func TestLoadFichierPartiel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	contenu := "page_size = 50\n"
	if err := os.WriteFile(path, []byte(contenu), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load : %v", err)
	}
	if c.PageSize != 50 {
		t.Errorf("PageSize = %d, veut 50", c.PageSize)
	}
	if c.Himalaya != "himalaya" {
		t.Errorf("Himalaya = %q, veut le défaut conservé", c.Himalaya)
	}
}

// les chemins suivent l'arbitrage XDG, avec repli sur $HOME quand la variable
// d'environnement est absente.

func TestCheminsXDG(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))

	p := Paths()

	if got, want := p.Config, filepath.Join(base, "config", "hippo", "config.toml"); got != want {
		t.Errorf("Config = %q, veut %q", got, want)
	}
	if got, want := p.Cache, filepath.Join(base, "state", "hippo", "cache.db"); got != want {
		t.Errorf("Cache = %q, veut %q", got, want)
	}
	if got, want := p.Log, filepath.Join(base, "state", "hippo", "hippo.log"); got != want {
		t.Errorf("Log = %q, veut %q", got, want)
	}
	if !filepath.IsAbs(p.State) {
		t.Errorf("State = %q, veut un chemin absolu", p.State)
	}
}

// sans XDG_STATE_HOME, on retombe sur ~/.local/state, comme le veut la spec.

func TestCheminsRepliSurHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")

	p := Paths()

	if got, want := p.Config, filepath.Join(home, ".config", "hippo", "config.toml"); got != want {
		t.Errorf("Config = %q, veut %q", got, want)
	}
	if got, want := p.State, filepath.Join(home, ".local", "state", "hippo"); got != want {
		t.Errorf("State = %q, veut %q", got, want)
	}
}

// le répertoire d'état doit pouvoir être créé au premier lancement.

func TestEnsureStateDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))

	p := Paths()
	if err := EnsureStateDir(p); err != nil {
		t.Fatalf("EnsureStateDir : %v", err)
	}
	info, err := os.Stat(p.State)
	if err != nil {
		t.Fatalf("le répertoire d'état n'existe pas après création : %v", err)
	}
	if !info.IsDir() {
		t.Error("le chemin d'état n'est pas un répertoire")
	}
}

// un fichier illisible doit être signalé, pas silencieusement ignoré.

func TestLoadFichierInvalide(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("page_size = \"beaucoup\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load aurait dû échouer sur un TOML invalide")
	}
}

// cycle 2 : la surcharge de touches.

func TestSurchargeDeTouche(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	contenu := "[keys]\nquitter = \"x\"\n"
	if err := os.WriteFile(path, []byte(contenu), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load : %v", err)
	}
	if got := c.Touche("quitter"); got != "x" {
		t.Errorf("Touche(quitter) = %q, veut %q", got, "x")
	}
}

// une action jamais surchargée tombe sur le nom de l'action, ce qui laisse un
// raccourci lisible dans la configuration.

func TestToucheParDéfaut(t *testing.T) {
	c := Default()
	if got := c.Touche("rafraichir"); got != "rafraichir" {
		t.Errorf("Touche(rafraichir) = %q, veut le nom de l'action", got)
	}
	if got := c.Touche("inconnu"); got != "inconnu" {
		t.Errorf("Touche(inconnu) = %q, veut le nom de l'action", got)
	}
}
