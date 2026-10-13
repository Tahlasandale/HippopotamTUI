// Package redaction porte la logique pure de la rédaction : qui reçoit la
// réponse, et comment une suite de critères de l'interface devient la requête
// que himalaya sait comprendre.
//
// Rien ici ne dépend du terminal ni de himalaya : ce sont des fonctions qu'on
// peut tester sans démarrer d'interface.
package redaction

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
)

// Source décrit le message auquel on répond, avec tout ce qu'il faut pour
// décider qui reçoit quoi.
type Source struct {
	Origine    himalaya.Origine
	Expediteur []himalaya.Adresse
	Vers       []himalaya.Adresse
	Copie      []himalaya.Adresse
	ReplyTo    []string
	Propres    []string
}

// Destinataires calcule les listes To et Cc d'une réponse.
//
// Répondre à tous conserve la répartition d'origine : ceux qui étaient en To
// restent en To, ceux qui étaient en Cc restent en Cc. Une réponse simple, elle,
// ne reprend personne en copie.
//
// Répondre à un message que l'on a soi-même envoyé est traité comme une réponse
// à tous : le seul expéditeur est soi-même, et la cible utile est donc l'ensemble
// des destinataires d'origine.
func (s Source) Destinataires(àTous bool) (to, cc []string) {
	if s.Origine.Kind == himalaya.Transfert {
		return nil, nil
	}

	moi := ensemble(s.Propres)
	vus := map[string]bool{}

	ajouter := func(adresse string, cible *[]string) {
		adresse = cléOriginale(adresse)
		clé := strings.ToLower(adresse)
		if clé == "" || moi[clé] || vus[clé] {
			return
		}
		vus[clé] = true
		*cible = append(*cible, adresse)
	}

	expéditeurEstMoi := len(s.Expediteur) > 0 && moi[strings.ToLower(s.Expediteur[0].Email)]
	àTous = àTous || expéditeurEstMoi

	switch {
	case len(s.ReplyTo) > 0 && !expéditeurEstMoi:
		for _, adresse := range s.ReplyTo {
			ajouter(adresse, &to)
		}
	case àTous:
		for _, adresse := range s.Expediteur {
			ajouter(adresse.Email, &to)
		}
		for _, adresse := range s.Vers {
			ajouter(adresse.Email, &to)
		}
	default:
		for _, adresse := range s.Expediteur {
			ajouter(adresse.Email, &to)
		}
	}

	if àTous {
		for _, adresse := range s.Copie {
			ajouter(adresse.Email, &cc)
		}
	}
	return to, cc
}

// cléOriginale ne garde que la partie locale et le domaine tels qu'écrits, en
// écartant un éventuel nom d'affichage : les options de himalaya attendent une
// adresse.
func cléOriginale(adresse string) string {
	if debut := strings.LastIndex(adresse, "<"); debut >= 0 {
		if fin := strings.LastIndex(adresse, ">"); fin > debut {
			return adresse[debut+1 : fin]
		}
	}
	return strings.TrimSpace(adresse)
}

func ensemble(liste []string) map[string]bool {
	out := make(map[string]bool, len(liste))
	for _, adresse := range liste {
		adresse = cléOriginale(adresse)
		if adresse != "" {
			out[strings.ToLower(adresse)] = true
		}
	}
	return out
}

// préfixesReconnus liste les préfixes d'objet déjà posés, dans toutes leurs
// écritures courantes. « RE : » et « Re: » sont les deux formes qu'on trouve
// dans la nature, et un message peut en porter plusieurs à la suite.
var préfixesReconnus = []string{"re :", "re:", "tr :", "tr:", "aw :", "aw:", "fw :", "fwd :", "fwd:", "fw:"}

// sansPréfixes retire tous les préfixes d'objet déjà posés.
func sansPréfixes(sujet string) string {
	nettoye := strings.TrimSpace(sujet)
	for {
		trouvé := false
		for _, préfixe := range préfixesReconnus {
			if len(nettoye) >= len(préfixe) && strings.EqualFold(nettoye[:len(préfixe)], préfixe) {
				nettoye = strings.TrimSpace(nettoye[len(préfixe):])
				trouvé = true
				break
			}
		}
		if !trouvé {
			return nettoye
		}
	}
}

// SujetReponse ramène l'objet à un seul « Re: », quel que soit le nombre de
// préfixes qu'il portait déjà : répondre à un message qui commence par « Re : »
// ne doit pas produire « Re: Re : ».
func SujetReponse(sujet string) string {
	nettoye := sansPréfixes(sujet)
	if nettoye == "" {
		return "Re:"
	}
	return "Re: " + nettoye
}

// SujetTransfert fait de même pour un transfert.
func SujetTransfert(sujet string) string {
	nettoye := sansPréfixes(sujet)
	if nettoye == "" {
		return "Fwd:"
	}
	return "Fwd: " + nettoye
}

// Citer préfixe chaque ligne du texte d'origine, comme le fait une réponse
// dans tous les clients de messagerie.
func Citer(origine string) string {
	if origine == "" {
		return ""
	}
	lignes := strings.Split(strings.ReplaceAll(origine, "\r\n", "\n"), "\n")
	for i, ligne := range lignes {
		ligne = strings.TrimRight(ligne, " ")
		if ligne == "" {
			lignes[i] = ">"
			continue
		}
		lignes[i] = "> " + ligne
	}
	return strings.Join(lignes, "\n")
}

