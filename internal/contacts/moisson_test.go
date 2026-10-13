package contacts

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tahlasandale/HippopotamTUI/internal/cache"
	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
)

// fauxClient répond à la liste des boîtes et aux pages d'enveloppes sans jamais
// sortir de ce test.
type fauxClient struct {
	boites     []himalaya.Boite
	enveloppes map[string][]himalaya.Enveloppe
	erreurs    map[string]string
	demandes   []string
}

func (f *fauxClient) Comptes(context.Context) ([]himalaya.Compte, error) {
	return []himalaya.Compte{{Nom: "proton", Defaut: true}}, nil
}

func (f *fauxClient) Boites(_ context.Context, compte string) ([]himalaya.Boite, error) {
	f.demandes = append(f.demandes, "boites:"+compte)
	if msg := f.erreurs["boites"]; msg != "" {
		return nil, &himalaya.ErreurHimalaya{Msg: msg}
	}
	return f.boites, nil
}

func (f *fauxClient) Enveloppes(_ context.Context, compte, boite string, page, taille int) ([]himalaya.Enveloppe, error) {
	f.demandes = append(f.demandes, "env:"+compte+"/"+boite)
	if msg := f.erreurs["env"]; msg != "" {
		return nil, &himalaya.ErreurHimalaya{Msg: msg}
	}
	lot := f.enveloppes[boite]
	début := (page - 1) * taille
	if début >= len(lot) {
		return nil, nil
	}
	fin := début + taille
	if fin > len(lot) {
		fin = len(lot)
	}
	return lot[début:fin], nil
}

func (f *fauxClient) Rechercher(context.Context, string, string, string, int, int) ([]himalaya.Enveloppe, error) {
	return nil, nil
}
func (f *fauxClient) Message(context.Context, string, string, string) ([]byte, error) {
	return nil, nil
}
func (f *fauxClient) PoserDrapeau(context.Context, string, string, []string, himalaya.Drapeau) error {
	return nil
}
func (f *fauxClient) RetirerDrapeau(context.Context, string, string, []string, himalaya.Drapeau) error {
	return nil
}
func (f *fauxClient) Deplacer(context.Context, string, string, string, []string) error {
	return nil
}
func (f *fauxClient) Supprimer(context.Context, string, string, []string) error {
	return nil
}
func (f *fauxClient) Squelette(context.Context, himalaya.Origine, string, string) (himalaya.Brouillon, error) {
	return himalaya.Brouillon{}, nil
}
func (f *fauxClient) Enregistrer(context.Context, himalaya.Envoi, string, string) error {
	return nil
}
func (f *fauxClient) Envoyer(context.Context, himalaya.Envoi, string, string) error {
	return nil
}

func storeDeTest(t *testing.T) *cache.Store {
	t.Helper()
	store, err := cache.Ouvrir(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Fermer() })
	return store
}

// le moisson doit balayer les deux boîtes utiles et s'arrêter dès qu'une page
// revient vide, sans marteler le serveur.

func TestRécolterLesDeuxBoîtes(t *testing.T) {
	client := &fauxClient{
		boites: []himalaya.Boite{
			{ID: "Inbox", Nom: "Inbox", Role: "inbox"},
			{ID: "Sent", Nom: "Sent", Role: "sent"},
			{ID: "Archive", Nom: "Archive"},
		},
		enveloppes: map[string][]himalaya.Enveloppe{
			"Inbox": {
				{ID: "1", De: []himalaya.Adresse{{Nom: "Alice", Email: "alice@example.org"}}},
				{ID: "2", De: []himalaya.Adresse{{Nom: "Bob", Email: "bob@example.org"}}},
			},
			"Sent": {
				{ID: "3", Vers: []himalaya.Adresse{{Nom: "Carla", Email: "carla@example.org"}}},
			},
		},
	}
	store := storeDeTest(t)
	m := &Moissonneur{Client: client, Cache: store, ParPage: 2, Maximum: 100}

	trouvés, err := m.Récolter(context.Background(), "proton", nil, nil)
	if err != nil {
		t.Fatalf("Récolter : %v", err)
	}
	if len(trouvés) != 3 {
		t.Fatalf("trouvés = %+v, veut 3 contacts", trouvés)
	}
	for _, voulu := range []string{"env:proton/Inbox", "env:proton/Sent"} {
		présent := false
		for _, demande := range client.demandes {
			if demande == voulu {
				présent = true
			}
		}
		if !présent {
			t.Errorf("%s n'a jamais été demandée : %v", voulu, client.demandes)
		}
	}
	if slices(client.demandes, "env:proton/Archive") {
		t.Error("la boîte Archive n'a rien à faire dans un carnet")
	}
}

