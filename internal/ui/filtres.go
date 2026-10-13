package ui

import (
	"strings"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/redaction"
	tea "github.com/charmbracelet/bubbletea"
)

// Le constructeur de filtres est délibérément guidé : six critères combinés par
// « et », tous compréhensibles sans connaître le langage de himalaya. La requête
// réellement envoyée est affichée en dessous, pour qu'on puisse vérifier ce qui
// part avant de valider.

type critère struct {
	clé   string
	texte string
}

var critèresFiltres = []critère{
	{clé: "de", texte: "De"},
	{clé: "objet", texte: "Objet"},
	{clé: "corps", texte: "Corps"},
	{clé: "depuis", texte: "Depuis le"},
	{clé: "avant", texte: "Avant le"},
	{clé: "nonlus", texte: "Non lus"},
}

func valeurCritère(fs redaction.Filtres, clef string) string {
	switch clef {
	case "de":
		return fs.De
	case "objet":
		return fs.Objet
	case "corps":
		return fs.Corps
	case "depuis":
		if fs.Depuis.IsZero() {
			return ""
		}
		return fs.Depuis.Format("02/01/2006")
	case "avant":
		if fs.Avant.IsZero() {
			return ""
		}
		return fs.Avant.Format("02/01/2006")
	case "nonlus":
		if fs.NonLusSeuls {
			return "oui"
		}
		return "non"
	}
	return ""
}

func poserCritère(fs *redaction.Filtres, clef, valeur string) {
	switch clef {
	case "de":
		fs.De = valeur
	case "objet":
		fs.Objet = valeur
	case "corps":
		fs.Corps = valeur
	case "depuis":
		fs.Depuis = lireDate(valeur)
	case "avant":
		fs.Avant = lireDate(valeur)
	case "nonlus":
		fs.NonLusSeuls = valeur == "oui"
	}
}

// estDate signale les critères qui demandent une date et non du texte.
func estDate(clef string) bool { return clef == "depuis" || clef == "avant" }

// estBascule signale le critère booléen, qui se coche et se décoche.
func estBascule(clef string) bool { return clef == "nonlus" }

// lireDate accepte jj/mm/aaaa et aaaa-mm-jj. Une date illisible ne vaut pas zéro
// janvier : elle est simplement ignorée, pour ne pas chercher un message écrit
// le 1er janvier de l'an 1.
func lireDate(saisie string) time.Time {
	saisie = strings.TrimSpace(saisie)
	for _, format := range []string{"02/01/2006", "2006-01-02", "2/1/2006"} {
		if t, err := time.Parse(format, saisie); err == nil {
			return t
		}
	}
	return time.Time{}
}

// déplacerDate fait glisser un critère de date d'un mois, ce qui rend le
// réglage possible au clavier sans taper une date entière.
func déplacerDate(fs *redaction.Filtres, clef string, mois int) {
	var base time.Time
	switch clef {
	case "depuis":
		base = fs.Depuis
		if base.IsZero() {
			base = time.Now()
		}
	case "avant":
		base = fs.Avant
		if base.IsZero() {
			base = time.Now()
		}
	default:
		return
	}
	poserDate(fs, clef, base.AddDate(0, mois, 0))
}

// poserDate écrit une date dans le critère désigné.
func poserDate(fs *redaction.Filtres, clef string, quand time.Time) {
	switch clef {
	case "depuis":
		fs.Depuis = quand
	case "avant":
		fs.Avant = quand
	}
}

// auClavierFiltres traite le panneau de filtres. Il renvoie vrai si la touche a
// été consommée ici.
func (m *Modèle) auClavierFiltres(msg tea.KeyMsg) (bool, tea.Cmd) {
	if m.écran != ÉcranFiltres {
		return false, nil
	}

	critèreCourant := critèresFiltres[m.indexFiltre]

	switch msg.String() {

	case "down":
		m.validerFiltre()
		m.indexFiltre = (m.indexFiltre + 1) % len(critèresFiltres)
		m.préparerFiltre()
		return true, nil

	case "up":
		m.validerFiltre()
		m.indexFiltre = (m.indexFiltre + len(critèresFiltres) - 1) % len(critèresFiltres)
		m.préparerFiltre()
		return true, nil

	case "left":
		if estBascule(critèreCourant.clé) {
			m.validerFiltre()
			if m.filtres.NonLusSeuls {
				m.filtres.NonLusSeuls = false
			} else {
				m.filtres.NonLusSeuls = true
			}
			m.saisieFiltre = ""
		} else if estDate(critèreCourant.clé) {
			m.déplacerDate(-1)
		}
		return true, nil

	case "right":
		if estBascule(critèreCourant.clé) {
			m.validerFiltre()
			if m.filtres.NonLusSeuls {
				m.filtres.NonLusSeuls = false
			} else {
				m.filtres.NonLusSeuls = true
			}
			m.saisieFiltre = ""
		} else if estDate(critèreCourant.clé) {
			m.déplacerDate(1)
		}
		return true, nil

	case " ", "space":
		if estBascule(critèreCourant.clé) {
			m.validerFiltre()
			m.filtres.NonLusSeuls = !m.filtres.NonLusSeuls
		}
		return true, nil

	case "backspace":
		m.saisieFiltre = trimUneRune(m.saisieFiltre)
		return true, nil

	case "u":
		// « u » vide le critère courant : c'est le raccourci pour repartir de
		// zéro sans effacer les quatre autres.
		m.saisieFiltre = ""
		m.validerFiltre()
		return true, nil

	case "enter":
		m.validerFiltre()
		m.filtresAppliquées = m.filtres
		m.saisieFiltre = ""
		m.écran = ÉcranListe
		m.page = 1
		m.curseur = 0
		m.décalage = 0
		return true, m.chargerEnveloppes(true)

	case "esc":
		m.annulerFiltres()
		return true, m.chargerEnveloppes(true)
	}

	if msg.Type == tea.KeyRunes && !estBascule(critèreCourant.clé) {
		m.saisieFiltre += msg.String()
		m.validerFiltre()
		return true, nil
	}
	return false, nil
}

// déplacerDate applique le glissement au critère courant.
func (m *Modèle) déplacerDate(mois int) {
	m.validerFiltre()
	déplacerDate(&m.filtres, critèresFiltres[m.indexFiltre].clé, mois)
	m.saisieFiltre = valeurCritère(m.filtres, critèresFiltres[m.indexFiltre].clé)
}

// préparerFiltre charge la valeur du critère dans la saisie, pour qu'on puisse
// la corriger caractère par caractère.
func (m *Modèle) préparerFiltre() {
	m.saisieFiltre = valeurCritère(m.filtres, critèresFiltres[m.indexFiltre].clé)
	if critèresFiltres[m.indexFiltre].clé == "nonlus" {
		m.saisieFiltre = ""
	}
}

// validerFiltre écrit la saisie dans le filtre correspondant.
func (m *Modèle) validerFiltre() {
	clef := critèresFiltres[m.indexFiltre].clé
	if estBascule(clef) {
		return
	}
	poserCritère(&m.filtres, clef, strings.TrimSpace(m.saisieFiltre))
}

// annulerFiltres abandonne le panneau sans toucher à la recherche en cours.
func (m *Modèle) annulerFiltres() {
	m.filtres = m.filtresAppliquées
	m.saisieFiltre = ""
	m.écran = écranSous(ÉcranFiltres)
}
