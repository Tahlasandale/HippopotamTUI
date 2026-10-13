package redaction

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
)

func adresse(nom, email string) himalaya.Adresse {
	return himalaya.Adresse{Nom: nom, Email: email}
}

var (
	moi    = []string{"Orphin@protonmail.com", "meute.saintfrancois78@gmail.com"}
	alice  = adresse("Alice", "alice@example.org")
	bob    = adresse("Bob", "bob@example.org")
	carla  = adresse("Carla", "carla@example.org")
	moiALi = adresse("Orphin", "Orphin@protonmail.com")

	// réponse est la source la plus courante : Alice écrit, Bob est en To,
	// Carla est en copie, et nous ne sommes pas l'expéditeur.
	réponse = Source{
		Origine:    himalaya.Origine{Kind: himalaya.Reponse, ID: "1"},
		Expediteur: []himalaya.Adresse{alice},
		Vers:       []himalaya.Adresse{bob},
		Copie:      []himalaya.Adresse{carla},
		Propres:    moi,
	}
)

// une réponse simple ne vise que l'expéditeur.

func TestReponseSimple(t *testing.T) {
	source := réponse

	to, cc := source.Destinataires(false)
	if !reflect.DeepEqual(to, []string{"alice@example.org"}) {
		t.Errorf("To = %v", to)
	}
	if len(cc) != 0 {
		t.Errorf("Cc = %v, veut vide", cc)
	}
}

// une réponse simple ne reprend personne en copie, même si l'original en avait.

func TestReponseSimpleNeReprendPasLesCc(t *testing.T) {
	source := réponse

	_, cc := source.Destinataires(false)
	if len(cc) != 0 {
		t.Errorf("Cc = %v, veut vide : répondre à quelqu'un ne met personne en copie", cc)
	}
}

// le Reply-To prime sur le From quand il est présent : c'est à lui qu'on écrit.

func TestReponsePrendLeReplyTo(t *testing.T) {
	source := réponse
	source.ReplyTo = []string{"list@example.org"}

	to, _ := source.Destinataires(false)
	if !reflect.DeepEqual(to, []string{"list@example.org"}) {
		t.Errorf("To = %v, veut le Reply-To", to)
	}
}

// répondre à tous : les To d'origine restent en To, les Cc restent en Cc, et ses
// propres adresses disparaissent des deux listes.

func TestReponseÀTous(t *testing.T) {
	source := réponse
	source.Vers = append(source.Vers, moiALi)
	source.Copie = append(source.Copie, moiALi)

	to, cc := source.Destinataires(true)
	if !reflect.DeepEqual(to, []string{"alice@example.org", "bob@example.org"}) {
		t.Errorf("To = %v, veut Alice puis Bob, sans Orphin", to)
	}
	if !reflect.DeepEqual(cc, []string{"carla@example.org"}) {
		t.Errorf("Cc = %v, veut Carla seule", cc)
	}
}

// répondre à un de ses propres messages : on vise les destinataires d'origine,
// pas l'expéditeur, qui est soi-même.

func TestReponseÀUnEnvoi(t *testing.T) {
	source := Source{
		Origine:    himalaya.Origine{Kind: himalaya.Reponse, ID: "1"},
		Expediteur: []himalaya.Adresse{moiALi},
		Vers:       []himalaya.Adresse{alice, bob},
		Copie:      []himalaya.Adresse{carla},
		Propres:    moi,
	}

	to, cc := source.Destinataires(false)
	if !reflect.DeepEqual(to, []string{"alice@example.org", "bob@example.org"}) {
		t.Errorf("To = %v, veut les destinataires d'origine", to)
	}
	if !reflect.DeepEqual(cc, []string{"carla@example.org"}) {
		t.Errorf("Cc = %v", cc)
	}
}

// une adresse apparaissant deux fois, en To puis en Cc, ne doit apparaître
// qu'une fois : un doublon dans un courriel s'étale d'un fil de discussion.

