package ui

import (
	"fmt"
	"strings"

	"github.com/Tahlasandale/HippopotamTUI/internal/cache"
	"github.com/Tahlasandale/HippopotamTUI/internal/config"
	"github.com/Tahlasandale/HippopotamTUI/internal/contacts"
	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	"github.com/Tahlasandale/HippopotamTUI/internal/redaction"
	"github.com/Tahlasandale/HippopotamTUI/internal/rfc822"
	tea "github.com/charmbracelet/bubbletea"
)

// Écran désigne la vue courante. Le TUI est un pager : on est dans la liste, on
// entre dans un message, on rédige, et quelques panneaux secondaires.
type Écran int

const (
	ÉcranListe Écran = iota
	ÉcranMessage
	ÉcranRédaction
	ÉcranFiltres
	ÉcranAide
	ÉcranJournal
	ÉcranChoix
)

func (e Écran) String() string {
	switch e {
	case ÉcranListe:
		return "liste"
	case ÉcranMessage:
		return "message"
	case ÉcranRédaction:
		return "rédaction"
	case ÉcranFiltres:
		return "filtres"
	case ÉcranAide:
		return "aide"
	case ÉcranJournal:
		return "journal"
	case ÉcranChoix:
		return "choix"
	}
	return "inconnu"
}

// Modèle est l'état unique du TUI. Tout ce qui est affiché en découle.
type Modèle struct {
	client  himalaya.Client
	cache   *cache.Store
	cfg     config.Config
	journal *Journal

	comptes []himalaya.Compte
	compte  string
	boîtes  []himalaya.Boite
	boîte   string

	enveloppes        []himalaya.Enveloppe
	curseur           int
	décalage          int
	page              int
	filtres           redaction.Filtres
	filtresAppliquées redaction.Filtres
	indexFiltre       int
	saisieFiltre      string

	message      *rfc822.Message
	messageID    string
	messageBoîte string
	lignes       []string
	défilement   int

	formulaire    *formulaire
	contacts      []contacts.Contact
	contactChoisi int

	écran       Écran
	choix       *choix
	erreur      string
	info        string
	occupé      bool
	attente     string
	progression string

	largeur, hauteur int
	premièreFois     bool
	démo             bool
	carnet           *contacts.Moissonneur
}

// Nouveau construit le modèle et démarre la découverte des comptes.
func Nouveau(client himalaya.Client, store *cache.Store, cfg config.Config, démo bool) *Modèle {
	m := &Modèle{
		client:       client,
		cache:        store,
		cfg:          cfg,
		démo:         démo,
		journal:      NouveauJournal(),
		page:         1,
		premièreFois: vraiPremièreFois(store),
	}
	if store != nil {
		m.carnet = &contacts.Moissonneur{Client: client, Cache: store}
	}
	return m
}

func vraiPremièreFois(store *cache.Store) bool {
	if store == nil {
		return true
	}
	// Un cache déjà peuplé signifie que le bandeau a déjà été montré.
	_, quand, ok := store.Enveloppes("proton", "Inbox", 1)
	return !ok || quand.IsZero()
}

// Init lance le chargement des comptes.
//
// L'horloge du journal n'est armée que lorsqu'un fichier de journal est
// configuré : sans fichier, elle ne tournerait que pour purger du vide, et les
// tests d'interface attendraient une minute à chaque lancement.
func (m *Modèle) Init() tea.Cmd {
	commandes := []tea.Cmd{m.chargerComptes()}
	if m.journal != nil && m.journal.fichier != "" {
		commandes = append(commandes, m.armerJournal())
	}
	return tea.Batch(commandes...)
}

