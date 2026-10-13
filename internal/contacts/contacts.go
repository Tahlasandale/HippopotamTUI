// Package contacts reconstruit un carnet d'adresses à partir des messages.
//
// himalaya n'a pas de carnet : proposer une adresse pendant la frappe impose
// donc d'aller la chercher quelque part. On la prend dans les messages eux-mêmes
// — les Envoyés et la boîte de réception de chaque compte — parce que ce sont les
// correspondants réels, sans rien inventer et sans toucher au réseau au-delà de
// ce que l'utilisateur consulte déjà.
package contacts

import (
	"sort"
	"strings"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
)

// Contact est une adresse propositionnable pendant la frappe.
type Contact struct {
	Nom   string
	Email string

	// SurAdresse signale que la proposition a été trouvée par l'adresse et non
	// par le nom. Elle doit alors être présentée comme une adresse seule : taper
	// « alice@ex » et se voir proposer « Alice Martin <alice@example.org> »
	// obligerait à retirer le nom à la main.
	SurAdresse bool
}

// Affichage rend le contact selon ce que l'utilisateur a tapé : une adresse doit
// rester une adresse, pour qu'on puisse la reprendre telle quelle.
func (c Contact) Affichage() string {
	if c.SurAdresse || c.Nom == "" || c.PresqueEmail() {
		return c.Email
	}
	return c.Nom + " <" + c.Email + ">"
}

// Ligne est ce qui est affiché dans la liste de propositions.
func (c Contact) Ligne() string { return c.Affichage() }

// PresqueEmail indique que le nom d'affichage n'est qu'une adresse déguisée, ce
// qui arrive quand l'expéditeur d'une liste de diffusion n'a pas de nom.
func (c Contact) PresqueEmail() bool {
	return c.Nom != "" && c.Nom == c.Email
}

