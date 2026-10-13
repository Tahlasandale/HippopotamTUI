// Package rfc822 décode les octets RFC 5322 que himalaya rend avec
// `message read --raw`.
//
// Le TUI décode lui-même plutôt que de consommer le JSON de himalaya : ce JSON
// est la structure interne de l'outil, avec ses énumérations et ses offsets
// d'octets, et rien ne garantit qu'elle reste stable d'une version à l'autre.
// Les octets bruts, eux, sont normalisés par le protocole, et la bibliothèque
// standard suffit à les lire. C'est aussi ce qui permet de tester avec de vrais
// fichiers .eml, écrits à la main, dans testdata/.
package rfc822

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Message est un message décodé, prêt à être affiché.
type Message struct {
	Sujet     string
	De        string
	Vers      string
	Cc        string
	Bcc       string
	ReponseA  string
	MessageID string
	Date      time.Time

	// Corps est le texte à afficher. C'est la partie text/plain quand elle
	// existe, le HTML ramené à du texte sinon.
	Corps string

	// HTML est la partie text/html quand elle existe. Elle sert au repli et au
	// choix du rendu.
	HTML string

	// Pièces est la liste des pièces jointes. Le TUI les affiche mais ne les
	// télécharge pas.
	Pièces []Pièce

	// DejaMarkdown vaut vrai quand la partie textuelle est balisée en Markdown,
	// cas rare mais réel pour les notes de version.
	DejaMarkdown bool
}

// Pièce est une pièce jointe.
type Pièce struct {
	Nom    string
	Type   string
	Taille int
}

// EstMarkdown indique si le message mérite un rendu Markdown.
//
// La réponse est volontairement fausse pour un texte brut : passer un courriel
// ordinaire dans glamour produirait des titres et des tableaux là où il n'y a
// que des paragraphes.
func (m *Message) EstMarkdown() bool {
	return m.HTML != "" || m.DejaMarkdown
}

// Analyser décode un message complet.
func Analyser(brut []byte) (*Message, error) {
	lu, err := mail.ReadMessage(bytes.NewReader(brut))
	if err != nil {
		return nil, fmt.Errorf("lecture du message : %w", err)
	}

	msg := &Message{
		Sujet:     decodeEnTete(lu.Header.Get("Subject")),
		De:        adresseLisible(decodeEnTete(lu.Header.Get("From"))),
		Vers:      adresseLisible(decodeEnTete(lu.Header.Get("To"))),
		Cc:        adresseLisible(decodeEnTete(lu.Header.Get("Cc"))),
		Bcc:       adresseLisible(decodeEnTete(lu.Header.Get("Bcc"))),
		ReponseA:  adresseLisible(decodeEnTete(lu.Header.Get("Reply-To"))),
		MessageID: strings.Trim(lu.Header.Get("Message-Id"), "<> "),
	}

	msg.Date = LireDate(lu.Header.Get("Date"))

	msg.Corps, msg.HTML, msg.Pièces, msg.DejaMarkdown = analyserCorps(lu.Header, lu.Body)

	if msg.Corps == "" && msg.HTML != "" {
		msg.Corps = HTMLVersTexte(msg.HTML)
	}
	return msg, nil
}

// analyserCorps parcourt le corps du message, en descendant dans les parties
// imbriquées.
// adresseLisible retire les chevrons d'une adresse qui n'en a pas besoin :
// « <mailer@lists.sr.ht> » se lit « mailer@lists.sr.ht », et l'espace qui reste
// autour des plusieurs adresses ne sert à rien.
// sansGuillemets retire les guillemets d'un nom d'affichage, que le protocole
// exige mais que personne ne veut lire à l'écran.
func sansGuillemets(valeur string) string {
	if len(valeur) >= 2 && strings.HasPrefix(valeur, `"`) {
		if fin := strings.Index(valeur[1:], `"`); fin > 0 {
			return strings.TrimSpace(valeur[1:fin+1]) + valeur[fin+2:]
		}
	}
	return valeur
}

func adresseLisible(valeur string) string {
	parties := strings.Split(valeur, ",")
	for i, p := range parties {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && strings.HasPrefix(p, "<") && strings.HasSuffix(p, ">") && !strings.Contains(p[1:len(p)-1], "<") {
			p = p[1 : len(p)-1]
		}
		parties[i] = sansGuillemets(strings.TrimSpace(p))
	}
	return strings.Join(parties, ", ")
}

