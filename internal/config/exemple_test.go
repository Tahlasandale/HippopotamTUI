package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// Le fichier d'exemple doit être un fichier de configuration valide : s'il ne
// l'est pas, la première chose que fera un utilisateur de « hippo config init »
// sera de lire un message d'erreur.
func TestExempleEstValide(t *testing.T) {
	dir := t.TempDir()
	chemin := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(chemin, []byte(Exemple(Default())), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load(chemin)
	if err != nil {
		t.Fatalf("le fichier d'exemple n'est pas chargeable : %v", err)
	}
	if c.PageSize != Default().PageSize {
		t.Errorf("page_size = %d, veut %d", c.PageSize, Default().PageSize)
	}
}

// et il doit rester lisible : une ligne de commentaire par clé, pour que la
// configuration s'explique sans ouvrir le code.
func TestExempleEstCommenté(t *testing.T) {
	exemple := Exemple(Default())

	for _, clé := range []string{"himalaya", "editor", "page_size", "refresh_minutes",
		"contacts_refresh_days", "timeout_seconds", "[keys]"} {
		if !strings.Contains(exemple, clé) {
			t.Errorf("l'exemple ne parle pas de %q", clé)
		}
	}

	commentaires := 0
	for _, ligne := range strings.Split(exemple, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ligne), "#") {
			commentaires++
		}
	}
	if commentaires < 10 {
		t.Errorf("%d lignes de commentaire, un exemple doit être commenté", commentaires)
	}

	// et il doit dire pourquoi hippo ne lit pas la configuration de himalaya :
	// c'est la question que tout le monde se pose en ouvrant le fichier.
	if !strings.Contains(exemple, "mots de passe") {
		t.Error("l'exemple doit justifier le choix de ne pas lire la config de himalaya")
	}
}

// « hippo config afficher » doit montrer la valeur en cours et le défaut de
// chaque clé : c'est la seule façon de comprendre d'où vient un comportement.
func TestExpliquer(t *testing.T) {
	c := Default()
	c.PageSize = 50
	c.Editor = "micro"

	texte := Expliquer(c)
	for _, attendu := range []string{"page_size", "50", "25", "micro", "détection automatique",
		"timeout_seconds", "90"} {
		if !strings.Contains(texte, attendu) {
			t.Errorf("l'explication ne montre pas %q :\n%s", attendu, texte)
		}
	}
}

// des touches surchargées sont énumérées, et dans un ordre stable.
func TestExpliquerLesTouches(t *testing.T) {
	c := Default()
	c.Keys = map[string]string{"rafraichir": "R", "quitter": "Q"}

	texte := Expliquer(c)
	première := strings.Index(texte, "quitter")
	seconde := strings.Index(texte, "rafraichir")

	if première < 0 || seconde < 0 {
		t.Fatalf("les touches ne sont pas listées :\n%s", texte)
	}
	if première > seconde {
		t.Error("les touches devraient être triées pour un affichage stable")
	}
}

// un fichier qui contient des clés inconnues ne doit pas faire échouer le
// chargement : le TUI peut tourner avec une configuration écrite pour une
// version plus récente.
func TestClésInconnuesTolérées(t *testing.T) {
	dir := t.TempDir()
	chemin := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(chemin, []byte("cle_inconnue = 42\npage_size = 10\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var brut map[string]any
	if _, err := toml.DecodeFile(chemin, &brut); err != nil {
		t.Fatal(err)
	}

	c, err := Load(chemin)
	if err != nil {
		t.Fatalf("Load : %v", err)
	}
	if c.PageSize != 10 {
		t.Errorf("page_size = %d, veut 10", c.PageSize)
	}
}

// le répertoire d'état doit être créé avec des permissions reasonableness : il
// contient un carnet d'adresses.
func TestPermissionsDuRépertoire(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))

	p := Paths()
	if err := EnsureStateDir(p); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(p.State)
	if err != nil {
		t.Fatal(err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o700 {
		t.Errorf("permissions = %o, veut 700", permissions)
	}
}
