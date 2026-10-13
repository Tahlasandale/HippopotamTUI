package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tahlasandale/HippopotamTUI/internal/cache"
	"github.com/Tahlasandale/HippopotamTUI/internal/config"
	"github.com/Tahlasandale/HippopotamTUI/internal/contacts"
	"github.com/Tahlasandale/HippopotamTUI/internal/demo"
	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	"github.com/Tahlasandale/HippopotamTUI/internal/rfc822"
	tea "github.com/charmbracelet/bubbletea"
)

// Le modèle est testé sur le client de démonstration : aucun réseau, aucun
// binaire himalaya, et un jeu de messages stable d'un test à l'autre.

// harnais pilote le modèle comme le ferait bubbletea : les commandes renvoyées
// sont exécutées puis leur résultat réinjecté, jusqu'à ce qu'il n'y ait plus rien
// à faire.
type harnais struct {
	t *testing.T
	m *Modèle
	// en_attente est la file des commandes produites par le modèle, que le
	// drainer exécute.
	en_attente []tea.Cmd
}

func nouveauHarnais(t *testing.T) *harnais {
	t.Helper()

	client := demo.Nouveau()
	store, err := ouvrirStore(t)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Editor = "true" // les tests ne doivent jamais ouvrir un vrai éditeur
	m := Nouveau(client, store, cfg, true)

	h := &harnais{t: t, m: m}
	h.envoyer(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.drainer(m.Init())
	return h
}

func ouvrirStore(t *testing.T) (*cache.Store, error) {
	return cache.Ouvrir(filepath.Join(t.TempDir(), "cache.db"))
}

func (h *harnais) envoyer(msg tea.Msg) tea.Cmd {
	h.t.Helper()
	var cmd tea.Cmd
	modèle, cmd := h.m.Update(msg)
	h.m = modèle.(*Modèle)
	return cmd
}

// drainer exécute les commandes jusqu'à ce que le modèle n'en demande plus :
// c'est le rôle de la boucle bubbletea, simulé ici.
func (h *harnais) drainer(init tea.Cmd) {
	h.t.Helper()
	if init != nil {
		h.en_attente = append([]tea.Cmd{init}, h.en_attente...)
	}

	for tour := 0; tour < 32; tour++ {
		if len(h.en_attente) == 0 {
			return
		}
		courant := h.en_attente
		h.en_attente = nil

		for _, cmd := range courant {
			if cmd == nil {
				continue
			}
			msg := cmd()
			if msg == nil || msg == tea.Quit() {
				continue
			}
			if lot, ok := msg.(tea.BatchMsg); ok {
				h.en_attente = append(h.en_attente, lot...)
				continue
			}
			if suite := h.envoyer(msg); suite != nil {
				h.en_attente = append(h.en_attente, suite)
			}
		}
	}
	h.t.Fatal("les commandes ne se terminent pas : boucle dans le modèle")
}

// vider exécute ce qui reste en attente.
func (h *harnais) vider() {
	h.t.Helper()
	h.drainer(nil)
}

// toucher envoie une touche comme le terminal le ferait, et exécute la commande
// qu'elle déclenche.
func (h *harnais) toucher(msg tea.KeyMsg) {
	h.t.Helper()
	if cmd := h.envoyer(msg); cmd != nil {
		h.drainer(cmd)
	}
}

func (h *harnais) rune(touche string) {
	h.t.Helper()
	r := []rune(touche)
	h.toucher(tea.KeyMsg{Type: tea.KeyRunes, Runes: r})
}

func (h *harnais) taper(texte string) {
	h.t.Helper()
	for _, r := range texte {
		h.rune(string(r))
	}
}

func (h *harnais) touche(speciale tea.KeyType) {
	h.t.Helper()
	h.toucher(tea.KeyMsg{Type: speciale})
}

func (h *harnais) ctrl(letter string) {
	h.t.Helper()
	h.toucher(tea.KeyMsg{Type: tea.KeyCtrlC})
}

// vue rend l'écran et le normalise pour la comparaison.
func (h *harnais) vue() string {
	h.t.Helper()
	return h.m.View()
}

// ------------------------------------------------------------------ tests

func TestDémarrageChargeLaBoîteDeRéception(t *testing.T) {
	h := nouveauHarnais(t)

	if h.m.compte != "proton" {
		t.Errorf("compte = %q, veut le compte par défaut", h.m.compte)
	}
	if h.m.boîte != "Inbox" {
		t.Errorf("boîte = %q, veut Inbox", h.m.boîte)
	}
	if len(h.m.enveloppes) == 0 {
		t.Fatal("aucune enveloppe chargée")
	}
	if h.m.écran != ÉcranListe {
		t.Errorf("écran = %v, veut la liste", h.m.écran)
	}

	vue := h.vue()
	for _, attendu := range []string{"proton", "Inbox", "1/", "Commission Scolaire"} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("la vue ne montre pas %q :\n%s", attendu, vue)
		}
	}
}

