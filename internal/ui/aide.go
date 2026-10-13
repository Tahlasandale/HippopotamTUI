package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// aide est l'ensemble des raccourcis documentés. C'est la seule source de vérité :
// la barre d'état, cette page et docs/raccourcis.md disent la même chose.
type raccourci struct {
	touches []string
	effet   string
}

var aideListe = []raccourci{
	{[]string{"↑", "k"}, "ligne précédente"},
	{[]string{"↓", "j"}, "ligne suivante"},
	{[]string{"←", "→"}, "boîte précédente ou suivante"},
	{[]string{"début", "fin"}, "première ou dernière ligne"},
	{[]string{"pg↑", "pg↓"}, "écran précédent ou suivant"},
	{[]string{"entrée"}, "ouvrir le message"},
	{[]string{"M", "entrée"}, "message suivant sans repasser par la liste"},
	{[]string{"v"}, "marquer lu ou non lu"},
	{[]string{"F"}, "marquer ou démarquer"},
	{[]string{"d"}, "mettre à la corbeille"},
	{[]string{"s"}, "déplacer vers une boîte"},
	{[]string{"m"}, "changer de boîte"},
	{[]string{"S"}, "changer de compte"},
	{[]string{"c"}, "écrire un nouveau message"},
	{[]string{"r"}, "répondre"},
	{[]string{"a"}, "répondre à tous"},
	{[]string{"t"}, "transférer"},
	{[]string{"/"}, "filtres guidés"},
	{[]string{"R"}, "rafraîchir la liste"},
	{[]string{"J"}, "journal des appels"},
	{[]string{"?"}, "cette aide"},
	{[]string{"q"}, "quitter"},
}

var aideMessage = []raccourci{
	{[]string{"↑", "↓"}, "défiler"},
	{[]string{"←", "→"}, "message précédent ou suivant"},
	{[]string{"entrée", "n"}, "message suivant"},
	{[]string{"début", "fin"}, "haut ou bas du message"},
	{[]string{"v"}, "marquer lu ou non lu"},
	{[]string{"d"}, "mettre à la corbeille"},
	{[]string{"r", "a", "t"}, "répondre, à tous, transférer"},
	{[]string{"q"}, "retour à la liste"},
}

var aideRédaction = []raccourci{
	{[]string{"↑", "↓"}, "champ précédent ou suivant"},
	{[]string{"tab"}, "champ suivant, ou valider une proposition"},
	{[]string{"entrée"}, "accepter l'adresse proposée"},
	{[]string{"ctrl+r"}, "ouvrir le corps dans l'éditeur"},
	{[]string{"ctrl+s"}, "envoyer"},
	{[]string{"échap"}, "abandonner, en gardant un brouillon"},
}

func (m *Modèle) vueAide() string {
	var out []string
	out = append(out, stTitre.Render("Raccourcis"))
	out = append(out, stLigneCourte.Render(strings.Repeat("─", m.largeur)))

	for _, section := range []struct {
		titre  string
		lignes []raccourci
	}{
		{"Liste", aideListe},
		{"Message", aideMessage},
		{"Rédaction", aideRédaction},
	} {
		out = append(out, "", stValeur.Render(section.titre))
		out = append(out, rendreAide(section.lignes, m.largeur)...)
	}

	if m.démo {
		out = append(out, "", stOrange.Render("Mode démonstration : aucun compte réel n'est utilisé."))
	}

	out = append(out, "", m.barreDuBas())
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

func rendreAide(lignes []raccourci, largeur int) []string {
	// On mesure la plus large colonne de touches pour aligner les effets.
	colonne := 0
	for _, r := range lignes {
		if l := lipgloss.Width(strings.Join(r.touches, " ")); l > colonne {
			colonne = l
		}
	}

	out := make([]string, 0, len(lignes))
	for _, r := range lignes {
		gauche := stSelection.Render(fmt.Sprintf("%-*s", colonne, strings.Join(r.touches, " ")))
		out = append(out, gauche+"  "+couperColonnes(r.effet, max0(largeur-colonne-2)))
	}
	return out
}

// ------------------------------------------------------------------ choix

// choix est le sélecteur générique : boîtes, comptes, destination d'un
// déplacement. Un seul mécanisme pour les trois, c'est une chose de moins à
// apprendre.
type choix struct {
	titre     string
	items     []string
	sélection int
	sur       func(int) tea.Cmd
	valider   bool
}

func nouveauChoix(titre string, items []string, index int, sur func(int) tea.Cmd) *choix {
	if index < 0 || index >= len(items) {
		index = 0
	}
	return &choix{titre: titre, items: items, sélection: index, sur: sur}
}

func (c *choix) décaler(delta int) {
	if len(c.items) == 0 {
		return
	}
	c.sélection = (c.sélection + delta + len(c.items)) % len(c.items)
}

func (m *Modèle) vueChoix() string {
	if m.choix == nil {
		return m.vueListe()
	}

	var out []string
	out = append(out, stTitre.Render(m.choix.titre))
	out = append(out, stLigneCourte.Render(strings.Repeat("─", m.largeur)))
	out = append(out, "")

	for i, item := range m.choix.items {
		if i == m.choix.sélection {
			out = append(out, stSelection.Render(" "+remplir(couperColonnes(item, max0(m.largeur-2)), m.largeur)+" "))
		} else {
			out = append(out, " "+couperColonnes(item, max0(m.largeur-2)))
		}
	}
	if len(m.choix.items) == 0 {
		out = append(out, stLigneCourte.Render("(rien à proposer)"))
	}

	out = append(out, "", m.barreDuBas())
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// confirmerChoix exécute l'action du sélecteur et referme le panneau.
func (m *Modèle) confirmerChoix() tea.Cmd {
	if m.choix == nil {
		return nil
	}
	action := m.choix.sur
	index := m.choix.sélection
	m.choix = nil
	m.écran = ÉcranListe
	if action == nil {
		return nil
	}
	return action(index)
}

// ------------------------------------------------------------------ filtres (vue)

func (m *Modèle) vueFiltres() string {
	var out []string
	out = append(out, stTitre.Render("Filtres"))
	out = append(out, stLigneCourte.Render(strings.Repeat("─", m.largeur)))
	out = append(out, "")

	for i, c := range critèresFiltres {
		valeur := valeurCritère(m.filtres, c.clé)
		if i == m.indexFiltre {
			valeur = m.saisieFiltre
			if valeur == "" {
				valeur = stLigneCourte.Render("(vide)")
			}
		} else if valeur == "" {
			valeur = stLigneCourte.Render("(vide)")
		}

		ligne := fmt.Sprintf("%-10s %s", c.texte, valeur)
		if i == m.indexFiltre {
			out = append(out, stSelection.Render(couperColonnes(ligne, m.largeur)))
		} else {
			out = append(out, couperColonnes(ligne, m.largeur))
		}
	}

	out = append(out, "")
	out = append(out, stEtiquette.Render("Requête envoyée à himalaya :"))
	out = append(out, "  "+stLigneCouper(m.filtres.Requête(), max0(m.largeur-2)))
	out = append(out, "", m.barreDuBas())
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}
