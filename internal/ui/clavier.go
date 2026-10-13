package ui

import (
	"fmt"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	tea "github.com/charmbracelet/bubbletea"
)

// Le clavier est réparti en deux familles : la navigation se fait aux flèches,
// parce qu'on la fait tout le temps et qu'elle doit être prévisible ; les
// actions se font sur des lettres, parce qu'on les fait peu et qu'il faut les
// retenir. Les raccourcis vim restent acceptés en secours : taper j pour descendre
// coûte moins cher que déplacer la main, et les habitués ne sont pas punis.
func (m *Modèle) auClavier(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Le panneau de filtres capte tout : on y tape du texte, y compris des
	// lettres qui sont des commandes ailleurs.
	if m.écran == ÉcranFiltres {
		if consommée, cmd := m.auClavierFiltres(msg); consommée {
			return m, cmd
		}
		return m, nil
	}

	// Aide, journal et sélecteur se referment tous de la même façon.
	if m.écran == ÉcranAide || m.écran == ÉcranJournal || m.écran == ÉcranChoix {
		if refermée, cmd := m.fermerPanneau(msg); refermée {
			return m, cmd
		}
	}

	switch m.écran {
	case ÉcranRédaction:
		return m.auClavierRédaction(msg)
	case ÉcranMessage:
		return m.auClavierMessage(msg)
	default:
		return m.auClavierListe(msg)
	}
}

// fermerPanneau traite les touches qui quittent un panneau secondaire. Elle
// renvoie vrai si la touche a été consommée, avec la commande qui en découle.
func (m *Modèle) fermerPanneau(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?":
		if m.écran == ÉcranChoix {
			m.choix = nil
		}
		m.écran = écranSous(m.écran)
		return true, nil

	case "enter":
		if m.écran == ÉcranChoix {
			return true, m.confirmerChoix()
		}
		m.écran = écranSous(m.écran)
		return true, nil

	case "j", "k", "up", "down":
		if m.écran == ÉcranChoix && m.choix != nil {
			m.choix.décaler(signeFlèche(msg.String()))
			return true, nil
		}
	}
	return false, nil
}

func écranSous(e Écran) Écran {
	if e == ÉcranMessage {
		return ÉcranListe
	}
	return ÉcranListe
}

// signeFlèche donne le sens d'une touche de déplacement. Les noms techniques
// ("up", "down") et les glyphes sont acceptés côte à côte : selon la façon dont le
// terminal les transmet, on reçoit l'un ou l'autre.
func signeFlèche(touche string) int {
	switch touche {
	case "down", "↓", "j", "ctrl+n":
		return 1
	case "up", "↑", "k", "ctrl+p":
		return -1
	case "right", "l", "tab":
		return 1
	case "left", "h", "shift+tab":
		return -1
	}
	return 0
}

// ------------------------------------------------------------------ liste

func (m *Modèle) auClavierListe(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		m.allerÀ(m.curseur - 1)
		return m, nil

	case "down", "j":
		m.allerÀ(m.curseur + 1)
		return m, nil

	case "pgup", "ctrl+b":
		m.allerÀ(m.curseur - m.lignesVisibles())
		return m, nil

	case "pgdown", "ctrl+f", " ":
		m.allerÀ(m.curseur + m.lignesVisibles())
		return m, nil

	case "home", "g":
		m.allerÀ(0)
		return m, nil

	case "end", "G":
		m.allerÀ(len(m.enveloppes) - 1)
		return m, nil

	case "left", "h":
		m.boîtePrécédente()
		return m, m.chargerEnveloppes(false)

	case "right", "l":
		m.boîteSuivante()
		return m, m.chargerEnveloppes(false)

	case "enter":
		return m, m.ouvrirMessage()

	case "M", "ctrl+n":
		m.ouvrirMessageSuivant()
		return m, m.ouvrirMessage()

	case "v":
		return m, m.basculerDrapeau(himalaya.PoussièreVolee, !m.courantLu())

	case "F":
		return m, m.basculerDrapeau(himalaya.Marqué, !m.courantMarqué())

	case "d", "delete":
		return m, m.supprimerMessage()

	case "s":
		m.choix = nouveauChoix("Déplacer vers", m.nomsBoîtes(), m.indexDeBoîte(m.boîte), func(i int) tea.Cmd {
			if i < 0 || i >= len(m.boîtes) {
				return nil
			}
			return m.déplacerMessage(m.boîtes[i].ID)
		})
		m.écran = ÉcranChoix
		return m, nil

	case "m":
		m.choix = nouveauChoix("Boîte", m.nomsBoîtes(), m.indexDeBoîte(m.boîte), func(i int) tea.Cmd {
			if i < 0 || i >= len(m.boîtes) {
				return nil
			}
			return m.changerBoîte(m.boîtes[i].ID)
		})
		m.écran = ÉcranChoix
		return m, nil

	case "S":
		noms := []string{}
		for _, c := range m.comptes {
			noms = append(noms, c.Nom)
		}
		m.choix = nouveauChoix("Compte", noms, indexDe(noms, m.compte), func(i int) tea.Cmd {
			if i < 0 || i >= len(m.comptes) {
				return nil
			}
			return m.changerCompte(m.comptes[i].Nom)
		})
		m.écran = ÉcranChoix
		return m, nil

	case "c":
		return m, m.préparerBrouillon(himalaya.Origine{Kind: himalaya.Nouveau})

	case "r":
		return m, m.préparerRéponse(himalaya.Reponse)

	case "a":
		return m, m.préparerRéponse(himalaya.ReponseTout)

	case "t":
		return m, m.préparerRéponse(himalaya.Transfert)

	case "/":
		m.filtresAppliquées = m.filtres
		m.indexFiltre = 0
		m.préparerFiltre()
		m.écran = ÉcranFiltres
		m.info = ""
		m.erreur = ""
		return m, nil

	case ":":
		m.écran = ÉcranAide
		return m, nil

	case "R":
		return m, m.chargerEnveloppes(true)

	case "?":
		m.écran = ÉcranAide
		return m, nil

	case "J":
		m.écran = ÉcranJournal
		return m, nil
	}
	return m, nil
}

