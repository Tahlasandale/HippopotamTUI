package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/config"
	"github.com/Tahlasandale/HippopotamTUI/internal/demo"
	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Capture est un écran rendu hors terminal, avec un nom.
//
// C'est ce qui permet de documenter l'interface sans Whoeverprise une capture
// d'écran qui vieillira mal, et de la régénérer d'une commande quand l'affichage
// change : « hippo capture » réécrit docs/captures/.
type Capture struct {
	Nom      string
	Intitulé string
	Texte    string
}

// Capturer rend les écrans principaux du client fourni.
func Capturer(client himalaya.Client, largeur, hauteur int) []Capture {
	cfg := config.Default()
	cfg.Editor = "true" // aucune capture ne doit ouvrir un éditeur

	m := Nouveau(client, nil, cfg, true)
	m.journal.Fixer(func() time.Time { return demo.InstantFigé })

	modèle, _ := m.Update(tea.WindowSizeMsg{Width: largeur, Height: hauteur})
	m = modèle.(*Modèle)
	file := jouer(m, []tea.Cmd{m.Init()})

	var captures []Capture
	ajouter := func(nom, intitulé string) {
		captures = append(captures, Capture{Nom: nom, Intitulé: intitulé, Texte: m.View()})
	}

	ajouter("accueil", "Le premier lancement")
	m.premièreFois = false
	ajouter("liste", "La boîte de réception")

	file = pas(m, file, tea.KeyMsg{Type: tea.KeyEnter})
	ajouter("message", "Un message ouvert")

	file = pas(m, file, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	ajouter("redaction", "La rédaction d'une réponse")

	file = pas(m, file, tea.KeyMsg{Type: tea.KeyEsc})
	file = pas(m, file, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	ajouter("filtres", "Le constructeur de filtres")

	file = pas(m, file, tea.KeyMsg{Type: tea.KeyEsc})
	file = pas(m, file, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	ajouter("journal", "Le journal des appels")

	file = pas(m, file, tea.KeyMsg{Type: tea.KeyEsc})
	file = pas(m, file, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	ajouter("aide", "Les raccourcis")

	return captures
}

// jouer exécute les commandes en attente, comme le ferait la boucle bubbletea.
// La file est locale à la capture : l'interface n'a pas à savoir qu'on la pilote.
func jouer(m *Modèle, file []tea.Cmd) []tea.Cmd {
	for tour := 0; tour < 32 && len(file) > 0; tour++ {
		courant := file
		file = nil
		for _, cmd := range courant {
			if cmd == nil {
				continue
			}
			msg := cmd()
			if msg == nil {
				continue
			}
			if lot, ok := msg.(tea.BatchMsg); ok {
				file = append(file, lot...)
				continue
			}
			modèle, suite := m.Update(msg)
			m = modèle.(*Modèle)
			if suite != nil {
				file = append(file, suite)
			}
		}
	}
	return file
}

// pas feedit une touche au modèle et exécute ce qu'elle déclenche.
func pas(m *Modèle, file []tea.Cmd, msg tea.KeyMsg) []tea.Cmd {
	modèle, suite := m.Update(msg)
	*m = *modèle.(*Modèle)
	if suite != nil {
		file = append(file, suite)
	}
	return jouer(m, file)
}

// SansANSI retire les séquences de couleur : une capture destinée à la
// documentation doit rester lisible dans un fichier texte.
func SansANSI(source string) string {
	var out strings.Builder
	for i := 0; i < len(source); i++ {
		if source[i] == 0x1b {
			for i < len(source) && source[i] != 'm' {
				i++
			}
			continue
		}
		out.WriteByte(source[i])
	}
	return out.String()
}

// EnTeur rôle de bordure : un cadre discret, ce qui permet d'inclure une capture
// dans un fichier Markdown sans en dépendre du support des couleurs.
func EnTeur(titre, texte string) string {
	lignes := strings.Split(SansANSI(texte), "\n")
	largeur := 0
	for _, ligne := range lignes {
		if l := lipgloss.Width(ligne); l > largeur {
			largeur = l
		}
	}

	var out strings.Builder
	fmt.Fprintf(&out, "┌─ %s %s\n", titre, strings.Repeat("─", max0(largeur-lipgloss.Width(titre)-2)))
	for _, ligne := range lignes {
		fmt.Fprintf(&out, "│ %s\n", ligne)
	}
	fmt.Fprintf(&out, "└%s\n", strings.Repeat("─", largeur+2))
	return out.String()
}