// le carnet doit être réutilisable sans repasser par le réseau.

func TestCarnetMisEnCache(t *testing.T) {
	client := &fauxClient{
		boites:     []himalaya.Boite{{ID: "Inbox", Role: "inbox"}, {ID: "Sent", Nom: "Sent"}},
		enveloppes: map[string][]himalaya.Enveloppe{"Inbox": {{ID: "1", De: []himalaya.Adresse{{Nom: "Alice", Email: "alice@example.org"}}}}},
	}
	store := storeDeTest(t)
	m := &Moissonneur{Client: client, Cache: store, ParPage: 10, Maximum: 100}

	if _, err := m.Récolter(context.Background(), "proton", nil, nil); err != nil {
		t.Fatal(err)
	}

	relus := m.Lire("proton")
	if len(relus) != 1 || relus[0].Email != "alice@example.org" {
		t.Errorf("Lire = %+v", relus)
	}
	if m.ÀRafraîchir("proton", 7) {
		t.Error("un carnet tout juste moissonné ne doit pas se rafraîchir")
	}
	if !m.ÀRafraîchir("gmail", 7) {
		t.Error("un compte jamais moissonné doit se rafraîchir")
	}
}

// une erreur de himalaya doit remonter telle quelle, pas disparaître dans un
// carnet vide qui donnerait l'impression que tout va bien.

func TestErreurDeMoisson(t *testing.T) {
	client := &fauxClient{
		boites:  []himalaya.Boite{{ID: "Inbox", Role: "inbox"}},
		erreurs: map[string]string{"env": "connexion IMAP refusée"},
	}
	m := &Moissonneur{Client: client, Cache: storeDeTest(t), ParPage: 10, Maximum: 100}

	_, err := m.Récolter(context.Background(), "proton", nil, nil)
	if err == nil {
		t.Fatal("Récolter aurait dû échouer")
	}
	if !strings.Contains(err.Error(), "connexion IMAP refusée") {
		t.Errorf("erreur = %q, veut le message de himalaya", err)
	}
}

// un compte sans boîte Sent ni Inbox ne peut pas produire de carnet : il faut le
// dire plutôt que de renvoyer une liste vide sans explication.

func TestCompteSansBoîteUtile(t *testing.T) {
	client := &fauxClient{boites: []himalaya.Boite{{ID: "Archive", Nom: "Archive"}}}
	m := &Moissonneur{Client: client, Cache: storeDeTest(t)}

	if _, err := m.Récolter(context.Background(), "proton", nil, nil); err == nil {
		t.Fatal("Récolter aurait dû échouer")
	}
}

// Gmail ne déclare pas de rôle « sent » : la boîte s'appelle Sent, et c'est
// elle qu'il faut balayer.

func TestBoîtesSansRôle(t *testing.T) {
	boîtes := []himalaya.Boite{
		{ID: "[Gmail]/Messages envoyés", Nom: "Sent"},
		{ID: "[Gmail]/Boîte de réception", Nom: "INBOX"},
	}
	sent, inbox := BoîtesÀBalayer(boîtes)

	if sent != "[Gmail]/Messages envoyés" {
		t.Errorf("sent = %q", sent)
	}
	if inbox != "[Gmail]/Boîte de réception" {
		t.Errorf("inbox = %q", inbox)
	}
}

func slices(liste []string, voulu string) bool {
	for _, item := range liste {
		if item == voulu {
			return true
		}
	}
	return false
}