func TestNavigationAuxFlèches(t *testing.T) {
	h := nouveauHarnais(t)

	premier := h.m.enveloppes[0].Sujet
	h.touche(tea.KeyDown)
	if h.m.curseur != 1 {
		t.Fatalf("curseur = %d, veut 1", h.m.curseur)
	}
	if h.vue() == "" {
		t.Error("la vue est vide après navigation")
	}
	if h.m.enveloppes[1].Sujet == premier {
		t.Error("les deux premiers messages auraient le même objet")
	}

	// remonter au-delà de la première ligne ne doit pas sortir de la liste.
	for i := 0; i < 5; i++ {
		h.touche(tea.KeyUp)
	}
	if h.m.curseur != 0 {
		t.Errorf("curseur = %d, veut 0 après remontée excessive", h.m.curseur)
	}

	h.touche(tea.KeyEnd)
	if h.m.curseur != len(h.m.enveloppes)-1 {
		t.Errorf("curseur = %d, veut la dernière ligne", h.m.curseur)
	}
}

func TestRaccourcisVimRestentAcceptés(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("j")
	if h.m.curseur != 1 {
		t.Fatalf("curseur = %d, veut 1 après « j »", h.m.curseur)
	}
	h.rune("k")
	if h.m.curseur != 0 {
		t.Fatalf("curseur = %d, veut 0 après « k »", h.m.curseur)
	}
	h.rune("G")
	if h.m.curseur != len(h.m.enveloppes)-1 {
		t.Fatalf("curseur = %d, veut la fin après « G »", h.m.curseur)
	}
	h.rune("g")
	if h.m.curseur != 0 {
		t.Fatalf("curseur = %d, veut le début après « g »", h.m.curseur)
	}
}

func TestOuvertureDUnMessage(t *testing.T) {
	h := nouveauHarnais(t)

	h.touche(tea.KeyEnter)
	h.vider()

	if h.m.écran != ÉcranMessage {
		t.Fatalf("écran = %v, want le message", h.m.écran)
	}
	if h.m.message == nil {
		t.Fatal("aucun message décodé")
	}
	if h.m.message.Sujet == "" {
		t.Error("le message n'a pas d'objet")
	}
	if len(h.m.lignes) == 0 {
		t.Error("le corps n'a pas été rendu")
	}

	vue := h.vue()
	for _, attendu := range []string{"De :", "Objet", h.m.message.Sujet} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("la vue ne montre pas %q :\n%s", attendu, vue)
		}
	}

	// un message non lu devient lu à l'ouverture, sans repasser par le réseau.
	if !h.m.enveloppes[0].A(himalaya.PoussièreVolee) {
		t.Error("le message devrait avoir été marqué comme lu")
	}
}

func TestRetourÀLaListe(t *testing.T) {
	h := nouveauHarnais(t)

	h.touche(tea.KeyEnter)
	h.vider()
	h.rune("q")

	if h.m.écran != ÉcranListe {
		t.Errorf("écran = %v, veut la liste", h.m.écran)
	}
}

func TestMessageSuivantSansRepasserParLaListe(t *testing.T) {
	h := nouveauHarnais(t)

	h.touche(tea.KeyEnter)
	h.vider()
	premier := h.m.message.Sujet

	h.rune("M")
	h.vider()

	if h.m.curseur != 1 {
		t.Errorf("curseur = %d, veut 1", h.m.curseur)
	}
	if h.m.message.Sujet == premier {
		t.Error("le message suivant n'a pas été ouvert")
	}
	if h.m.écran != ÉcranMessage {
		t.Errorf("écran = %v, veut rester dans le message", h.m.écran)
	}
}

