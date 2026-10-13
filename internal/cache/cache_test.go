package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
)

// écrireFichier écrase un fichier de test, pour simuler un cache corrompu.
func écrireFichier(chemin string, contenu []byte) error {
	return os.WriteFile(chemin, contenu, 0o600)
}

func ouvrir(t *testing.T) *Store {
	t.Helper()
	store, err := Ouvrir(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("Ouvrir : %v", err)
	}
	t.Cleanup(func() { store.Fermer() })
	return store
}

func envelopes() []himalaya.Enveloppe {
	pièceJointe := true
	return []himalaya.Enveloppe{
		{ID: "1", Sujet: "Bonjour", De: []himalaya.Adresse{{Nom: "Alice", Email: "alice@example.org"}}, PieceJointeRaw: &pièceJointe},
		{ID: "2", Sujet: "Réunion", De: []himalaya.Adresse{{Email: "bob@example.org"}}},
	}
}

// le cache survit à la fermeture du programme : c'est tout son intérêt.

func TestPersistance(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "cache.db")
	store, err := Ouvrir(chemin)
	if err != nil {
		t.Fatal(err)
	}

	instant := time.Date(2026, 10, 13, 22, 0, 0, 0, time.UTC)
	if err := store.EnregistrerEnveloppes("proton", "Inbox", 1, envelopes(), instant); err != nil {
		t.Fatalf("EnregistrerEnveloppes : %v", err)
	}
	store.Fermer()

	store, err = Ouvrir(chemin)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Fermer()

	trouvées, quand, ok := store.Enveloppes("proton", "Inbox", 1)
	if !ok {
		t.Fatal("la page d'enveloppes a disparu après réouverture")
	}
	if len(trouvées) != 2 || trouvées[0].Sujet != "Bonjour" {
		t.Errorf("Enveloppes = %+v", trouvées)
	}
	if !quand.Equal(instant) {
		t.Errorf("quand = %v, veut %v", quand, instant)
	}
	// le drapeau de pièce jointe est un *bool en JSON : il doit survivre au
	// passage par le disque.
	if !trouvées[0].PiecesJointes {
		t.Error("l'indicateur de pièce jointe a été perdu en sérialisant")
	}
}

// deux pages d'une même boîte ne doivent pas se confondre, ni deux boîtes d'un
// même compte, ni deux comptes.

func TestClésDistinctes(t *testing.T) {
	store := ouvrir(t)
	instant := time.Now()

	store.EnregistrerEnveloppes("proton", "Inbox", 1, envelopes(), instant)
	store.EnregistrerEnveloppes("proton", "Inbox", 2, []himalaya.Enveloppe{{ID: "9", Sujet: "Page deux"}}, instant)
	store.EnregistrerEnveloppes("proton", "Archive", 1, []himalaya.Enveloppe{{ID: "8", Sujet: "Archivée"}}, instant)
	store.EnregistrerEnveloppes("gmail", "Inbox", 1, []himalaya.Enveloppe{{ID: "7", Sujet: "Gmail"}}, instant)

	if _, _, ok := store.Enveloppes("proton", "Inbox", 2); !ok {
		t.Error("la page 2 d'Inbox est introuvable")
	}
	if env, _, _ := store.Enveloppes("proton", "Archive", 1); env[0].Sujet != "Archivée" {
		t.Error("la boîte Archive s'est confondue avec Inbox")
	}
	if env, _, _ := store.Enveloppes("gmail", "Inbox", 1); env[0].Sujet != "Gmail" {
		t.Error("les comptes se sont confondus")
	}
	if _, _, ok := store.Enveloppes("proton", "Inbox", 3); ok {
		t.Error("une page jamais enregistrée ne doit pas exister")
	}
}

// un rafraîchissement sans nouveau contenu doit quand même dater la page, sinon
// le TUI rafraîchit indéfiniment.

func TestToucherSansNouveauContenu(t *testing.T) {
	store := ouvrir(t)

	store.EnregistrerEnveloppes("proton", "Inbox", 1, envelopes(), time.Now().Add(-time.Hour))
	avant, _, _ := store.Enveloppes("proton", "Inbox", 1)

	maintenant := time.Now()
	if err := store.ToucherEnveloppes("proton", "Inbox", maintenant); err != nil {
		t.Fatal(err)
	}

	_, quand, _ := store.Enveloppes("proton", "Inbox", 1)
	if !quand.After(maintenant.Add(-time.Minute)) {
		t.Errorf("quand = %v, veut une date fraîche", quand)
	}
	trouvées, _, _ := store.Enveloppes("proton", "Inbox", 1)
	if len(trouvées) != len(avant) {
		t.Error("toucher une page ne doit pas la vider")
	}
}

// une page absente doit se signaler par ok=false, jamais par une tranche vide
// qu'on confondrait avec une boîte réellement vide.

func TestPageAbsente(t *testing.T) {
	store := ouvrir(t)

	env, quand, ok := store.Enveloppes("proton", "Inbox", 1)
	if ok || len(env) != 0 || !quand.IsZero() {
		t.Errorf("Enveloppes = %v, %v, %v ; veut rien, zéro, false", env, quand, ok)
	}
}

func TestContacts(t *testing.T) {
	store := ouvrir(t)
	instant := time.Now()

	liste := []Contact{
		{Nom: "Alice Martin", Email: "alice@example.org"},
		{Nom: "Bob", Email: "bob@example.org"},
	}
	if err := store.EnregistrerContacts("proton", liste, instant); err != nil {
		t.Fatalf("EnregistrerContacts : %v", err)
	}

	trouvés, quand, ok := store.Contacts("proton")
	if !ok || len(trouvés) != 2 {
		t.Fatalf("Contacts = %v, %v, %v", trouvés, quand, ok)
	}
	if trouvés[0].Email != "alice@example.org" {
		t.Errorf("Contact = %+v", trouvés[0])
	}

	// les contacts sont triés : c'est ce qui rend la frappe prévisible.
	if trouvés[0].Nom > trouvés[1].Nom {
		t.Error("les contacts ne sont pas triés par nom")
	}

	if _, _, ok := store.Contacts("gmail"); ok {
		t.Error("un compte sans moisson ne doit rien retourner")
	}
}

func TestPurger(t *testing.T) {
	store := ouvrir(t)
	instant := time.Now()
	store.EnregistrerEnveloppes("proton", "Inbox", 1, envelopes(), instant)
	store.EnregistrerContacts("proton", []Contact{{Email: "a@b.c"}}, instant)

	if err := store.Purger(); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := store.Enveloppes("proton", "Inbox", 1); ok {
		t.Error("le cache d'enveloppes aurait dû être vidé")
	}
	if _, _, ok := store.Contacts("proton"); ok {
		t.Error("l'index de contacts aurait dû être vidé")
	}
}

// un cache corrompu ne doit pas empêcher le TUI de démarrer : on le signale et on
// repart d'un fichier neuf plutôt que de refuser de s'ouvrir.

func TestCacheCorrompu(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "cache.db")
	store, err := Ouvrir(chemin)
	if err != nil {
		t.Fatal(err)
	}
	store.Fermer()

	corrompu := []byte("ceci n'est pas une base bbolt")
	if err := écrireFichier(chemin, corrompu); err != nil {
		t.Fatal(err)
	}

	store, err = Ouvrir(chemin)
	if err != nil {
		t.Fatalf("un cache corrompu ne doit pas faire échouer l'ouverture : %v", err)
	}
	defer store.Fermer()
}
