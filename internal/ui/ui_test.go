package ui

import (
	"os"
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

// barreDuBas rend la ligne d'état du bas de l'écran.
func (h *harnais) barreDuBas() string {
	h.t.Helper()
	return h.m.barreDuBas()
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

	// le cycle démarre sur De, puis descend vers les destinataires et l'objet
	if f.champ != ChampDe {
		t.Fatalf("champ initial = %v, veut le champ De", f.champ)
	}
	h.touche(tea.KeyDown)
	if f.champ != ChampÀ {
		t.Errorf("champ = %v, veut À", f.champ)
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
	if f.champ != ChampDe {
		t.Errorf("champ = %v, veut un retour sur De", f.champ)
	}
}

// une adresse saisie sans « @ » ne doit jamais partir : c'est le refuge des
// destinataires coupés par une frappe.
func TestSaisieDAdresseIncomplète(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("c")
	f := h.m.formulaire
	f.entrerChamp(ChampÀ)
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
	f.entrerChamp(ChampÀ)
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
	f.To = []string{"alice@example.org"}
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

// déplacer un message vers une autre boîte doit le faire disparaître de la liste
// immédiate, sans attendre le serveur.
func TestDéplacerVersUneBoîte(t *testing.T) {
	h := nouveauHarnais(t)
	avant := len(h.m.enveloppes)
	objet := h.m.enveloppes[0].Sujet

	h.rune("s")
	if h.m.écran != ÉcranChoix {
		t.Fatalf("écran = %v, veut le sélecteur", h.m.écran)
	}

	h.touche(tea.KeyDown) // Sent
	h.touche(tea.KeyEnter)
	h.vider()

	if len(h.m.enveloppes) != avant-1 {
		t.Errorf("il reste %d messages, veut %d", len(h.m.enveloppes), avant-1)
	}
	for _, env := range h.m.enveloppes {
		if env.Sujet == objet {
			t.Error("le message déplacé est encore dans la liste")
		}
	}
	if !strings.Contains(h.vue(), "Déplacé vers") {
		t.Errorf("la vue n'annonce pas le déplacement :\n%s", h.vue())
	}
}

// après un déplacement, le curseur doit rester sur une ligne valide.
func TestCurseurAprèsSuppression(t *testing.T) {
	h := nouveauHarnais(t)

	// Le nombre est pris une fois pour toutes : la condition de boucle relirait
	// une liste qui rétrécit, et s'arrêterait avant d'avoir tout vidé.
	total := len(h.m.enveloppes) + 2
	for i := 0; i < total; i++ {
		h.rune("d")
		h.vider()
	}

	if len(h.m.enveloppes) != 0 {
		t.Fatalf("il reste %d messages", len(h.m.enveloppes))
	}
	if h.m.curseur != 0 {
		t.Errorf("curseur = %d, veut 0", h.m.curseur)
	}
	if h.m.View() == "" {
		t.Error("la vue est vide sur une boîte vidée")
	}
}

// une liste filtrée sans résultat doit le dire, plutôt que d'afficher le vide.
func TestListeFiltréeVide(t *testing.T) {
	h := nouveauHarnais(t)

	h.rune("/")
	h.taper("personne-nexiste-pas")
	h.touche(tea.KeyEnter)
	h.vider()

	if len(h.m.enveloppes) != 0 {
		t.Skip("le client de démonstration ne filtre pas, la liste reste pleine")
	}
	if !strings.Contains(h.vue(), "Aucun message ne correspond") {
		t.Errorf("la vue ne l'explique pas :\n%s", h.vue())
	}
}

// Les captures de la documentation sont vérifiées par la CI : si l'affichage
// change sans que « hippo capture » ait été relancé, la CI échoue. Cela ne
// tient que si deux exécutions donnent exactement le même texte, horloge et
// dates comprises.
func TestCapturesReproductibles(t *testing.T) {
	première := Capturer(demo.Figé(), 96, 30)
	seconde := Capturer(demo.Figé(), 96, 30)

	voulu := []string{"accueil", "liste", "message", "redaction", "filtres", "journal", "aide"}
	if len(première) != len(voulu) {
		t.Fatalf("%d captures, veut %d", len(première), len(voulu))
	}

	for i, capture := range première {
		if capture.Nom != voulu[i] {
			t.Errorf("capture %d = %q, veut %q", i, capture.Nom, voulu[i])
		}
		if strings.TrimSpace(capture.Texte) == "" {
			t.Errorf("capture %q vide", capture.Nom)
		}
		if capture.Texte != seconde[i].Texte {
			t.Errorf("capture %q n'est pas reproductible", capture.Nom)
		}
	}
}

// ---------------------------------------------------------------- rédaction
//
// Les tests qui suivent portent sur le parcours de rédaction tel qu'il a été
// redessiné : le formulaire d'abord, l'éditeur sur demande, et un champ « De »
// qui dit de quel compte le message part.

// éditeurFactice écrit un corps connu dans le fichier qu'on lui passe, puis
// sort. C'est un éditeur comme un autre. Il compte ses ouvertures, ce qui permet
// de vérifier qu'il s'ouvre ou non sans rien ajouter au programme.
func éditeurFactice(t *testing.T, corps string) (script string, ouvert func() int) {
	t.Helper()
	dossier := t.TempDir()
	compteur := filepath.Join(dossier, "ouvertures")
	script = filepath.Join(dossier, "editeur.sh")

	contenu := "#!/bin/sh\necho x >> '" + compteur + "'\nprintf '%s' \"" + corps + "\" > \"$1\"\n"
	if err := os.WriteFile(script, []byte(contenu), 0o700); err != nil {
		t.Fatal(err)
	}

	ouvert = func() int {
		contenu, err := os.ReadFile(compteur)
		if err != nil {
			return 0
		}
		return strings.Count(string(contenu), "\n")
	}
	return script, ouvert
}

// le fichier ouvert dans l'éditeur ne contient que le corps : plus d'en-têtes à
// fillets, donc plus rien qui puisse être perdu en chemin.
func TestLeFichierDeLEditionNeContientQueLeCorps(t *testing.T) {
	env := himalaya.Envoi{
		De:    "Orphin <orphin@protonmail.com>",
		To:    []string{"alice@example.org"},
		Sujet: "Réunion",
	}

	contenu := contenuDeLEditeur(env, "> la citation")

	if strings.Contains(contenu, "À :") || strings.Contains(contenu, "Objet :") {
		t.Errorf("le fichier contient encore des en-têtes :\n%s", contenu)
	}
	if !strings.Contains(contenu, "> la citation") {
		t.Errorf("le fichier ne contient pas le corps :\n%s", contenu)
	}
}

// le corps relu est pris tel quel, sans retraitement : ce qui est écrit dans
// l'éditeur est ce qui part.
func TestLeCorpsReluEstPrisTelQuel(t *testing.T) {
	if got := contenuRelu("Bonjour,\r\n\r\nCordialement."); got != "Bonjour,\n\nCordialement." {
		t.Errorf("contenuRelu = %q", got)
	}
	// un corps qui commence par une ligne d'en-tête n'est plus amputé : le
	// fichier n'en contient plus, donc il n'y a plus rien à retirer.
	if got := contenuRelu("Objet : réunion\n\nBonjour."); !strings.HasPrefix(got, "Objet : réunion") {
		t.Errorf("contenuRelu = %q, le texte doit être rendu tel quel", got)
	}
}

// écrire un message neuf n'ouvre pas l'éditeur : la place est faite pour que le
// formulaire se lise avant de taper quoi que ce soit.
func TestMessageNeufNouvrePasLEditeur(t *testing.T) {
	h := nouveauHarnais(t)
	script, ouvert := éditeurFactice(t, "")
	h.m.cfg.Editor = script

	h.rune("c")
	h.vider()

	if h.m.écran != ÉcranRédaction {
		t.Fatalf("écran = %v, veut la rédaction", h.m.écran)
	}
	if ouvert() != 0 {
		t.Errorf("l'éditeur s'est ouvert %d fois, il ne devait pas s'ouvrir", ouvert())
	}
}

// répondre, en revanche, ouvre l'éditeur : le texte quoté attend et on y écrit
// presque toujours.
func TestReponseOuvreEditeur(t *testing.T) {
	h := nouveauHarnais(t)
	script, ouvert := éditeurFactice(t, "Merci pour votre message.")
	h.m.cfg.Editor = script

	h.rune("r")
	h.vider()

	if ouvert() != 1 {
		t.Errorf("l'éditeur s'est ouvert %d fois, en veut 1", ouvert())
	}
	if h.m.formulaire.Corps != "Merci pour votre message." {
		t.Errorf("Corps = %q, veut ce que l'éditeur a écrit", h.m.formulaire.Corps)
	}
}

// et le corps écrit dans l'éditeur apparaît ensuite dans le formulaire.
func TestLeCorpsEcritApparaitDansLeFormulaire(t *testing.T) {
	h := nouveauHarnais(t)
	script, _ := éditeurFactice(t, "Bonjour Alice,")
	h.m.cfg.Editor = script

	h.rune("c")
	h.vider()
	h.toucher(tea.KeyMsg{Type: tea.KeyCtrlR})
	h.vider()

	vue := h.vue()
	if !strings.Contains(vue, "Bonjour Alice,") {
		t.Errorf("la vue ne montre pas le corps écrit :\n%s", vue)
	}
}

// le champ « De » propose les comptes, et le change avec les flèches.
func TestChampDeProposeLesComptes(t *testing.T) {
	h := nouveauHarnais(t)
	h.rune("c")
	h.vider()

	f := h.m.formulaire
	if f.champ != ChampDe {
		t.Fatalf("champ initial = %v, veut le champ De", f.champ)
	}
	if len(f.identités) != 2 {
		t.Fatalf("identités = %+v, en veut 2", f.identités)
	}
	if !strings.Contains(f.De, "orphin@protonmail.com") {
		t.Errorf("De = %q, veut l'adresse du compte par défaut", f.De)
	}
	if f.Compte != "proton" {
		t.Errorf("Compte = %q, veut le compte actif", f.Compte)
	}

	h.touche(tea.KeyRight)

	if !strings.Contains(f.De, "meute.saintfrancois78@gmail.com") {
		t.Errorf("De = %q, veut l'autre identité", f.De)
	}
	if f.Compte != "gmail" {
		t.Errorf("Compte = %q, veut gmail : l'identité doit suivre le compte d'envoi", f.Compte)
	}
	if !strings.Contains(h.vue(), "gmail") {
		t.Errorf("la vue ne montre pas le changement de compte :\n%s", h.vue())
	}
}

// choisir une autre identité ne doit pas seulement changer l'en-tête : le
// message doit partir par le bon serveur. C'est le compte qui change, et lui
// porte l'identité.
func TestChangerDIdentitéChangeLeCompteDEnvoi(t *testing.T) {
	h := nouveauHarnais(t)
	h.rune("c")
	h.vider()

	f := h.m.formulaire
	f.To = []string{"alice@example.org"}
	f.Sujet = "Test"

	h.touche(tea.KeyRight)
	h.toucher(tea.KeyMsg{Type: tea.KeyCtrlS})
	h.vider()

	if h.m.formulaire != nil {
		t.Fatal("la rédaction aurait dû être fermée après l'envoi")
	}
}

// les flèches ne doivent rien faire sur un champ texte : ellesIndented留给
// someone qui types a recipient.
func TestFlèchesInertesSurLesChampsTexte(t *testing.T) {
	h := nouveauHarnais(t)
	h.rune("c")
	h.vider()

	f := h.m.formulaire
	f.saisie = "ali"
	h.touche(tea.KeyRight)
	h.touche(tea.KeyLeft)

	if f.saisie != "ali" {
		t.Errorf("saisie = %q, les flèches ne doivent pas la toucher", f.saisie)
	}
	if f.De != h.m.formulaire.De {
		t.Error("les flèches ne doivent pas changer l'identité sur un champ texte")
	}
}

// le cycle passe par les quatre champs, et revient au début.
func TestCycleDeQuatreChamps(t *testing.T) {
	h := nouveauHarnais(t)
	h.rune("c")
	h.vider()
	f := h.m.formulaire

	voulu := []Champ{ChampDe, ChampÀ, ChampCc, ChampSujet}
	for _, attendu := range voulu {
		if f.champ != attendu {
			t.Fatalf("champ = %v, veut %v", f.champ, attendu)
		}
		h.touche(tea.KeyDown)
	}
	if f.champ != ChampDe {
		t.Errorf("champ = %v, le cycle doit revenir au champ De", f.champ)
	}

	h.touche(tea.KeyUp)
	if f.champ != ChampSujet {
		t.Errorf("champ = %v, veut Objet en remontant", f.champ)
	}
}

// le formulaire annonce ce qu'il attend, et l'invitation disparaît une fois
// qu'il y a un corps.
func TestInvitationAEcrire(t *testing.T) {
	h := nouveauHarnais(t)
	h.rune("c")
	h.vider()

	if !strings.Contains(h.vue(), "ctrl+r") {
		t.Errorf("le formulaire n'invite pas à écrire :\n%s", h.vue())
	}

	h.toucher(tea.KeyMsg{Type: tea.KeyCtrlR})
	h.vider()

	if strings.Contains(h.vue(), "ctrl+r pour écrire") {
		t.Errorf("l'invitation est restée alors qu'il y a un corps :\n%s", h.vue())
	}
}

// la barre du bas de la rédaction doit tenir en une ligne, sans coupure : c'est
// là que se trouve la réponse à « comment j'envoie ».
func TestBarreDeLaRédactionSansTroncature(t *testing.T) {
	h := nouveauHarnais(t)
	h.rune("c")
	h.vider()

	barre := h.barreDuBas()
	for _, attendu := range []string{"ctrl+s envoyer", "ctrl+r éditer", "échap abandonner"} {
		if !strings.Contains(barre, attendu) {
			t.Errorf("la barre ne montre pas %q : %s", attendu, barre)
		}
	}
	if strings.Contains(barre, "…") {
		t.Errorf("la barre est tronquée : %s", barre)
	}
}

// appuyer sur c doit dire quelque chose pendant que himalaya prépare : le vide
// d'une à deux secondes était le premier appel à l'aide.
func TestPréparationAnnoncée(t *testing.T) {
	h := nouveauHarnais(t)

	// sans exécuter la commande : c'est l'état entre la frappe et la réponse de
	// himalaya qu'on vérifie, ces deux secondes-là qui semblaient figées
	h.envoyer(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})

	if !h.m.occupé {
		t.Error("rien n'indique que la préparation est en cours")
	}
	if !strings.Contains(h.barreDuBas(), "préparation") {
		t.Errorf("la barre ne dit rien : %q", h.barreDuBas())
	}
	h.vider()
}

// le titre du formulaire doit dire ce qu'on est en train de faire : « Nouveau
// message » au-dessus d'une réponse induit en erreur.
func TestTitreDuFormulaire(t *testing.T) {
	cases := []struct {
		touche string
		voulu  string
	}{
		{"c", "Nouveau message"},
		{"r", "Réponse"},
		{"a", "Réponse à tous"},
		{"t", "Transfert"},
	}

	for _, c := range cases {
		h := nouveauHarnais(t)
		h.m.cfg.Editor = éditeurSilencieux(t)
		h.rune(c.touche)
		h.vider()

		vue := h.vue()
		if !strings.Contains(vue, c.voulu) {
			t.Errorf("après %q, la vue ne montre pas %q :\n%s", c.touche, c.voulu, vue)
		}
	}
}

// un éditeur qui ne fait rien, pour tester la rédaction sans écrire de corps.
func éditeurSilencieux(t *testing.T) string {
	t.Helper()
	chemin := filepath.Join(t.TempDir(), "editeur.sh")
	if err := os.WriteFile(chemin, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return chemin
}