// analyserCorps parcourt le corps du message, en descendant dans les parties
// imbriquées.
// LireDate lit une date d'en-tête dans les formes qu'on rencontre : RFC 5322, celle
// du protocole, et les formes ISO que produisent certains générateurs et que
// himalaya rend dans ses enveloppes.
//
// Exported parce que la liste des messages a le même problème : une date qu'on ne
// sait pas lire s'affiche « ??/?? » à l'écran, ce qui est pire que de la deviner.
func LireDate(valeur string) time.Time {
	valeur = strings.TrimSpace(valeur)
	if valeur == "" {
		return time.Time{}
	}
	if t, err := mail.ParseDate(valeur); err == nil {
		return t
	}
	for _, format := range []string{
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(format, valeur); err == nil {
			return t
		}
	}
	return time.Time{}
}

// analyserCorps parcourt le corps d'un message ou d'une partie, en descendant
// dans les imbriquements.
//
// Le corps est lu en entier avant d'être interprété : c'est la seule façon de
// savoir si une frontière annoncée existe vraiment. Certains envois ajoutent un
// second en-tête Content-Type à une partie multipart en annonçant ensuite un
// texte, et la Norme, qui veut que seul le premier compte, décrit alors un corps
// vide. Prendre en compte ce qui est réellement là vaut mieux qu'un message vide.
func analyserCorps(entêtes mail.Header, corps io.Reader) (texte, html string, pièces []Pièce, dejaMarkdown bool) {
	brut, _ := io.ReadAll(corps)

	// Plusieurs Content-Type peuvent être annoncés pour la même partie. On les
	// essaie tous dans l'ordre, et on garde le premier qui donne quelque chose :
	// c'est ce qui permet de lire les messages dont la partie annonce un
	// multipart inexistant avant d'annoncer un texte.
	for _, valeur := range entêtes["Content-Type"] {
		typeMedia, params := typeEtParams(valeur)

		switch {
		case strings.HasPrefix(typeMedia, "multipart/"):
			limite := params["boundary"]
			if limite == "" {
				continue
			}
			texte, html = "", ""
			parcourirMultipart(bytes.NewReader(brut), limite, &texte, &html, &pièces, &dejaMarkdown)
			if texte != "" || html != "" || len(pièces) > 0 {
				return texte, html, pièces, dejaMarkdown
			}

		case strings.HasPrefix(typeMedia, "text/"):
			contenu := décoderTransfert(brut, entêtes.Get("Content-Transfer-Encoding"))
			if len(contenu) == 0 {
				continue
			}
			if typeMedia == "text/markdown" {
				dejaMarkdown = true
			}
			if typeMedia == "text/html" {
				return "", string(contenu), pièces, dejaMarkdown
			}
			return string(contenu), "", pièces, dejaMarkdown
		}
	}

	// Aucun type lisible. Deux cas restent, tous deux rencontrés dans la vraie
	// vie : un multipart annoncé dont la frontière n'existe pas, et des en-têtes
	// placés dans le corps au lieu de l'être dans la partie. Dans le second cas,
	// la partie est relue comme si quelqu'un les avait rédigés correctement.
	if imbriqués, reste := entêtesDansLeCorps(brut); imbriqués != nil {
		if texte, html, pièces, dejaMarkdown = analyserCorps(imbriqués, bytes.NewReader(reste)); texte != "" || html != "" {
			return texte, html, pièces, dejaMarkdown
		}
	}

	if contenu := décoderTransfert(brut, entêtes.Get("Content-Transfer-Encoding")); len(contenu) > 0 {
		return string(contenu), "", pièces, dejaMarkdown
	}
	return "", "", pièces, dejaMarkdown
}

// entêtesDansLeCorps repère des en-têtes MIME écrits au début du corps d'une
// partie, ce que font les listes de diffusion : la partie annonce un multipart
// inexistant, puis écrit ses vrais en-têtes et son texte à la suite.
//
// C'est maladroit, mais fréquent, et sans cela le message s'afficherait vide.
func entêtesDansLeCorps(brut []byte) (mail.Header, []byte) {
	texte := normaliserFinDeLigne(string(brut))
	lignes := strings.Split(texte, "\n")

	var lues []string
	vuEnTête := false
	for i, ligne := range lignes {
		tronquée := ligne
		if i == 0 {
			tronquée = strings.TrimPrefix(ligne, "\t")
		}
		if i > 0 && !estEnTête(tronquée) {
			break
		}
		if !estEnTête(tronquée) {
			return nil, brut
		}
		vuEnTête = true
		lues = append(lues, tronquée)
	}
	if !vuEnTête {
		return nil, brut
	}

	lu, err := mail.ReadMessage(strings.NewReader(strings.Join(lues, "\n") + "\n\n"))
	if err != nil {
		return nil, brut
	}

	// Le texte commence après la première ligne vide.
	joined := strings.Join(lignes, "\n")
	entête := joined[:strings.Index(joined, "\n\n")] + "\n\n"
	reste := joined[len(entête):]
	return lu.Header, []byte(reste)
}