func (m *Modèle) lignesVisibles() int {
	n := m.hauteur - 6
	if n < 3 {
		n = 3
	}
	return n
}

func (m *Modèle) allerÀ(index int) {
	if len(m.enveloppes) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.enveloppes) {
		index = len(m.enveloppes) - 1
	}
	m.curseur = index
	m.censureDécalage()
	m.erreur = ""
}

func (m *Modèle) censureDécalage() {
	visibles := m.lignesVisibles()
	if m.curseur < m.décalage {
		m.décalage = m.curseur
	}
	if m.curseur >= m.décalage+visibles {
		m.décalage = m.curseur - visibles + 1
	}
	if m.décalage < 0 {
		m.décalage = 0
	}
}

func (m *Modèle) boîtePrécédente() {
	i := m.indexDeBoîte(m.boîte)
	if i > 0 {
		m.boîte = m.boîtes[i-1].ID
		m.reinitialiser()
	}
}

func (m *Modèle) boîteSuivante() {
	i := m.indexDeBoîte(m.boîte)
	if i >= 0 && i < len(m.boîtes)-1 {
		m.boîte = m.boîtes[i+1].ID
		m.reinitialiser()
	}
}

func (m *Modèle) reinitialiser() {
	m.page = 1
	m.curseur = 0
	m.décalage = 0
}

func (m *Modèle) nomsBoîtes() []string {
	noms := make([]string, 0, len(m.boîtes))
	for _, b := range m.boîtes {
		noms = append(noms, b.Nom)
	}
	return noms
}

func indexDe(liste []string, valeur string) int {
	for i, item := range liste {
		if item == valeur {
			return i
		}
	}
	return 0
}

func (m *Modèle) courantLu() bool {
	env := m.envelopeCourante()
	return env == nil || env.A(himalaya.PoussièreVolee)
}

func (m *Modèle) courantMarqué() bool {
	env := m.envelopeCourante()
	return env == nil || env.A(himalaya.Marqué)
}

// ouvrirMessageSuivant ouvre le message suivant sans repasser par la liste : c'est
// le geste de lecture continue, message après message.
func (m *Modèle) ouvrirMessageSuivant() {
	if m.curseur+1 < len(m.enveloppes) {
		m.curseur++
		m.censureDécalage()
	}
}

// préparerRéponse enrichit la demande d'un « répondre à tous » : les
// destinataires sont recalculés à partir du message affiché, pas de celui que
// himalaya déduirait de ses seuls en-têtes.
func (m *Modèle) préparerRéponse(genre himalaya.Genre) tea.Cmd {
	env := m.envelopeCourante()
	if env == nil {
		return nil
	}
	origine := himalaya.Origine{Kind: genre, ID: env.ID}

	if genre == himalaya.Transfert {
		return m.préparerBrouillon(origine)
	}
	return m.préparerBrouillon(origine)
}

// ------------------------------------------------------------------ message

