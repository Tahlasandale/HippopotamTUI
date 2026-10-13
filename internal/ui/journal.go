package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Le journal garde ce que le TUI a fait et ce que himalaya a répondu. Il est
// indispensable quand un appel échoue : le bandeau d'erreur montre la phrase de
// l'outil, le journal montre la commande exacte qu'il a essayée.

type ligneJournal struct {
	quand  time.Time
	niveau string
	texte  string
}

// Journal garde la trace des appels faits à himalaya.
type Journal struct {
	lignes  []ligneJournal
	maximum int
	fichier string
	noyau   []string
}

// NouveauJournal crée un journal en mémoire, rattaché au fichier de journal si
// on en a un chemin. Rien n'est écrit sur le disque tant que la ligne contient
// une adresse ou un sujet : un journal de TUI ne doit pas devenir une seconde
// boîte aux lettres.
func NouveauJournal(chemin ...string) *Journal {
	j := &Journal{maximum: 500}
	if len(chemin) > 0 && chemin[0] != "" {
		j.fichier = chemin[0]
	}
	return j
}

// Noter ajoute une ligne d'information.
func (j *Journal) Noter(texte string) { j.ajouter("info", texte) }

// Erreur ajoute une ligne d'erreur.
func (j *Journal) Erreur(texte string) { j.ajouter("erreur", texte) }

// ajouter range une ligne, en mémoire et éventuellement sur le disque.
func (j *Journal) ajouter(niveau, texte string) {
	if j == nil {
		return
	}
	j.lignes = append(j.lignes, ligneJournal{quand: time.Now(), niveau: niveau, texte: texte})
	if len(j.lignes) > j.maximum {
		j.lignes = j.lignes[len(j.lignes)-j.maximum:]
	}
	j.écrire(niveau, texte)
}

// écrire ne met sur le disque que les actions et les erreurs, pas la navigation.
func (j *Journal) écrire(niveau, texte string) {
	if j.fichier == "" {
		return
	}
	if niveau == "info" && !intéresse(texte) {
		return
	}
	os.MkdirAll(filepath.Dir(j.fichier), 0o700)
	f, err := os.OpenFile(j.fichier, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %-6s %s\n", time.Now().Format(time.RFC3339), niveau, texte)
}

// intéresse dit si une ligne d'information mérite le fichier : les écritures, les
// envois, les erreurs.
func intéresse(texte string) bool {
	for _, marqueur := range []string{"envoyé", "échec", "brouillon", "impossible", "erreur"} {
		if strings.Contains(strings.ToLower(texte), marqueur) {
			return true
		}
	}
	return false
}

// armerJournal prépare la rotation : le fichier de journal ne doit pas grossir
// indéfiniment sur un poste qui tourne des mois.
type tickJournalMsg time.Time

func (m *Modèle) armerJournal() tea.Cmd {
	return func() tea.Msg {
		if m.journal != nil && m.journal.fichier != "" {
			purgerJournal(m.journal.fichier, 2<<20)
		}
		return tickJournalMsg(time.Now())
	}
}

// purgerJournal réduit le fichier à la taille maximale quand il la dépasse,
// en gardant sa moitié la plus récente.
func purgerJournal(chemin string, maximum int64) {
	info, err := os.Stat(chemin)
	if err != nil || info.Size() <= maximum {
		return
	}
	contenu, err := os.ReadFile(chemin)
	if err != nil {
		return
	}
	moitié := len(contenu) / 2
	os.WriteFile(chemin, contenu[moitié:], 0o600)
}

func (m *Modèle) ticHorloge() tea.Cmd {
	return tea.Tick(time.Minute, func(t time.Time) tea.Msg { return tickJournalMsg(t) })
}

func (m *Modèle) vueJournal() string {
	var out []string
	out = append(out, stTitre.Render("Journal"))
	out = append(out, stLigneCouper("Les appels faits à himalaya, dans l'ordre. Rien n'y est écrit sur le disque à part les envois et les erreurs.", m.largeur))
	out = append(out, stLigneCourte.Render(strings.Repeat("─", m.largeur)))
	out = append(out, "")

	lignes := m.journal.lignes
	visibles := max0(m.hauteur - 9)
	if len(lignes) > visibles {
		lignes = lignes[len(lignes)-visibles:]
	}
	if len(lignes) == 0 {
		out = append(out, stLigneCourte.Render("(rien pour l'instant)"))
	}
	for _, l := range lignes {
		horodatage := stLigneCourte.Render(l.quand.Format("15:04:05"))
		texte := l.texte
		if l.niveau == "erreur" {
			texte = stErreur.Render(texte)
		}
		out = append(out, horodatage+" "+couperColonnes(texte, max0(m.largeur-10)))
	}

	out = append(out, "", m.barreDuBas())
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// stLigneCouper coupe proprement une ligne longue en la coupant sur un espace.
func stLigneCouper(texte string, largeur int) string {
	if largeur <= 0 {
		return ""
	}
	if lipgloss.Width(texte) <= largeur {
		return texte
	}
	morceau := texte[:largeur]
	if idx := strings.LastIndex(morceau, " "); idx > largeur/2 {
		morceau = morceau[:idx]
	}
	return morceau + "…"
}