func TestDédoublonnage(t *testing.T) {
	source := Source{
		Origine:    himalaya.Origine{Kind: himalaya.Reponse, ID: "1"},
		Expediteur: []himalaya.Adresse{adresse("Alice", "ALICE@example.org")},
		Vers:       []himalaya.Adresse{adresse("alice", "alice@EXAMPLE.org")},
		Copie:      []himalaya.Adresse{adresse("ALICE", "alice@example.ORG")},
		Propres:    moi,
	}

	to, cc := source.Destinataires(true)
	if !reflect.DeepEqual(to, []string{"ALICE@example.org"}) {
		t.Errorf("To = %v, veut une seule occurrence", to)
	}
	if len(cc) != 0 {
		t.Errorf("Cc = %v, l'adresse est déjà en To", cc)
	}
}

// la comparaison des adresses ignore la casse : ton compte proton s'écrit
// Orphin@protonmail.com avec un O majuscule dans la partie locale, et il est
// très facile de ne pas le recopier exactement.

func TestComparaisonInsensibleÀLaCasse(t *testing.T) {
	source := Source{
		Origine:    himalaya.Origine{Kind: himalaya.Reponse, ID: "1"},
		Expediteur: []himalaya.Adresse{adresse("Moi", "orphin@PROTONMAIL.com")},
		Propres:    moi,
	}

	to, _ := source.Destinataires(true)
	if len(to) != 0 {
		t.Errorf("To = %v, veut vide : c'est sa propre adresse", to)
	}
}

// un message sans expéditeur ne doit pas faire planter la réponse : il n'y a
// personne à qui écrire, le formulaire s'ouvrira vide et l'utilisateur corrigera.

func TestSansExpéditeur(t *testing.T) {
	source := Source{
		Origine: himalaya.Origine{Kind: himalaya.Reponse, ID: "1"},
		Vers:    []himalaya.Adresse{alice},
		Propres: moi,
	}

	to, cc := source.Destinataires(true)
	if !reflect.DeepEqual(to, []string{"alice@example.org"}) {
		t.Errorf("To = %v", to)
	}
	if len(cc) != 0 {
		t.Errorf("Cc = %v", cc)
	}
}

// un transfert ne reprend personne : son destinataire est choisi à la main.

func TestTransfertVide(t *testing.T) {
	source := réponse
	source.Origine.Kind = himalaya.Transfert

	to, cc := source.Destinataires(true)
	if len(to) != 0 || len(cc) != 0 {
		t.Errorf("To = %v, Cc = %v, veut des listes vides", to, cc)
	}
}

// le sujet d'une réponse est celui de l'original, préfixé une seule fois, même si
// l'original portait déjà un Re: ou un « Re : » avec une espace.

func TestSujetDeRéponse(t *testing.T) {
	cases := []struct{ original, attendu string }{
		{"Question", "Re: Question"},
		{"Re: Question", "Re: Question"},
		{"RE: Question", "Re: Question"},
		{"Re : Question", "Re: Question"},
		{"[liste] Question", "Re: [liste] Question"},
		{"", "Re:"},
	}

	for _, c := range cases {
		if got := SujetReponse(c.original); got != c.attendu {
			t.Errorf("SujetReponse(%q) = %q, veut %q", c.original, got, c.attendu)
		}
	}
}

func TestSujetDeTransfert(t *testing.T) {
	if got := SujetTransfert("Question"); got != "Fwd: Question" {
		t.Errorf("SujetTransfert = %q", got)
	}
	if got := SujetTransfert("Fwd: Question"); got != "Fwd: Question" {
		t.Errorf("SujetTransfert = %q", got)
	}
}

// l'objet est coupé proprement quand il est trop long pour une ligne, au mot le
// plus proche, jamais au milieu d'un mot.