func TestDéfilementDuMessage(t *testing.T) {
	h := nouveauHarnais(t)
	h.touche(tea.KeyEnter)
	h.vider()

	h.rune("G")
	if h.m.défilement == 0 && len(h.m.lignes) > h.m.lignesVisibles() {
		t.Error("« G » devrait aller à la fin du message")
	}
	h.rune("g")
	if h.m.défilement != 0 {
		t.Errorf("défilement = %d, veut 0", h.m.défilement)
	}
}

func TestBasculerLuEtMarqué(t *testing.T) {
	h := nouveauHarnais(t)

	// le premier message de la démonstration est non lu
	if h.m.enveloppes[0].A(himalaya.PoussièreVolee) {
		t.Fatal("le premier message devrait être non lu")
	}
	h.rune("v")
	if !h.m.enveloppes[0].A(himalaya.PoussièreVolee) {
		t.Error("« v » n'a pas marqué le message comme lu")
	}
	h.rune("v")
	if h.m.enveloppes[0].A(himalaya.PoussièreVolee) {
		t.Error("« v » aurait dû le marquer comme non lu")
	}

	h.rune("F")
	if !h.m.enveloppes[0].A(himalaya.Marqué) {
		t.Error("« F » n'a pas marqué le message")
	}
	h.rune("F")
	if h.m.enveloppes[0].A(himalaya.Marqué) {
		t.Error("« F » aurait dû démarquer le message")
	}
}

func TestSuppressionSansConfirmation(t *testing.T) {
	h := nouveauHarnais(t)
	avant := len(h.m.enveloppes)

	h.rune("d")
	h.vider()

	if len(h.m.enveloppes) != avant-1 {
		t.Errorf("il reste %d messages, veut %d", len(h.m.enveloppes), avant-1)
	}
	if !strings.Contains(h.vue(), "corbeille") {
		t.Errorf("la vue n'annonce pas la mise à la corbeille :\n%s", h.vue())
	}
}

func TestChangementDeBoîte(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("m")
	if h.m.écran != ÉcranChoix {
		t.Fatalf("écran = %v, veut le sélecteur", h.m.écran)
	}
	vue := h.vue()
	for _, attendu := range []string{"Boîte", "Inbox", "Sent", "Archive"} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("le sélecteur ne montre pas %q :\n%s", attendu, vue)
		}
	}

	h.touche(tea.KeyDown) // depuis Inbox, la boîte suivante
	h.touche(tea.KeyEnter)
	h.vider()

	if h.m.boîte != "Sent" {
		t.Errorf("boîte = %q, veut Sent", h.m.boîte)
	}
	if len(h.m.enveloppes) == 0 {
		t.Error("la boîte choisie est vide")
	}
}

func TestFlèchesChangentDeBoîte(t *testing.T) {
	h := nouveauHarnais(t)

	h.touche(tea.KeyRight)
	h.vider()

	if h.m.boîte != "Sent" {
		t.Errorf("boîte = %q, veut Sent", h.m.boîte)
	}
}

func TestChangementDeCompte(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("S")
	if h.m.écran != ÉcranChoix {
		t.Fatalf("écran = %v, veut le sélecteur de compte", h.m.écran)
	}
	h.touche(tea.KeyDown)
	h.touche(tea.KeyEnter)
	h.vider()

	if h.m.compte != "gmail" {
		t.Errorf("compte = %q, veut gmail", h.m.compte)
	}
}

func TestFiltresGuidés(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("/")
	if h.m.écran != ÉcranFiltres {
		t.Fatalf("écran = %v, veut les filtres", h.m.écran)
	}

	h.taper("Mairie")
	if h.m.filtres.De != "Mairie" {
		t.Errorf("filtre De = %q, want Mairie", h.m.filtres.De)
	}

	h.touche(tea.KeyEnter)
	h.vider()

	if h.m.écran != ÉcranListe {
		t.Errorf("écran = %v, veut la liste", h.m.écran)
	}
	if !strings.Contains(h.vue(), "filtre :") {
		t.Errorf("la vue n'annonce pas le filtre :\n%s", h.vue())
	}
}