// CiterPourTransfert encadre le message d'origine d'un séparateur lisible.
func CiterPourTransfert(origine, expéditeur, objet, date string) string {
	var b strings.Builder
	b.WriteString("-------- Message transféré --------\n")
	fmt.Fprintf(&b, "De : %s\n", expéditeur)
	if objet != "" {
		fmt.Fprintf(&b, "Objet : %s\n", objet)
	}
	if date != "" {
		fmt.Fprintf(&b, "Date : %s\n", date)
	}
	b.WriteString("\n")
	b.WriteString(strings.TrimRight(origine, "\n"))
	b.WriteString("\n")
	return b.String()
}

// Tronquer coupe un objet à la largeur disponible, au mot le plus proche, en
// comptant les caractères et non les octets : un accent ne doit jamais être
// coupé en deux.
func Tronquer(texte string, largeur int) string {
	if largeur <= 1 {
		return "…"
	}
	runes := []rune(texte)
	if len(runes) <= largeur {
		return texte
	}

	disponible := largeur - 1
	fin := disponible
	for fin > 0 && runes[fin] != ' ' {
		fin--
	}
	if fin == 0 {
		fin = disponible
	}
	return strings.TrimRight(string(runes[:fin]), " ") + "…"
}

// Filtres est l'état du constructeur de filtres guidés.
type Filtres struct {
	De          string
	Objet       string
	Corps       string
	Drapeaux    []string
	Depuis      time.Time
	Avant       time.Time
	NonLusSeuls bool
}

// Vides indique qu'aucun critère n'est renseigné.
func (f Filtres) Vides() bool { return f.Requête() == "" }

// Requête traduit les critères en requête himalaya, combinés par « and » dans
// un ordre fixe : deux exécutions sur les mêmes critères donnent la même
// requête, ce qui rend le cache et les tests fiables.
func (f Filtres) Requête() string {
	var clauses []string

	for _, clause := range []struct{ champ, valeur string }{
		{"from", f.De},
		{"subject", f.Objet},
		{"body", f.Corps},
	} {
		if valeur := strings.TrimSpace(clause.valeur); valeur != "" {
			clauses = append(clauses, clause.champ+" "+citer(valeur))
		}
	}

	for _, drapeau := range f.Drapeaux {
		if drapeau = strings.TrimSpace(drapeau); drapeau != "" {
			clauses = append(clauses, "flag "+drapeau)
		}
	}

	if !f.Depuis.IsZero() {
		clauses = append(clauses, "after "+f.Depuis.Format("2006-01-02"))
	}
	if !f.Avant.IsZero() {
		clauses = append(clauses, "before "+f.Avant.Format("2006-01-02"))
	}
	if f.NonLusSeuls {
		clauses = append(clauses, "not flag seen")
	}

	return strings.Join(clauses, " and ")
}

// Description rend les critères en français, pour les afficher sous la liste
// plutôt que de laisser fuir la syntaxe de himalaya.
func (f Filtres) Description() string {
	var parts []string
	for _, p := range []struct{ libellé, valeur string }{
		{"De", f.De},
		{"Objet", f.Objet},
		{"Corps", f.Corps},
	} {
		if valeur := strings.TrimSpace(p.valeur); valeur != "" {
			parts = append(parts, fmt.Sprintf("%s %q", p.libellé, valeur))
		}
	}
	for _, drapeau := range f.Drapeaux {
		if drapeau = strings.TrimSpace(drapeau); drapeau != "" {
			parts = append(parts, libelléDrapeau(drapeau))
		}
	}
	if !f.Depuis.IsZero() {
		parts = append(parts, "depuis le "+f.Depuis.Format("02/01/2006"))
	}
	if !f.Avant.IsZero() {
		parts = append(parts, "avant le "+f.Avant.Format("02/01/2006"))
	}
	if f.NonLusSeuls {
		parts = append(parts, "non lus seulement")
	}
	return strings.Join(parts, " · ")
}

func libelléDrapeau(drapeau string) string {
	switch drapeau {
	case "seen":
		return "déjà lus"
	case "answered":
		return "répondus"
	case "flagged":
		return "marqués"
	case "draft":
		return "brouillons"
	default:
		return drapeau
	}
}

// citer entoure un motif de guillemets dès qu'il contient autre chose que des
// mots simples, et échappe les guillemets qu'il contient déjà.
func citer(motif string) string {
	if !besoinDeGuillemets(motif) {
		return motif
	}
	if cite(motif) {
		return motif
	}
	échappé := strings.ReplaceAll(motif, `"`, `\"`)
	return `"` + échappé + `"`
}

func cite(motif string) bool {
	return len(motif) >= 2 && strings.HasPrefix(motif, `"`) && strings.HasSuffix(motif, `"`)
}

func besoinDeGuillemets(motif string) bool {
	if motif == "" {
		return false
	}
	for _, r := range motif {
		switch r {
		case ' ', '\t', '(', ')', '"', ':':
			return true
		}
	}
	return false
}
