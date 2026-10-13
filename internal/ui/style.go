package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Le parti pris est sobre : une couleur d'accent pour la ligne courante, une
// couleur douce pour les métadonnées, et rien d'autre. Un client mail se lit
// souvent dans le noir, et une interface qui clignote est un client mail qu'on
// ferme. Les couleurs s'adaptent au fond du terminal, clair ou sombre, et
// lipgloss redescend tout seul en 256 ou 16 couleurs quand le terminal ne sait
// pas faire mieux.

var (
	accent     = lipgloss.AdaptiveColor{Light: "#1d4ed8", Dark: "#7aa2f7"}
	accentFort = lipgloss.AdaptiveColor{Light: "#1e3a8a", Dark: "#bb9af7"}
	discret    = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#565f89"}
	vert       = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#9ece6a"}
	rouge      = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#f7768e"}
	orange     = lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#e0af68"}
	bleuPale   = lipgloss.AdaptiveColor{Light: "#eff6ff", Dark: "#1a1b26"}
	fondEncart = lipgloss.AdaptiveColor{Light: "#f3f4f6", Dark: "#24283b"}
	fondChoix  = lipgloss.AdaptiveColor{Light: "#dbeafe", Dark: "#2f3549"}
)

var (
	stLigneNormale = lipgloss.NewStyle().Foreground(lipgloss.NoColor{})
	stLigneCourte  = lipgloss.NewStyle().Foreground(discret)
	stLigneActive  = lipgloss.NewStyle().Foreground(accent).Bold(true)
	stSujetVole    = lipgloss.NewStyle().Foreground(discret)
	stEtiquette    = lipgloss.NewStyle().Foreground(discret)
	stValeur       = lipgloss.NewStyle().Foreground(accentFort)
	stTitre        = lipgloss.NewStyle().Foreground(accent).Bold(true)
	stTitreEcran   = lipgloss.NewStyle().Foreground(lipgloss.NoColor{}).Bold(true)
	stBarre        = lipgloss.NewStyle().Foreground(discret)
	stSelection    = lipgloss.NewStyle().Foreground(lipgloss.NoColor{}).Background(fondChoix).Bold(true)
	stEncart       = lipgloss.NewStyle().Foreground(lipgloss.NoColor{}).Background(fondEncart).Padding(0, 1)
	stErreur       = lipgloss.NewStyle().Foreground(rouge)
	stInfo         = lipgloss.NewStyle().Foreground(vert)
	stDroite       = lipgloss.NewStyle().Align(lipgloss.Right)
	stOrange       = lipgloss.NewStyle().Foreground(orange)
)

// borneFixe renvoie l'emplacement d'une zone dans une largeur donnée, de façon
// stable quand la fenêtre rétrécit : une colonne latérale disparaît avant que le
// texte ne soit coupé.
func borneFixe(largeur, entiere, minimale int) int {
	if largeur < minimale {
		return 0
	}
	if entiere > largeur-minimale {
		return largeur - minimale
	}
	return entiere
}

// profileTerminal indique le nombre de couleurs réellement disponibles. Il sert
// au journal : un terminal à 8 couleurs n'a pas à être deviné à l'écran.
func profileTerminal() termenv.Profile {
	return termenv.EnvColorProfile()
}

// couperColonnes découpe un texte en largeur, en scandalisant les caractères
// doubles d'une police CJK au lieu de couper au hasard.
func couperColonnes(s string, largeur int) string {
	if largeur <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= largeur {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > largeur {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// remplir complète une chaîne jusqu'à la largeur donnée, sans compter les
// caractères ANSI qu'elle contient déjà.
func remplir(s string, largeur int) string {
	manque := largeur - lipgloss.Width(s)
	if manque <= 0 {
		return s
	}
	return s + strings.Repeat(" ", manque)
}