// les critères de date se règlent au clavier, sans taper une date entière.
func TestFiltreDeDateAuClavier(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("/")
	for i := 0; i < 3; i++ { // De, Objet, Corps
		h.touche(tea.KeyDown)
	}
	if critèresFiltres[h.m.indexFiltre].clé != "depuis" {
		t.Fatalf("critère courant = %q, veut « depuis »", critèresFiltres[h.m.indexFiltre].clé)
	}

	h.touche(tea.KeyRight)
	if h.m.filtres.Depuis.IsZero() {
		t.Fatal("« depuis » devrait avoir une date")
	}
	avant := h.m.filtres.Depuis
	h.touche(tea.KeyRight)
	if !h.m.filtres.Depuis.After(avant) {
		t.Error("« depuis » devrait avancer d'un mois")
	}
}

func TestFiltreNonLus(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("/")
	for i := 0; i < 5; i++ {
		h.touche(tea.KeyDown)
	}
	h.touche(tea.KeySpace)

	if !h.m.filtres.NonLusSeuls {
		t.Fatal("« non lus » devrait être coché")
	}
	if !strings.Contains(h.m.filtres.Requête(), "not flag seen") {
		t.Errorf("requête = %q", h.m.filtres.Requête())
	}
}

func TestAnnulerLesFiltres(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("/")
	h.taper("Mairie")
	h.touche(tea.KeyEsc)

	if h.m.filtres.De != "" {
		t.Errorf("le filtre De = %q, veut vide après annulation", h.m.filtres.De)
	}
}

func TestAide(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("?")
	if h.m.écran != ÉcranAide {
		t.Fatalf("écran = %v, veut l'aide", h.m.écran)
	}
	vue := h.vue()
	for _, attendu := range []string{"Raccourcis", "Liste", "Rédaction", "répondre à tous"} {
		if !strings.Contains(vue, attendu) {
			t.Errorf("l'aide ne montre pas %q :\n%s", attendu, vue)
		}
	}
	h.touche(tea.KeyEsc)
	if h.m.écran != ÉcranListe {
		t.Errorf("écran = %v, veut la liste", h.m.écran)
	}
}

func TestJournal(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("J")
	if h.m.écran != ÉcranJournal {
		t.Fatalf("écran = %v, veut le journal", h.m.écran)
	}
	vue := h.vue()
	if !strings.Contains(vue, "comptes") {
		t.Errorf("le journal ne retrace pas les appels :\n%s", vue)
	}
}

func TestTerminalTropPetit(t *testing.T) {
	h := nouveauHarnais(t)

	h.envoyer(tea.WindowSizeMsg{Width: 20, Height: 5})
	vue := h.vue()

	if !strings.Contains(vue, "40 colonnes") {
		t.Errorf("la vue ne prévient pas de la taille insuffisante :\n%s", vue)
	}
}

// ---------------------------------------------------------------- rédaction

func TestComposerUnMessage(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("c")
	if h.m.formulaire == nil {
		t.Fatal("aucun formulaire après « c »")
	}
	if h.m.écran != ÉcranRédaction {
		t.Errorf("écran = %v, veut la rédaction", h.m.écran)
	}
	if h.m.formulaire.valide() {
		t.Error("un formulaire vide ne devrait pas être valide")
	}
}

func TestRéponseRemplitDestinataireEtObjet(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("r")
	if h.m.formulaire == nil {
		t.Fatal("aucun formulaire après « r »")
	}
	if !strings.HasPrefix(h.m.formulaire.Sujet, "Re: ") {
		t.Errorf("objet = %q, veut un préfixe Re:", h.m.formulaire.Sujet)
	}
	if len(h.m.formulaire.To) == 0 {
		t.Error("la réponse n'a pas de destinataire")
	}
	if h.m.formulaire.origine.Kind != himalaya.Reponse {
		t.Errorf("origine = %v, veut une réponse", h.m.formulaire.origine.Kind)
	}
}

