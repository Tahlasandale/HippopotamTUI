package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	"github.com/Tahlasandale/HippopotamTUI/internal/redaction"
	"github.com/Tahlasandale/HippopotamTUI/internal/rfc822"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View compose l'écran courant.
func (m *Modèle) View() string {
	if m.largeur < 40 || m.hauteur < 10 {
		return renduTropPetit(m.largeur, m.hauteur)
	}

	switch m.écran {
	case ÉcranMessage:
		return m.vueMessage()
	case ÉcranRédaction:
		return m.vueRédaction()
	case ÉcranAide:
		return m.vueAide()
	case ÉcranJournal:
		return m.vueJournal()
	case ÉcranFiltres:
		return m.vueFiltres()
	case ÉcranChoix:
		return m.vueChoix()
	default:
		return m.vueListe()
	}
}

func renduTropPetit(largeur, hauteur int) string {
	besoin := fmt.Sprintf("HippopotamTUI a besoin d'au moins 40 colonnes et 10 lignes.\nLe terminal en fait %d × %d.\n\n Agrandissez-le, ou réduisez la taille de la police.",
		largeur, hauteur)
	return stLigneCourte.Render(besoin)
}

// ------------------------------------------------------------------ en-tête

// entête décrit où l'on se trouve : compte, boîte, position, filtres. C'est la
// seule ligne qui ne change jamais de place, et elle rend la liste lisible sans
// avoir à chercher.
func (m *Modèle) entête() string {
	gauche := stTitre.Render("hippo")
	gauche += stLigneCourte.Render(" · ")
	gauche += stValeur.Render(m.compte)
	gauche += stLigneCourte.Render(" / ")
	gauche += stValeur.Render(m.boîte)

	if description := m.filtres.Description(); description != "" {
		gauche += stLigneCourte.Render(" · ") + stInfo.Render("filtre : "+description)
	}

	droite := ""
	if len(m.enveloppes) > 0 {
		droite = fmt.Sprintf("%d/%d", m.curseur+1, len(m.enveloppes))
	}

	espace := m.largeur - lipgloss.Width(gauche) - lipgloss.Width(droite)
	if espace < 1 {
		return couperColonnes(gauche, m.largeur)
	}
	return gauche + strings.Repeat(" ", espace) + stLigneCourte.Render(droite)
}

// barreDuBas tient lieu de barre d'état : raccourcis du moment, attente en cours,
// message d'erreur ou d'information.
func (m *Modèle) barreDuBas() string {
	if m.erreur != "" {
		return stErreur.Render(couperColonnes("! "+m.erreur, m.largeur))
	}
	if m.info != "" {
		return stInfo.Render(couperColonnes(m.info, m.largeur))
	}
	if m.occupé {
		attente := m.attente
		if attente == "" {
			attente = "chargement"
		}
		return stLigneCourte.Render("… " + attente + "…")
	}
	return m.raccourcisDuMoment()
}

// raccourcisDuMoment ne montre que ce qui sert dans l'écran courant : une barre
// d'aide saturée n'aide plus à rien.
func (m *Modèle) raccourcisDuMoment() string {
	var touches []string
	switch m.écran {
	case ÉcranListe:
		touches = []string{"↑↓ naviguer", "←→ boîte", "entrer lire", "r répondre", "a à tous", "t transférer", "c écrire", "/ filtres", "m boîte", "S compte", "s déplacer", "v lu", "F marquer", "d effacer", "R rafraîchir", "? aide", "q quitter"}
	case ÉcranMessage:
		touches = []string{"↑↓ défiler", "←→ message", "entrer suivant", "r répondre", "a à tous", "t transférer", "q liste", "? aide"}
	case ÉcranRédaction:
		touches = []string{"↑↓ champ", "tab suivant", "entendre proposition", "ctrl+r éditer", "ctrl+s envoyer", "échap abandonner"}
	case ÉcranFiltres:
		touches = []string{"↑↓ critère", "←→ basculer", "entrée valider", "échap annuler"}
	case ÉcranAide:
		touches = []string{"échap retour"}
	case ÉcranJournal:
		touches = []string{"échap retour"}
	case ÉcranChoix:
		touches = []string{"↑↓ choisir", "entrée valider", "échap annuler"}
	}

	texte := strings.Join(touches, stLigneCourte.Render("  ·  "))
	if lipgloss.Width(texte) <= m.largeur {
		return stLigneCourte.Render(texte)
	}

	// Trop long pour la fenêtre : on garde le début, qui contient la navigation,
	// plutôt que de tronquer au milieu d'un mot.
	morceau := texte
	for lipgloss.Width(morceau+"  ·  …") > m.largeur && len(morceau) > 20 {
		idx := strings.LastIndex(morceau, stLigneCourte.Render("  ·  "))
		if idx <= 0 {
			break
		}
		morceau = morceau[:idx]
	}
	return stLigneCourte.Render(morceau + "  ·  …")
}