func TestTroncatureDObjet(t *testing.T) {
	long := "Réunion du conseil municipal du premier octobre deux mille vingt-six"
	got := Tronquer(long, 30)
	if len([]rune(got)) > 30 {
		t.Errorf("Tronquer = %q (%d runes), veut au plus 30", got, len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("Tronquer = %q, veut une ellipse finale", got)
	}
	if got := Tronquer("Court", 30); got != "Court" {
		t.Errorf("Tronquer = %q, veut le texte inchangé", got)
	}
}

func TestTroncatureSurLesCaractèresEtPasLesOctets(t *testing.T) {
	// un objet de 200 caractères accentués doit être coupé à 10 sans jamais
	// casser un caractère au milieu de ses octets.
	sujet := strings.Repeat("éàüçô", 40)
	got := Tronquer(sujet, 10)

	if !strings.HasSuffix(got, "…") {
		t.Errorf("Tronquer = %q, veut une ellipse", got)
	}
	for _, r := range got {
		if r == '�' {
			t.Errorf("Tronquer = %q, veut de l'UTF-8 valide", got)
		}
	}
}

// ---------------------------------------------------------------- filtres

func TestFiltresVides(t *testing.T) {
	if q := (Filtres{}).Requête(); q != "" {
		t.Errorf("Requête = %q, veut vide", q)
	}
}

func TestFiltresGuident(t *testing.T) {
	f := Filtres{
		De:          "alice",
		Objet:       "réunion",
		Corps:       "annulée",
		Drapeaux:    []string{"seen"},
		Depuis:      date(2026, 1, 1),
		Avant:       date(2026, 2, 1),
		NonLusSeuls: true,
	}

	q := f.Requête()
	for _, attendu := range []string{
		`from alice`,
		`subject réunion`,
		`body annulée`,
		`flag seen`,
		`after 2026-01-01`,
		`before 2026-02-01`,
		`not flag seen`,
	} {
		if !strings.Contains(q, attendu) {
			t.Errorf("Requête = %q, ne contient pas %q", q, attendu)
		}
	}
	if !strings.Contains(q, " and ") {
		t.Errorf("Requête = %q, veut des critères combinés par and", q)
	}
}

// deux exécutions sur les mêmes critères doivent produire exactement la même
// requête, sinon le cache ne sert à rien.

func TestRequêteStable(t *testing.T) {
	f := Filtres{De: "a", Objet: "b", Drapeaux: []string{"seen", "flagged"}}
	if f.Requête() != f.Requête() {
		t.Error("la requête n'est pas stable")
	}
}

// un motif contenant un espace doit être cité : sans guillemets, himalaya le
// prendrait pour plusieurs motifs.

func TestCitationDesMotifs(t *testing.T) {
	q := Filtres{De: "Marie Curie", Objet: `dis " urgent "`}.Requête()

	if !strings.Contains(q, `from "Marie Curie"`) {
		t.Errorf("Requête = %q", q)
	}
	if !strings.Contains(q, `subject "dis \" urgent \""`) {
		t.Errorf("Requête = %q, veut les guillemets internes échappés", q)
	}
}

// un motif qui ressemble déjà à une citation ne doit pas être échappé deux fois.

func TestCitationIdempotente(t *testing.T) {
	q := Filtres{De: `"déjà cité"`}.Requête()
	if q != `from "déjà cité"` {
		t.Errorf("Requête = %q, veut la citation reprise telle quelle", q)
	}
}

func TestDrapeauxMultiples(t *testing.T) {
	q := Filtres{Drapeaux: []string{"seen", "answered"}}.Requête()
	if q != "flag seen and flag answered" {
		t.Errorf("Requête = %q", q)
	}
}

// des espaces autour d'une date inutilisable ne doivent pas produire une date
// pour le premier janvier de l'an zéro.

func TestDatesAbsentes(t *testing.T) {
	if q := (Filtres{Depuis: time.Time{}}).Requête(); strings.Contains(q, "after") {
		t.Errorf("Requête = %q, veut aucune date", q)
	}
	f := Filtres{Avant: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)}
	if q := f.Requête(); q != "before 2026-03-15" {
		t.Errorf("Requête = %q", q)
	}
}

// la description affichée sous les filtres doit rester lisible, sans laisser
// fuir la syntaxe de himalaya.

func TestDescriptionDesFiltres(t *testing.T) {
	d := Filtres{De: "alice", NonLusSeuls: true}.Description()
	if !strings.Contains(d, "alice") || !strings.Contains(d, "non lus") {
		t.Errorf("Description = %q", d)
	}
	if strings.Contains(d, "flag") {
		t.Errorf("Description = %q, ne doit pas laisser fuir la syntaxe", d)
	}
	if (Filtres{}).Description() != "" {
		t.Error("aucun filtre ne doit produire une description vide de sens")
	}
}

func date(année int, mois time.Month, jour int) time.Time {
	return time.Date(année, mois, jour, 0, 0, 0, 0, time.UTC)
}
