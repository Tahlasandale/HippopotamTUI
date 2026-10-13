package contacts

import (
	"reflect"
	"testing"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
)

// le moisson prend les trois champs d'adresses que contient une enveloppe.

func TestRécolterLesTroisChamps(t *testing.T) {
	envs := []himalaya.Enveloppe{{
		ID:   "1",
		De:   []himalaya.Adresse{{Nom: "Alice", Email: "alice@example.org"}},
		Vers: []himalaya.Adresse{{Nom: "Bob", Email: "bob@example.org"}},
		Cc:   []himalaya.Adresse{{Email: "carla@example.org"}},
	}}

	trouvés := Récolter(envs, []string{"moi@example.org"})
	if len(trouvés) != 3 {
		t.Fatalf("len = %d, veut 3 : %+v", len(trouvés), trouvés)
	}
}

// la même personne écrite différemment ne doit apparaître qu'une fois, la
// meilleure version gagnante étant celle qui porte un nom.

func TestDédoublonnerEnFavorisantLeNom(t *testing.T) {
	envs := []himalaya.Enveloppe{
		{ID: "1", De: []himalaya.Adresse{{Email: "alice@example.org"}}},
		{ID: "2", De: []himalaya.Adresse{{Nom: "Alice Martin", Email: "alice@example.org"}}},
		{ID: "3", Vers: []himalaya.Adresse{{Nom: "alice martin", Email: "ALICE@Example.org"}}},
	}

	trouvés := Récolter(envs, nil)
	if len(trouvés) != 1 {
		t.Fatalf("len = %d, veut 1 : %+v", len(trouvés), trouvés)
	}
	if trouvés[0].Nom != "Alice Martin" {
		t.Errorf("Nom = %q, veut Alice Martin", trouvés[0].Nom)
	}
}

// ses propres adresses n'ont rien à faire dans l'autocomplétion.

func TestExclureSesPropresAdresses(t *testing.T) {
	envs := []himalaya.Enveloppe{{
		ID:   "1",
		De:   []himalaya.Adresse{{Nom: "Moi", Email: "orphin@protonmail.com"}},
		Vers: []himalaya.Adresse{{Nom: "Alice", Email: "alice@example.org"}},
	}}

	trouvés := Récolter(envs, []string{"Orphin@Protonmail.com"})
	if len(trouvés) != 1 || trouvés[0].Email != "alice@example.org" {
		t.Errorf("trouvés = %+v", trouvés)
	}
}

// une adresse vide ou manifestement fausse ne doit pas polluer la liste.

func TestAdressesInexploitables(t *testing.T) {
	trouvés := Récolter([]himalaya.Enveloppe{{
		ID: "1",
		De: []himalaya.Adresse{
			{Email: ""},
			{Email: "   "},
			{Email: "inconnu"},
			{Nom: "Groupe", Email: "adresse@localhost"},
			{Nom: "Serveur", Email: "do-not-reply@lists.sr.ht"},
			{Nom: "Alice", Email: "alice@example.org"},
		},
	}}, nil)

	vus := []string{}
	for _, c := range trouvés {
		vus = append(vus, c.Email)
	}
	attendus := []string{"alice@example.org", "adresse@localhost"}
	if !reflect.DeepEqual(vus, attendus) {
		t.Errorf("contacts = %v, veut %v", vus, attendus)
	}
}

// la correspondance est une recherche par sous-séquence : « almt » trouve
// « Alice Martin », comme dans tout client mail qui propose ses contacts.

func TestSousSéquenceSurLeNom(t *testing.T) {
	liste := []Contact{
		{Nom: "Alice Martin", Email: "alice@example.org"},
		{Nom: "Bob Durand", Email: "bob@example.org"},
	}

	trouvés := Correspondance(liste, "almt", 10)
	if len(trouvés) == 0 || trouvés[0].Nom != "Alice Martin" {
		t.Errorf("trouvés = %+v", trouvés)
	}
}

// la saisie peut viser le nom ou l'adresse, et l'adresse compte double.

func TestCorrespondanceSurLEmail(t *testing.T) {
	liste := []Contact{
		{Nom: "Alice", Email: "alice@example.org"},
		{Nom: "Bob", Email: "bob@exemple.fr"},
	}

	trouvés := Correspondance(liste, "exemple", 10)
	if len(trouvés) != 1 || trouvés[0].Nom != "Bob" {
		t.Errorf("trouvés = %+v", trouvés)
	}
}