// ------------------------------------------------------------------ liste

func (m *Modèle) vueListe() string {
	var out []string
	out = append(out, m.entête())
	out = append(out, stLigneCourte.Render(strings.Repeat("─", m.largeur)))
	if m.premièreFois {
		out = append(out, m.bandeauAccueil()...)
		out = append(out, "")
	}
	out = append(out, m.lignesListe()...)
	out = append(out, "")
	out = append(out, m.barreDuBas())
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// bandeauAccueil n'apparaît qu'une fois, au tout premier lancement. Cinq
// raccourcis suffisent : le reste est dans « ? », qui n'est jamais loin.
func (m *Modèle) bandeauAccueil() []string {
	ligne := func(touche, effet string) string {
		return stSelection.Render(fmt.Sprintf(" %-8s ", touche)) + " " + stLigneCourte.Render(effet)
	}
	return []string{
		stTitre.Render("Bienvenue dans HippopotamTUI.") + stLigneCourte.Render(" Les flèches naviguent, les lettres agissent."),
		"",
		ligne("↑ ↓", "se déplacer dans la liste"),
		ligne("entrée", "ouvrir le message"),
		ligne("r  a  t", "répondre, à tous, transférer"),
		ligne("c", "écrire un message"),
		ligne("?", "tous les raccourcis"),
	}
}

func (m *Modèle) lignesListe() []string {
	if len(m.enveloppes) == 0 {
		raison := "Aucun message."
		if m.occupé {
			raison = "Chargement…"
		} else if !m.filtres.Vides() {
			raison = "Aucun message ne correspond à ces filtres."
		}
		return []string{"", stLigneCourte.Render(raison)}
	}

	visibles := m.lignesVisibles()
	if m.décalage > len(m.enveloppes)-1 {
		m.décalage = max0(len(m.enveloppes) - 1)
	}
	fin := min(m.décalage+visibles, len(m.enveloppes))

	lignes := make([]string, 0, visibles)
	for i := m.décalage; i < fin; i++ {
		lignes = append(lignes, m.rendreEnveloppe(m.enveloppes[i], i == m.curseur))
	}
	return lignes
}

// rendreEnveloppe compose une ligne de liste : date, expéditeur, objet, drapeaux.
// La colonne expéditeur a une largeur fixe calculée une fois, sinon les objets
// sauteraient d'une ligne à l'autre et la liste serait illisible.
func (m *Modèle) rendreEnveloppe(env himalaya.Enveloppe, active bool) string {
	marque := " "
	switch {
	case env.A(himalaya.Marqué):
		marque = stOrange.Render("★")
	case env.A(himalaya.PoussièreVolee):
		marque = " "
	default:
		marque = stValeur.Render("•")
	}

	date := formaterDate(env.Date)
	expéditeur := couperColonnes(m.nomExpéditeurDe(env), 22)
	objet := redaction.Tronquer(env.Sujet, max0(m.largeur-lipgloss.Width(date)-lipgloss.Width(expéditeur)-12))

	if env.PiecesJointes {
		objet = "📎 " + objet
	}

	ligne := fmt.Sprintf("%s %s %s %s", marque, stLigneCourte.Render(date),
		remplir(stLigneNormale.Render(couperColonnes(expéditeur, 22)), 22), stLigneNormale.Render(couperColonnes(objet, m.largeur-30)))

	if active {
		return stSelection.Render(couperColonnes(ligne, m.largeur))
	}
	if !env.A(himalaya.PoussièreVolee) {
		return ligne
	}
	return stSujetVole.Render(couperColonnes(ligne, m.largeur))
}

// nomExpéditeurDe préfère le nom d'affichage, et tombe sur l'adresse quand le
// message vient d'une liste de diffusion qui n'en fournit pas.
func (m *Modèle) nomExpéditeurDe(env himalaya.Enveloppe) string {
	if len(env.De) == 0 {
		return "(sans expéditeur)"
	}
	nom := env.De[0].Nom
	if nom == "" {
		return env.De[0].Email
	}
	return nom
}

// formaterDate rend l'heure pour un message du jour, la date au-delà. C'est le
// format qu'on veut voir dans une liste de courrier : « 21:17 » quand ça vient
// d'arriver, « 12/10 » quand c'est plus ancien.
func formaterDate(brut string) string {
	t := rfc822.LireDate(brut)
	if t.IsZero() {
		return "??/??"
	}
	aujourdhui := time.Now()
	if t.Year() == aujourdhui.Year() && t.YearDay() == aujourdhui.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("02/01")
}

// ------------------------------------------------------------------ message

func (m *Modèle) vueMessage() string {
	if m.message == nil {
		return m.vueListe()
	}

	var out []string
	out = append(out, m.entête())
	out = append(out, stLigneCourte.Render(strings.Repeat("─", m.largeur)))
	out = append(out, m.rendreEntêtesMessage()...)
	out = append(out, "")
	out = append(out, m.lignesMessage()...)
	out = append(out, "")
	out = append(out, m.barreDuBas())
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

func (m *Modèle) rendreEntêtesMessage() []string {
	msg := m.message
	largeur := max0(m.largeur - 9)
	ligne := func(nom, valeur string) string {
		if valeur == "" {
			return ""
		}
		return stEtiquette.Render(fmt.Sprintf("%-7s ", nom+" :")) + couperColonnes(valeur, largeur)
	}

	var out []string
	for _, entête := range []struct{ nom, valeur string }{
		{"De", msg.De},
		{"À", msg.Vers},
		{"Cc", msg.Cc},
		{"Objet", msg.Sujet},
	} {
		if ligne := ligne(entête.nom, entête.valeur); ligne != "" {
			out = append(out, ligne)
		}
	}
	if !msg.Date.IsZero() {
		out = append(out, ligne("Date", msg.Date.Format("lundi 2 janvier 2006 à 15:04")))
	}
	if len(msg.Pièces) > 0 {
		noms := []string{}
		for _, p := range msg.Pièces {
			noms = append(noms, p.Nom)
		}
		out = append(out, stEtiquette.Render(fmt.Sprintf("%-7s ", "Pièces :"))+stLigneCourte.Render(strings.Join(noms, ", ")))
	}
	return out
}

func (m *Modèle) lignesMessage() []string {
	visibles := m.lignesVisibles()
	if m.défilement > max0(len(m.lignes)-visibles) {
		m.défilement = max0(len(m.lignes) - visibles)
	}
	fin := min(m.défilement+visibles, len(m.lignes))

	out := make([]string, 0, visibles)
	for i := m.défilement; i < fin; i++ {
		out = append(out, couperColonnes(m.lignes[i], m.largeur))
	}
	if len(m.lignes) == 0 {
		out = append(out, stLigneCourte.Render("(message vide)"))
	}
	return out
}

// ------------------------------------------------------------------ rédaction

func (m *Modèle) vueRédaction() string {
	if m.formulaire == nil {
		return m.vueListe()
	}

	var out []string
	out = append(out, stTitre.Render("Nouveau message"))
	out = append(out, stLigneCourte.Render(strings.Repeat("─", m.largeur)))
	out = append(out, rendreFormulaire(m.formulaire, m.largeur)...)

	if len(m.formulaire.proposés) > 0 {
		out = append(out, "")
		out = append(out, m.rendrePropositions()...)
	}

	out = append(out, "")
	out = append(out, m.barreDuBas())
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

func (m *Modèle) rendrePropositions() []string {
	var out []string
	for i, p := range m.formulaire.proposés {
		ligne := couperColonnes(p.Ligne(), max0(m.largeur-2))
		if i == m.formulaire.choisi {
			out = append(out, stSelection.Render(" "+remplir(ligne, max0(m.largeur-2))+" "))
		} else {
			out = append(out, " "+ligne)
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = tea.KeyMsg{}