// sansAccent retire les diacritiques et met en minuscules, pour que taper
// « elodie » trouve « Élodie ».
func sansAccent(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'à', 'â', 'ä', 'á', 'ã', 'å':
			out.WriteRune('a')
		case 'é', 'è', 'ê', 'ë':
			out.WriteRune('e')
		case 'î', 'ï', 'í':
			out.WriteRune('i')
		case 'ô', 'ö', 'ó', 'õ':
			out.WriteRune('o')
		case 'ù', 'û', 'ü', 'ú':
			out.WriteRune('u')
		case 'ç':
			out.WriteRune('c')
		case 'ñ':
			out.WriteRune('n')
		case 'œ':
			out.WriteRune('o')
		case 'æ':
			out.WriteRune('a')
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// sansBruit retire les espaces, pour comparer des saisies Prediction-style.
func sansBruit(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// automatesIninterdits sont les adresses qu'on ne propose jamais : répondre à
// une liste de diffusion qui se désabonne seule est un classique du genre.
var automatesIninterdits = []string{
	"do-not-reply", "donotreply", "no-reply", "noreply", "bounce", "mailer-daemon",
}

// exploitable indique si une adresse peut être proposée à un humain.
func exploitable(email string) bool {
	email = strings.TrimSpace(email)
	avant, après, trouvé := strings.Cut(email, "@")
	if !trouvé || avant == "" || après == "" {
		return false
	}
	bas := sansAccent(avant)
	for _, interdit := range automatesIninterdits {
		if strings.Contains(bas, interdit) {
			return false
		}
	}
	return true
}

// Récolter extrait les contacts d'un lot d'enveloppes.
//
// propres est la liste de nos adresses : inutile de nous proposer de nous écrire.
// Une même adresse n'apparaît qu'une fois, dans la version la plus riche, celle
// qui porte un nom.
func Récolter(enveloppes []himalaya.Enveloppe, propres []string) []Contact {
	moi := map[string]bool{}
	for _, adresse := range propres {
		moi[strings.ToLower(strings.TrimSpace(adresse))] = true
	}

	index := map[string]*Contact{}
	for _, env := range enveloppes {
		for _, adresse := range append(append([]himalaya.Adresse{}, env.De...), append(env.Vers, env.Cc...)...) {
			email := strings.TrimSpace(adresse.Email)
			if !exploitable(email) || moi[strings.ToLower(email)] {
				continue
			}

			clé := strings.ToLower(email)
			existant, vu := index[clé]
			if !vu {
				index[clé] = &Contact{Nom: strings.TrimSpace(adresse.Nom), Email: email}
				continue
			}
			if strings.TrimSpace(adresse.Nom) != "" && existant.Nom == "" {
				existant.Nom = strings.TrimSpace(adresse.Nom)
			}
		}
	}

	trouvés := make([]Contact, 0, len(index))
	for _, contact := range index {
		trouvés = append(trouvés, *contact)
	}
	sort.Slice(trouvés, func(i, j int) bool {
		return sansAccent(trouvés[i].Affichage()) < sansAccent(trouvés[j].Affichage())
	})
	return trouvés
}

// Pages calcule combien de pages de taille parPage il faut parcourir pour
// couvrir total messages, sans dépasser maximum.
func Pages(total, parPage, maximum int) int {
	if total <= 0 || maximum <= 0 {
		return 0
	}
	if parPage <= 0 {
		// Sans taille de page, on ne peut calculer qu'une page : le TUI aura de
		// toute façon un effectif par défaut.
		return 1
	}
	pages := (total + parPage - 1) / parPage
	if pages > maximum/parPage {
		pages = maximum / parPage
	}
	if pages < 1 {
		pages = 1
	}
	return pages
}

// Correspondance propose les contacts qui correspondent à une saisie.
//
// La recherche est une sous-séquence : « almt » trouve « Alice Martin », comme
// dans tout client mail qui se respecte. Un préfixe exact l'emporte toujours sur
// une correspondance plus lâche, et le nombre de propositions est plafonné par
// l'appelant, qui sait combien de lignes il peut afficher.
func Correspondance(contacts []Contact, saisie string, limite int) []Contact {
	clée := sansBruit(sansAccent(saisie))
	if clée == "" || limite <= 0 {
		return nil
	}

	type scored struct {
		contact Contact
		score   int
		index   int
	}

	var résultats []scored
	for i, contact := range contacts {
		score := noter(contact, clée)
		if score < 0 {
			continue
		}
		contact.SurAdresse = provenanceAdresse(contact, clée) && !provenanceNom(contact, clée)
		résultats = append(résultats, scored{contact: contact, score: score, index: i})
	}

	// À score égal, l'ordre alphabétique tranche : deux contacts peuvent
	// commencer pareil, et l'utilisateur mérite de voir « Alice » avant « Aline ».
	sort.SliceStable(résultats, func(i, j int) bool {
		if résultats[i].score != résultats[j].score {
			return résultats[i].score < résultats[j].score
		}
		gauche := sansAccent(résultats[i].contact.Affichage())
		droite := sansAccent(résultats[j].contact.Affichage())
		if gauche != droite {
			return gauche < droite
		}
		return résultats[i].index < résultats[j].index
	})

	if len(résultats) > limite {
		résultats = résultats[:limite]
	}
	out := make([]Contact, 0, len(résultats))
	for _, r := range résultats {
		out = append(out, r.contact)
	}
	return out
}

// provenanceNom indique que la saisie correspond au nom d'affichage.
func provenanceNom(c Contact, clée string) bool {
	nom := sansBruit(sansAccent(c.Nom))
	return nom != "" && (nom == clée || len([]rune(nom)) > len([]rune(clée)) && string([]rune(nom)[:len([]rune(clée))]) == clée)
}

// provenanceAdresse indique que la saisie correspond à l'adresse.
func provenanceAdresse(c Contact, clée string) bool {
	email := sansBruit(sansAccent(c.Email))
	return email != "" && (email == clée || len([]rune(email)) > len([]rune(clée)) && string([]rune(email)[:len([]rune(clée))]) == clée)
}

// noter renvoie le score d'un contact, ou -1 quand il ne correspond pas. Plus le
// score est bas, plus le contact est pertinent : l'égalité l'emporte sur le
// préfixe, le préfixe sur la sous-séquence, et le nom sur l'adresse.
func noter(contact Contact, clée string) int {
	runeClé := []rune(clée)

	meilleur := -1
	suivre := func(score int) {
		if score >= 0 && (meilleur < 0 || score < meilleur) {
			meilleur = score
		}
	}

	for _, champ := range []struct {
		texte string
		poids int
	}{
		{sansBruit(sansAccent(contact.Nom)), 0},
		{sansBruit(sansAccent(contact.Email)), 100},
	} {
		if champ.texte == "" {
			continue
		}

		runes := []rune(champ.texte)
		switch {
		case string(runes) == clée:
			suivre(champ.poids)
		case len(runes) > len(runeClé) && string(runes[:len(runeClé)]) == clée:
			suivre(champ.poids + 1)
		case sousSuite(runes, runeClé):
			suivre(champ.poids + 10 + len(runes) - len(runeClé))
		}
	}
	return meilleur
}

// sousSuite indique si les runes cherchées apparaissent dans l'ordre dans le
// texte, sans qu'aucune ne manque.
func sousSuite(texte, cherchées []rune) bool {
	i := 0
	for _, r := range texte {
		if i < len(cherchées) && r == cherchées[i] {
			i++
		}
	}
	return i == len(cherchées)
}
