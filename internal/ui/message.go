package ui

import (
	"strings"

	"github.com/Tahlasandale/HippopotamTUI/internal/rfc822"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// rendreCorps transforme un message en lignes affichables.
//
// Le rendu Markdown n'est appliqué que lorsque le message le mérite vraiment : un
// HTML, ou une partie text/markdown. Passer un courriel ordinaire dans glamour
// produirait des titres, des emphases et des tableaux là où il n'y a que des
// paragraphes — c'est-à-dire pour presque tous les messages du monde.
func rendreCorps(msg *rfc822.Message, largeur int) []string {
	if msg == nil {
		return nil
	}
	if largeur < 20 {
		largeur = 20
	}

	if msg.EstMarkdown() {
		if rendu, err := glamourMarkdown(msg.Corps, largeur); err == nil && rendu != "" {
			return strings.Split(rendu, "\n")
		}
	}

	// Sans rendu balisé, le texte est simplement replié sur la largeur, avec les
	// lignes vides conservées : elles séparent les paragraphes.
	return normaliserCorps(msg.Corps, largeur)
}

// glamourMarkdown rend le Markdown avec glamour. Une largeur nulle est tolérée,
// glamour prend alors sa largeur par défaut.
func glamourMarkdown(source string, largeur int) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", nil
	}
	style := "light"
	if lipgloss.HasDarkBackground() {
		style = "dark"
	}

	options := []glamour.TermRendererOption{
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(largeur),
	}
	rendu, err := glamour.NewTermRenderer(options...)
	if err != nil {
		return "", err
	}
	return rendu.Render(source)
}

// normaliserCorps replie le texte en respectant les paragraphes.
func normaliserCorps(corps string, largeur int) []string {
	corps = strings.ReplaceAll(corps, "\r\n", "\n")
	brut := strings.Split(corps, "\n")

	var out []string
	for _, ligne := range brut {
		out = append(out, strings.Split(replier(ligne, largeur), "\n")...)
	}
	// Un message se termine normalement par un retour à la ligne : on n'en garde
	// pas une ligne vide en bas de l'écran.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}

// replier coupe une ligne trop longue sur un mot, en indiquant la coupure par
// deux points comme le fait un terminal.
func replier(ligne string, largeur int) string {
	runes := []rune(ligne)
	if len(runes) <= largeur {
		return ligne
	}

	morceaux := []string{}
	reste := string(runes)
	for len([]rune(reste)) > largeur {
		fin := largeur
		// On remonte au dernier espace avant la coupure, pour ne pas casser un
		// mot au milieu.
		for fin > 0 && reste[fin] != ' ' {
			fin--
		}
		if fin < largeur/2 {
			fin = largeur
		}
		morceaux = append(morceaux, reste[:fin])
		reste = reste[fin:]
		reste = strings.TrimPrefix(reste, " ")
	}
	morceaux = append(morceaux, reste)
	return strings.Join(morceaux, "\n")
}