// Update traite un message du programme.
func (m *Modèle) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.largeur, m.hauteur = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.auClavier(msg)

	case msgComptes:
		if msg.err != nil {
			m.échouer("Lecture des comptes", msg.err)
			return m, nil
		}
		m.comptes = msg.comptes
		nom, _ := himalaya.CompteParDéfaut(msg.comptes)
		if nom != "" {
			m.compte = nom
		}
		m.journal.Noter(fmt.Sprintf("comptes : %s", nomsDes(msg.comptes)))
		return m, tea.Batch(m.chargerBoîtes(), m.moissonnerCarnet())

	case msgBoîtes:
		if msg.err != nil {
			m.échouer("Lecture des boîtes", msg.err)
			return m, nil
		}
		m.boîtes = msg.boîtes
		if m.boîte == "" || m.indexDeBoîte(m.boîte) < 0 {
			m.boîte = boîteParDéfaut(msg.boîtes)
		}
		m.journal.Noter(fmt.Sprintf("boîtes de %s : %d", m.compte, len(msg.boîtes)))
		return m, m.chargerEnveloppes(false)

	case msgEnveloppes:
		m.occupé = false
		if msg.err != nil {
			m.échouer("Lecture des messages", msg.err)
			return m, nil
		}
		m.enveloppes = msg.enveloppes
		m.censureCurseur()
		m.journal.Noter(fmt.Sprintf("%s/%s : %d messages", m.compte, m.boîte, len(msg.enveloppes)))
		return m, nil

	case msgCorps:
		m.occupé = false
		if msg.err != nil {
			m.échouer("Lecture du message", msg.err)
			return m, tea.Batch(m.chargerEnveloppes(true))
		}
		m.message = msg.message
		m.lignes = rendreCorps(msg.message, m.largeur)
		m.défilement = 0
		m.écran = ÉcranMessage
		// Le drapeau part en arrière-plan : l'écran ne doit pas attendre le
		// réseau pour montrer le message qu'on vient d'ouvrir.
		m.marquerLuSansRafraîchir()
		return m, m.marquerLu()

	case msgBrouillon:
		m.occupé = false
		if msg.err != nil {
			m.échouer("Préparation du message", msg.err)
			m.formulaire = nil
			m.écran = ÉcranListe
			return m, nil
		}
		br := CompléterBrouillon(msg.brouillon, msg.origine, m.message)
		m.formulaire = nouveauFormulaire(br, msg.origine, m.contacts)
		m.écran = ÉcranRédaction
		return m, m.lancerÉditeur()

	case msgEnvoyé:
		m.occupé = false
		if msg.err != nil {
			m.échouer("Envoi", msg.err)
			m.journal.Noter("l'envoi a échoué : " + msg.err.Error())
			return m, m.lancerÉditeur()
		}
		sujet := ""
		if m.formulaire != nil {
			sujet = m.formulaire.Sujet
		}
		m.info = "Message envoyé."
		m.journal.Noter("message envoyé : " + sujet)
		m.formulaire = nil
		m.attente = ""
		m.écran = ÉcranListe
		return m, m.chargerEnveloppes(true)

	case msgBrouillonEnregistré:
		m.occupé = false
		if msg.err != nil {
			m.échouer("Brouillon", msg.err)
			return m, nil
		}
		m.info = "Brouillon enregistré."
		return m, nil

	case msgAction:
		m.occupé = false
		if msg.err != nil {
			m.échouer(msg.quoi, msg.err)
			m.journal.Noter(msg.quoi + " : échec, " + msg.err.Error())
			return m, nil
		}
		m.journal.Noter(msg.quoi + " : fait")
		if msg.info != "" {
			m.info = msg.info
		}
		if !msg.rafraîchir {
			return m, nil
		}
		return m, m.chargerEnveloppes(true)

	case msgCarnet:
		m.rechargerCarnet()
		m.journal.Noter(fmt.Sprintf("carnet : %d contacts", msg.n))
		return m, nil

	case tickJournalMsg:
		if m.journal == nil || m.journal.fichier == "" {
			return m, nil
		}
		return m, m.ticHorloge()

	case msgÉditée:
		if msg.err != nil {
			m.occupé = false
			m.annoncer(fmt.Sprintf("Éditeur : %v", msg.err))
			return m, nil
		}
		if m.formulaire != nil {
			m.formulaire.Corps = msg.texte
			m.formulaire.modifié = true
		}
		return m, nil
	}

	return m, nil
}

// ------------------------------------------------------------------ erreurs

func (m *Modèle) échouer(quoi string, err error) {
	m.occupé = false
	m.erreur = fmt.Sprintf("%s : %v", quoi, err)
	m.info = ""
	m.journal.Erreur(quoi + " : " + err.Error())
}

func (m *Modèle) annoncer(texte string) {
	m.info = texte
	m.erreur = ""
}

func nomsDes(comptes []himalaya.Compte) string {
	noms := make([]string, 0, len(comptes))
	for _, c := range comptes {
		noms = append(noms, c.Nom)
	}
	return strings.Join(noms, ", ")
}

func boîteParDéfaut(boîtes []himalaya.Boite) string {
	for _, b := range boîtes {
		if b.Role == "inbox" {
			return b.ID
		}
	}
	if len(boîtes) > 0 {
		return boîtes[0].ID
	}
	return ""
}

// censeureDuCurseur remet le curseur dans les bornes après un changement de
// boîte, de filtres ou de suppression.
func (m *Modèle) censureCurseur() {
	if m.curseur >= len(m.enveloppes) {
		m.curseur = len(m.enveloppes) - 1
	}
	if m.curseur < 0 {
		m.curseur = 0
	}
	m.censureDécalage()
}

// envelopeCourante rend l'enveloppe sous le curseur, ou nil.
func (m *Modèle) envelopeCourante() *himalaya.Enveloppe {
	if m.curseur < 0 || m.curseur >= len(m.enveloppes) {
		return nil
	}
	return &m.enveloppes[m.curseur]
}

// indexDeBoîte donne la position d'une boîte dans la liste.
func (m *Modèle) indexDeBoîte(id string) int {
	for i, b := range m.boîtes {
		if b.ID == id {
			return i
		}
	}
	return -1
}