func (m *Modèle) auClavierMessage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "q", "esc":
		m.écran = ÉcranListe
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		m.défiler(-1)
		return m, nil

	case "down", "j":
		m.défiler(1)
		return m, nil

	case "pgup", "ctrl+b":
		m.défiler(-m.lignesVisibles())
		return m, nil

	case "pgdown", "ctrl+f":
		m.défiler(m.lignesVisibles())
		return m, nil

	case "home", "g":
		m.défilement = 0
		return m, nil

	case "end", "G":
		m.défilement = max0(len(m.lignes) - m.lignesVisibles())
		return m, nil

	case "left", "h":
		m.allerÀ(m.curseur - 1)
		return m, m.ouvrirMessage()

	case "right", "l":
		m.allerÀ(m.curseur + 1)
		return m, m.ouvrirMessage()

	case "enter", "n", "M":
		m.ouvrirMessageSuivant()
		return m, m.ouvrirMessage()

	case "v":
		return m, m.basculerDrapeau(himalaya.PoussièreVolee, !m.courantLu())

	case "F":
		return m, m.basculerDrapeau(himalaya.Marqué, !m.courantMarqué())

	case "d", "delete":
		m.écran = ÉcranListe
		return m, m.supprimerMessage()

	case "s":
		m.choix = nouveauChoix("Déplacer vers", m.nomsBoîtes(), m.indexDeBoîte(m.boîte), func(i int) tea.Cmd {
			if i < 0 || i >= len(m.boîtes) {
				return nil
			}
			return m.déplacerMessage(m.boîtes[i].ID)
		})
		m.écran = ÉcranChoix
		return m, nil

	case "r":
		return m, m.préparerRéponse(himalaya.Reponse)

	case "a":
		return m, m.préparerRéponse(himalaya.ReponseTout)

	case "t":
		return m, m.préparerRéponse(himalaya.Transfert)

	case "c":
		return m, m.préparerBrouillon(himalaya.Origine{Kind: himalaya.Nouveau})

	case "ctrl+s":
		m.info = "Corps copié dans le presse-papiers s'il y en a un, sinon affiché dans le journal."
		m.journal.Noter("corps du message " + m.messageID)
		return m, nil

	case "R":
		return m, m.ouvrirMessage()

	case "?":
		m.écran = ÉcranAide
		return m, nil

	case "J":
		m.écran = ÉcranJournal
		return m, nil
	}
	return m, nil
}

func (m *Modèle) défiler(delta int) {
	m.défilement += delta
	if m.défilement < 0 {
		m.défilement = 0
	}
	if m.défilement > max0(len(m.lignes)-m.lignesVisibles()) {
		m.défilement = max0(len(m.lignes) - m.lignesVisibles())
	}
}

// ------------------------------------------------------------------ rédaction

func (m *Modèle) auClavierRédaction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.formulaire
	if f == nil {
		m.écran = ÉcranListe
		return m, nil
	}

	switch msg.String() {

	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		return m.quitterRédaction()

	case "ctrl+s":
		f.valider()
		if !f.valide() {
			m.erreur = "Il faut au moins un destinataire et un objet."
			return m, nil
		}
		m.erreur = ""
		m.occupé = true
		m.attente = "envoi"
		return m, m.envoyer()

	case "ctrl+r":
		f.valider()
		return m, m.lancerÉditeur()

	case "tab", "down":
		f.valider()
		f.entrerChamp(f.ChampSuivant())
		return m, nil

	case "shift+tab", "up":
		f.valider()
		f.entrerChamp(f.ChampPrécédent())
		return m, nil

	case "left", "right":
		// Les flèches latérales servent à choisir l'identité dexpédition, et
		// seulement cela : sur un champ texte, elles ne doivent rien faire, sinon
		// un destinataire saisi au clavier se retrouve coupé en deux.
		if f.champ != ChampDe {
			return m, nil
		}
		if f.changerIdentité(signeFlèche(msg.String())) {
			m.annoncer(fmt.Sprintf("Part de %s.", f.De))
		}
		return m, nil

	case "enter":
		if f.accepterProposition() {
			return m, nil
		}
		f.valider()
		return m, nil

	case "backspace":
		f.saisie = trimUneRune(f.saisie)
		f.actualiserPropositions()
		return m, nil

	case "ctrl+n", "ctrl+p":
		if len(f.proposés) > 0 {
			f.choisi += signeFlèche(msg.String())
			if f.choisi < 0 {
				f.choisi = len(f.proposés) - 1
			}
			if f.choisi >= len(f.proposés) {
				f.choisi = 0
			}
		}
		return m, nil
	}

	// Le champ De se choisit, il ne se tape pas : y écrire n'aurait aucun
	// effet, et une adresse saisie à la main sans compte ne partirait jamais.
	if msg.Type == tea.KeyRunes && f.champ != ChampDe {
		f.saisie += msg.String()
		f.modifié = true
		f.actualiserPropositions()
	}
	return m, nil
}

// quitterRédaction abandonne la rédaction. Un formulaire non vide part en
// brouillon : c'est le filet quand on ferme le laptop au milieu d'une phrase. Un
// formulaire vidé n'enregistre rien, pour ne pas laisser de coquilles.
func (m *Modèle) quitterRédaction() (tea.Model, tea.Cmd) {
	f := m.formulaire
	m.formulaire = nil
	m.écran = ÉcranListe

	if f == nil || f.vide() {
		m.info = "Rédaction abandonnée."
		return m, m.chargerEnveloppes(true)
	}
	m.info = "Rédaction enregistrée en brouillon."
	m.journal.Noter("brouillon conservé : " + f.Sujet)
	return m, tea.Batch(m.enregistrerBrouillon(), m.chargerEnveloppes(true))
}

func trimUneRune(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}