// un nom court comme une seule lettre propose beaucoup : c'est voulu, l'utilisateur
// affine avec les flèches, et la liste est plafonnée.

func TestListePlafonnée(t *testing.T) {
	liste := []Contact{
		{Nom: "Alice", Email: "a@x.org"},
		{Nom: "Aline", Email: "b@x.org"},
		{Nom: "Alix", Email: "c@x.org"},
		{Nom: "Alban", Email: "d@x.org"},
	}

	if len(Correspondance(liste, "a", 2)) != 2 {
		t.Error("la liste de propositions doit respecter la limite")
	}
}

// pas de correspondance : pas de popup, pas de souci.

func TestSansCorrespondance(t *testing.T) {
	liste := []Contact{{Nom: "Alice", Email: "alice@example.org"}}

	if got := Correspondance(liste, "zzzzz", 10); len(got) != 0 {
		t.Errorf("trouvés = %+v, veut aucun", got)
	}
	if got := Correspondance(nil, "a", 10); len(got) != 0 {
		t.Errorf("sur une liste vide = %+v", got)
	}
}

// une saisie vide ne doit pas proposer les mille contacts du moisson : la popup
// n'a de sens qu'à partir du premier caractère.

func TestSaisieVide(t *testing.T) {
	liste := []Contact{{Nom: "Alice", Email: "alice@example.org"}}
	if got := Correspondance(liste, "", 10); len(got) != 0 {
		t.Errorf("trouvés = %+v, veut aucun sur une saisie vide", got)
	}
	if got := Correspondance(liste, "   ", 10); len(got) != 0 {
		t.Errorf("trouvés = %+v, veut aucun sur une saisie blanche", got)
	}
}

// un préfixe doit primer sur une sous-séquence plus lâche : taper « ali » doit
// remonter « Alice » avant « Aline ».

func TestPréfixePrioritaire(t *testing.T) {
	liste := []Contact{
		{Nom: "Aline Duval", Email: "aline@x.org"},
		{Nom: "Alice Martin", Email: "alice@x.org"},
	}

	trouvés := Correspondance(liste, "ali", 10)
	if len(trouvés) != 2 {
		t.Fatalf("trouvés = %+v", trouvés)
	}
	if trouvés[0].Nom != "Alice Martin" {
		t.Errorf("premier = %q, veut Alice Martin (le préfixe exact)", trouvés[0].Nom)
	}
}

// la recherche ignore la casse et les accents frappés à la française.

func TestRechercheInsensible(t *testing.T) {
	liste := []Contact{{Nom: "Élodie Durand", Email: "elodie@example.org"}}

	if len(Correspondance(liste, "elodie", 10)) == 0 {
		t.Error("la saisie sans accent doit trouver Élodie")
	}
	if len(Correspondance(liste, "ÉLODIE", 10)) == 0 {
		t.Error("la saisie en majuscules doit trouver Élodie")
	}
}

// l'affichage retenu est celui du champ qui a servi à la correspondance : taper
// une adresse propose l'adresse, taper un nom propose le nom.

func TestAffichageSelonLeChamp(t *testing.T) {
	liste := []Contact{{Nom: "Alice Martin", Email: "alice@example.org"}}

	if got := Correspondance(liste, "alice@ex", 1)[0].Affichage(); got != "alice@example.org" {
		t.Errorf("Affichage = %q, veut l'adresse seule", got)
	}
}

func TestAffichageAvecNom(t *testing.T) {
	if got := (Contact{Nom: "Alice", Email: "a@x.org"}).Affichage(); got != "Alice <a@x.org>" {
		t.Errorf("Affichage = %q", got)
	}
	if got := (Contact{Email: "a@x.org"}).Affichage(); got != "a@x.org" {
		t.Errorf("Affichage = %q", got)
	}
}

// le moisson se fait par pages : le TUI doit savoir quand s'arrêter.

func TestPagesÀParcourir(t *testing.T) {
	cases := []struct {
		total, parPage, maximum int
		attendu                 int
	}{
		{100, 25, 2000, 4},
		{10, 25, 2000, 1},
		{5000, 25, 2000, 80},
		{0, 25, 2000, 0},
		{100, 0, 2000, 1},
	}
	for _, c := range cases {
		if got := Pages(c.total, c.parPage, c.maximum); got != c.attendu {
			t.Errorf("Pages(%d, %d, %d) = %d, veut %d", c.total, c.parPage, c.maximum, got, c.attendu)
		}
	}
}