func TestRéponseÀTousDedoublonne(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("a")
	f := h.m.formulaire
	if f == nil {
		t.Fatal("aucun formulaire")
	}
	vus := map[string]bool{}
	for _, adresse := range append(append([]string{}, f.To...), f.Cc...) {
		if vus[strings.ToLower(adresse)] {
			t.Errorf("l'adresse %s apparaît deux fois", adresse)
		}
		vus[strings.ToLower(adresse)] = true
	}
}

func TestChangementDeChampDansLeFormulaire(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("r")
	f := h.m.formulaire

	if f.champ != ChampÀ {
		t.Fatalf("champ initial = %v, veut le champ À", f.champ)
	}
	h.touche(tea.KeyDown)
	if f.champ != ChampCc {
		t.Errorf("champ = %v, veut Cc", f.champ)
	}
	h.touche(tea.KeyDown)
	if f.champ != ChampSujet {
		t.Errorf("champ = %v, veut Objet", f.champ)
	}
	h.touche(tea.KeyDown)
	if f.champ != ChampÀ {
		t.Errorf("champ = %v, veut un retour sur À", f.champ)
	}
}

// une adresse saisie sans « @ » ne doit jamais partir : c'est le refuge des
// destinataires coupés par une frappe.
func TestSaisieDAdresseIncomplète(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("c")
	f := h.m.formulaire
	f.saisie = ""
	h.taper("alice")
	h.touche(tea.KeyEnter)

	if len(f.To) != 0 {
		t.Errorf("destinataires = %v, aucun ne contient de « @ »", f.To)
	}
}

// la proposition d'adresse vient du carnet moissonné.
func TestAutocomplétion(t *testing.T) {
	h := nouveauHarnais(t)

	h.m.contacts = []contacts.Contact{{Nom: "Alice Martin", Email: "alice@example.org"}}
	h.rune("c")

	f := h.m.formulaire
	f.contacts = h.m.contacts
	f.saisie = ""
	h.taper("almt")

	if len(f.proposés) == 0 {
		t.Fatalf("aucune proposition pour « almt » : %+v", f.contacts)
	}
	h.touche(tea.KeyEnter)

	f.valider()
	if len(f.To) != 1 || f.To[0] != "alice@example.org" {
		t.Errorf("destinataires = %v, veut alice@example.org", f.To)
	}
}

