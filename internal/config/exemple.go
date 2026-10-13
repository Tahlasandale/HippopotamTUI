package config

import (
	"fmt"
	"strings"
)

// Exemple rend un fichier de configuration commenté, avec les valeurs en cours
// d'usage. C'est ce que « hippo config init » écrit : quelqu'un qui lit le
// fichier doit comprendre chaque ligne sans jamais ouvrir le code.
func Exemple(c Config) string {
	var b strings.Builder

	b.WriteString(`# Configuration de HippopotamTUI
#
# Ce fichier ne contient que des préférences d'ergonomie. Les mots de passe et
# les comptes restent dans la configuration de himalaya, que hippo ne lit
# jamais : il ne lui parle que par la ligne de commande.
#
# Toutes les clés ont un défaut qui fonctionne. Vous pouvez supprimer ce fichier
# entièrement, hippo continuera de tourner.

# Chemin du binaire himalaya. À changer seulement s'il n'est pas dans le PATH.
himalaya = "` + c.Himalaya + `"

# Éditeur utilisé pour écrire le corps d'un message. Vide : hippo prend $EDITOR,
# puis micro, nvim, vim, vi, dans cet ordre.
editor = "` + c.Editor + `"

# Nombre de messages demandés à himalaya par page.
page_size = ` + fmt.Sprint(c.PageSize) + `

# Minutes au-delà desquelles la liste affichée est rechargée depuis le serveur.
# Une valeur basse rend l'interface plus vive mais sollicite plus le compte mail.
refresh_minutes = ` + fmt.Sprint(c.RefreshMinutes) + `

# Jours avant que le carnet d'adresses soit reconstruit au démarrage.
contacts_refresh_days = ` + fmt.Sprint(c.ContactsRefreshDays) + `

# Surcharge ponctuelle d'une touche. Clé : nom de l'action, valeur : touche.
# Les noms d'actions sont ceux de docs/raccourcis.md.
# Exemple :
# [keys]
# quitter = "ctrl+x"
`)
	return b.String()
}

// Expliquer rend la configuration en cours avec les valeurs par défaut, pour
// « hippo config afficher ».
func Expliquer(c Config) string {
	var b strings.Builder

	ligne := func(nom, valeur, défaut string) {
		fmt.Fprintf(&b, "  %-22s %-30s (défaut : %s)\n", nom, valeur, défaut)
	}

	b.WriteString("Configuration en vigueur :\n")
	d := Default()
	ligne("himalaya", c.Himalaya, d.Himalaya)
	ligne("editor", videOu(c.Editor), videOu(d.Editor))
	ligne("page_size", fmt.Sprint(c.PageSize), fmt.Sprint(d.PageSize))
	ligne("refresh_minutes", fmt.Sprint(c.RefreshMinutes), fmt.Sprint(d.RefreshMinutes))
	ligne("contacts_refresh_days", fmt.Sprint(c.ContactsRefreshDays), fmt.Sprint(d.ContactsRefreshDays))

	if len(c.Keys) == 0 {
		b.WriteString("  aucune touche surchargeée\n")
		return b.String()
	}
	b.WriteString("  touches surchargeées :\n")
	for _, nom := range touchesTriées(c.Keys) {
		fmt.Fprintf(&b, "    %-22s %s\n", nom, c.Keys[nom])
	}
	return b.String()
}

func videOu(s string) string {
	if s == "" {
		return "(détection automatique)"
	}
	return s
}

func touchesTriées(keys map[string]string) []string {
	noms := make([]string, 0, len(keys))
	for nom := range keys {
		noms = append(noms, nom)
	}
	for i := 0; i < len(noms); i++ {
		for j := i + 1; j < len(noms); j++ {
			if noms[j] < noms[i] {
				noms[i], noms[j] = noms[j], noms[i]
			}
		}
	}
	return noms
}