// estEnTête reconnaît une ligne « Nom: valeur ».
func estEnTête(ligne string) bool {
	i := strings.Index(ligne, ":")
	if i <= 0 {
		return false
	}
	for _, c := range ligne[:i] {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && c != '-' {
			return false
		}
	}
	return true
}

// typeEtParams lit un Content-Type en tolérant les fautes.
//
// mime.ParseMediaType est strict : il échoue sur un paramètre sans valeur, comme
// « boundary=abc charset=utf-8 », que produisent certaines listes de diffusion.
// Échouer ici ferait passer une partie texte pour un contenu illisible, et le
// message s'afficherait avec ses propres en-têtes à l'écran. On relit donc à la
// main ce que la bibliothèque a refusé.
func typeEtParams(valeur string) (string, map[string]string) {
	if typeMedia, params, err := mime.ParseMediaType(valeur); err == nil && typeMedia != "" {
		return typeMedia, params
	}

	morceaux := strings.Split(valeur, ";")
	typeMedia := strings.ToLower(strings.TrimSpace(morceaux[0]))

	params := map[string]string{}
	for _, morceau := range morceaux[1:] {
		clef, val, trouvé := strings.Cut(morceau, "=")
		clef = strings.ToLower(strings.TrimSpace(clef))
		if clef == "" {
			continue
		}
		if trouvé {
			// La valeur s'arrête au premier espace : « boundary=abc charset=x »
			// décrit deux paramètres, pas une frontière contenant un espace.
			val, _, _ = strings.Cut(val, " ")
		}
		params[clef] = strings.Trim(strings.TrimSpace(val), `"`)
	}
	if typeMedia == "" {
		typeMedia = "text/plain"
	}
	return typeMedia, params
}

// parcourirMultipart descend dans les parties d'un multipart.
func parcourirMultipart(lecteur io.Reader, limite string, texte *string, html *string, pièces *[]Pièce, dejaMarkdown *bool) {
	mp := multipart.NewReader(lecteur, limite)
	for {
		partie, err := mp.NextPart()
		if err != nil {
			// Une partie tronquée ne doit pas faire perdre ce qu'on a déjà lu.
			return
		}

		disposition, dispParams, _ := mime.ParseMediaType(partie.Header.Get("Content-Disposition"))
		nom := dispParams["filename"]
		if nom == "" {
			nom = dispParams["name"]
		}
		if nom == "" {
			_, typeParams, _ := mime.ParseMediaType(partie.Header.Get("Content-Type"))
			nom = typeParams["name"]
		}

		brut, _ := io.ReadAll(partie)

		// Une partie est une pièce jointe dès qu'elle n'est pas du texte, ou dès
		// qu'elle est annoncée comme telle. Une pièce sans nom de fichier reste
		// donc une pièce, et le texte du message ne l'avale pas.
		// typePrincipal suffirait, mais il se rabat sur « application/octet-stream »
		// dès qu'un paramètre est mal écrit : on perdrait alors la frontière et la
		// partie deviendrait une fausse pièce jointe.
		typeMedia, _ := typeEtParams(partie.Header.Get("Content-Type"))
		estTexte := strings.HasPrefix(typeMedia, "text/")
		estComposite := strings.HasPrefix(typeMedia, "multipart/")

		if !estComposite && (disposition == "attachment" || (nom != "" && !estTexte) || (!estTexte && disposition != "inline")) {
			contenu := décoderTransfert(brut, partie.Header.Get("Content-Transfer-Encoding"))
			*pièces = append(*pièces, Pièce{
				Nom:    nom,
				Type:   typeMedia,
				Taille: len(contenu),
			})
			continue
		}

		sousTexte, sousHTML, sousPièces, sousMarkdown := analyserCorps(mail.Header(partie.Header), bytes.NewReader(brut))
		*dejaMarkdown = *dejaMarkdown || sousMarkdown
		*pièces = append(*pièces, sousPièces...)
		if sousTexte != "" && *texte == "" {
			*texte = sousTexte
		}
		if sousHTML != "" && *html == "" {
			*html = sousHTML
		}
	}
}