func TestEnvoi(t *testing.T) {
	client := demo.Nouveau()
	store, _ := ouvrirStore(t)
	cfg := config.Default()
	cfg.Editor = "true" // les tests ne doivent jamais ouvrir un vrai éditeur
	m := Nouveau(client, store, cfg, true)

	h := &harnais{t: t, m: m}
	h.envoyer(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.vider()

	h.rune("c")
	f := h.m.formulaire
	f.saisie = "alice@example.org"
	f.valider()
	f.Sujet = "Un message de test"
	f.Corps = "Bonjour depuis les tests."

	h.toucher(tea.KeyMsg{Type: tea.KeyCtrlS})
	h.vider()

	if h.m.écran != ÉcranListe {
		t.Errorf("écran = %v, veut la liste après envoi", h.m.écran)
	}
	if !strings.Contains(h.vue(), "envoyé") {
		t.Errorf("la vue n'annonce pas l'envoi :\n%s", h.vue())
	}
}

func TestEnvoyerSansDestinataireRefusé(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("c")
	h.toucher(tea.KeyMsg{Type: tea.KeyCtrlS})

	if h.m.écran != ÉcranRédaction {
		t.Errorf("écran = %v, veut rester en rédaction", h.m.écran)
	}
	if !strings.Contains(h.m.erreur, "destinataire") {
		t.Errorf("erreur = %q, veut une mention de destinataire", h.m.erreur)
	}
}

// quitter la rédaction avec un contenu doit laisser un brouillon ; quitter un
// formulaire vidé ne doit rien laisser.
func TestAbandonnerUneRédactionGardeLeBrouillon(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("r")
	h.touche(tea.KeyEsc)

	if h.m.écran != ÉcranListe {
		t.Errorf("écran = %v, veut la liste", h.m.écran)
	}
	if h.m.formulaire != nil {
		t.Error("le formulaire aurait dû être abandonné")
	}
	if !strings.Contains(h.m.info, "brouillon") {
		t.Errorf("info = %q, veut une mention de brouillon", h.m.info)
	}
}

func TestAbandonnerUneRédactionVideNeLaisseRien(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("c")
	h.touche(tea.KeyEsc)

	if strings.Contains(h.m.info, "brouillon") {
		t.Errorf("info = %q, un formulaire vide ne doit rien enregistrer", h.m.info)
	}
}

// ------------------------------------------------------------------ divers

func TestRenduDuCorpsReplieLesLonguesLignes(t *testing.T) {
	message := &rfc822.Message{Corps: strings.Repeat("a", 300)}
	lignes := rendreCorps(message, 40)

	for _, ligne := range lignes {
		if len([]rune(ligne)) > 60 {
			t.Errorf("ligne de %d runes, la largeur est 40", len([]rune(ligne)))
		}
	}
	if len(lignes) < 7 {
		t.Errorf("lignes = %d, le texte aurait dû être replié", len(lignes))
	}
}

func TestRenduMarkdownSeulementQuandIlFaut(t *testing.T) {
	balise := &rfc822.Message{Corps: "# Titre\n\nDu **gras** ici.", DejaMarkdown: true}
	if !balise.EstMarkdown() {
		t.Error("une partie text/markdown devrait être rendue avec glamour")
	}

	html := &rfc822.Message{Corps: "Titre\n\nDu gras.", HTML: "<h1>Titre</h1><p>Du <b>gras</b>.</p>"}
	if !html.EstMarkdown() {
		t.Error("un message HTML devrait être rendu avec glamour")
	}

	simple := &rfc822.Message{Corps: "Bonjour\n\nDu gras, mais pas de balisage."}
	if simple.EstMarkdown() {
		t.Error("un texte brut ne doit jamais passer dans glamour")
	}
}

func TestBandeauDeBienvenue(t *testing.T) {
	h := nouveauHarnais(t)

	if !strings.Contains(h.vue(), "Bienvenue") {
		t.Errorf("la vue ne montre pas le bandeau d'accueil :\n%s", h.vue())
	}
}

// himalaya ne sait pas toujours lire un corps : le TUI a déjà décodé le message
// affiché, il s'en sert pour construire la citation plutôt que de laisser un
// squelette vide.
func TestCitationDeSecours(t *testing.T) {
	message := &rfc822.Message{
		Sujet: "Invitation",
		De:    "Mairie <mairie@example.org>",
		Corps: "Bonjour,\n\nCordialement.",
	}

	br := CompléterBrouillon(himalaya.Brouillon{Sujet: "Re : Invitation"}, himalaya.Origine{Kind: himalaya.Reponse}, message)

	if br.Sujet != "Re: Invitation" {
		t.Errorf("Sujet = %q, veut un seul Re:", br.Sujet)
	}
	if !strings.Contains(br.Corps, "> Bonjour,") {
		t.Errorf("Corps = %q, veut la citation du message affiché", br.Corps)
	}
}

// un objet déjà préfixé deux fois ne s'empile pas.
func TestObjetNormalisé(t *testing.T) {
	br := CompléterBrouillon(himalaya.Brouillon{Sujet: "Re: Re : Invitation"}, himalaya.Origine{Kind: himalaya.Reponse}, nil)
	if br.Sujet != "Re: Invitation" {
		t.Errorf("Sujet = %q", br.Sujet)
	}
}

// le transfert est encadré d'un séparateur lisible.
func TestTransfertEncadré(t *testing.T) {
	message := &rfc822.Message{Sujet: "Invitation", De: "Mairie <mairie@example.org>", Corps: "Bonjour."}

	br := CompléterBrouillon(himalaya.Brouillon{Sujet: "Invitation"}, himalaya.Origine{Kind: himalaya.Transfert}, message)
	if br.Sujet != "Fwd: Invitation" {
		t.Errorf("Sujet = %q", br.Sujet)
	}
	if !strings.Contains(br.Corps, "Message transféré") {
		t.Errorf("Corps = %q", br.Corps)
	}
	if len(br.To) != 0 {
		t.Errorf("To = %v, un transfert n'a personne en face", br.To)
	}
}