// décoderTransfert décode un contenu selon son encodage de transfert, puis
// normalise les fins de ligne.
func décoderTransfert(brut []byte, encodage string) []byte {
	var sortie io.Reader = bytes.NewReader(brut)

	switch {
	case strings.EqualFold(strings.TrimSpace(encodage), "base64"):
		sortie = base64.NewDecoder(base64.StdEncoding, bytes.NewReader(brut))
	case strings.EqualFold(strings.TrimSpace(encodage), "quoted-printable"):
		sortie = quotedprintable.NewReader(bytes.NewReader(brut))
	}

	contenu, err := io.ReadAll(sortie)
	if err != nil && len(contenu) == 0 {
		return nil
	}
	return []byte(normaliserFinDeLigne(string(contenu)))
}

func typePrincipal(valeur string) string {
	t, _, err := mime.ParseMediaType(valeur)
	if err != nil {
		return "application/octet-stream"
	}
	return t
}

// décodeur sert à décoder les en-têtes encodés en RFC 2047. WordDecoder a une
// méthode à pointeur, il lui faut donc une variable et non une valeur.
var décodeur mime.WordDecoder

// decodeEnTete décode les en-têtes RFC 2047 et normalise les fins de ligne.
func decodeEnTete(valeur string) string {
	if valeur == "" {
		return ""
	}
	decoded := &strings.Builder{}
	lignes := strings.Split(valeur, "\n")
	for i, ligne := range lignes {
		if i > 0 {
			decoded.WriteString(" ")
		}
		decoded.WriteString(strings.TrimSpace(ligne))
	}
	valeur = decoded.String()

	if d, err := décodeur.DecodeHeader(valeur); err == nil {
		valeur = d
	}
	return normaliserFinDeLigne(valeur)
}

func normaliserFinDeLigne(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// HTMLVersTexte ramène une page HTML à un texte lisible dans un terminal.
//
// Le style et les scripts sont écartés, les blocs séparent des lignes, les
// entités HTML sont décodées et les espaces multiples réduits. C'est le strict
// nécessaire : on n'a pas besoin d'un moteur de rendu, seulement de pouvoir lire
// la newsletter qui n'a pas su envoyer de texte brut.
func HTMLVersTexte(source string) string {
	nœud, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return normaliserFinDeLigne(stripperDeBalises(source))
	}

	var out strings.Builder
	var parser func(*html.Node)
	parser = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			switch n.Data {
			case "script", "style", "head", "title":
				// Ces éléments ne portent pas de texte destiné au lecteur : on ne
				// descend pas dedans, sans quoi le script d'une page s'afficherait.
				return
			case "br":
				out.WriteByte('\n')
				return
			}
		case html.TextNode:
			out.WriteString(n.Data)
			return
		}

		for enfant := n.FirstChild; enfant != nil; enfant = enfant.NextSibling {
			parser(enfant)
		}

		if n.Type == html.ElementNode && estBloc(n.Data) {
			out.WriteByte('\n')
		}
	}
	parser(nœud)

	return nettoyerTexte(out.String())
}

func estBloc(balise string) bool {
	switch balise {
	case "p", "div", "section", "article", "header", "footer", "main", "aside",
		"h1", "h2", "h3", "h4", "h5", "h6", "li", "tr", "blockquote",
		"pre", "ul", "ol", "table", "hr":
		return true
	}
	return false
}

// stripperDeBalises est le repli quand le HTML ne peut même pas être analysé.
func stripperDeBalises(source string) string {
	var out strings.Builder
	profondeur := 0
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '<':
			profondeur++
		case '>':
			if profondeur > 0 {
				profondeur--
				out.WriteByte('\n')
			}
		default:
			if profondeur == 0 {
				out.WriteByte(source[i])
			}
		}
	}
	return html.UnescapeString(out.String())
}

// nettoyerTexte réduit les espaces et supprime les lignes vides en série.
func nettoyerTexte(source string) string {
	texte := normaliserFinDeLigne(source)
	texte = strings.ReplaceAll(texte, "\r\n", "\n")

	lignes := strings.Split(texte, "\n")
	nettoyées := make([]string, 0, len(lignes))
	for _, ligne := range lignes {
		ligne = strings.Join(strings.Fields(ligne), " ")
		ligne = strings.TrimSpace(ligne)
		nettoyées = append(nettoyées, ligne)
	}

	var out []string
	for _, ligne := range nettoyées {
		if ligne == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		out = append(out, ligne)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
