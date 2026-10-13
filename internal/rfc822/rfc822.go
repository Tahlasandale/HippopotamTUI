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
		De:        decodeEnTete(lu.Header.Get("From")),
		Vers:      decodeEnTete(lu.Header.Get("To")),
		Cc:        decodeEnTete(lu.Header.Get("Cc")),
		Bcc:       decodeEnTete(lu.Header.Get("Bcc")),
		ReponseA:  decodeEnTete(lu.Header.Get("Reply-To")),
		MessageID: strings.Trim(lu.Header.Get("Message-Id"), "<> "),
	}

	if date, err := mail.ParseDate(lu.Header.Get("Date")); err == nil {
		msg.Date = date
	}

	msg.Corps, msg.HTML, msg.Pièces, msg.DejaMarkdown = analyserCorps(lu)

	if msg.Corps == "" && msg.HTML != "" {
		msg.Corps = HTMLVersTexte(msg.HTML)
	}
	return msg, nil
}

// analyserCorps parcourt le corps du message, en descendant dans les parties
// imbriquées.
func analyserCorps(lu *mail.Message) (texte, html string, pièces []Pièce, dejaMarkdown bool) {
	typeMedia, params, err := mime.ParseMediaType(lu.Header.Get("Content-Type"))
	if err != nil {
		typeMedia = "text/plain"
		params = map[string]string{}
	}
	if typeMedia == "" {
		typeMedia = "text/plain"
	}
	if len(lu.Header["Content-Type"]) > 0 && strings.Contains(strings.ToLower(lu.Header.Get("Content-Type")), "text/markdown") {
		dejaMarkdown = true
	}

	corps := lu.Body

	if strings.HasPrefix(typeMedia, "multipart/") {
		limite := params["boundary"]
		if limite != "" {
			if err := parcourirMultipart(corps, limite, &texte, &html, &pièces, &dejaMarkdown); err == nil {
				return texte, html, pièces, dejaMarkdown
			}
		}
		// Sans frontière exploitable, on retombe sur une lecture brute plutôt que
		// d'afficher une erreur à l'utilisateur.
	}

	if strings.HasPrefix(typeMedia, "text/") {
		contenu := lire(corps, lu.Header.Get("Content-Transfer-Encoding"))
		if typeMedia == "text/html" {
			return "", string(contenu), pièces, dejaMarkdown
		}
		return string(contenu), "", pièces, dejaMarkdown
	}

	return "", "", pièces, dejaMarkdown
}

func parcourirMultipart(lecteur io.Reader, limite string, texte *string, html *string, pièces *[]Pièce, dejaMarkdown *bool) error {
	mp := multipart.NewReader(lecteur, limite)
	for {
		partie, err := mp.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
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

		if disposition == "attachment" || (nom != "" && !strings.HasPrefix(partie.Header.Get("Content-Type"), "text/")) {
			contenu := lire(partie, partie.Header.Get("Content-Transfer-Encoding"))
			*pièces = append(*pièces, Pièce{
				Nom:    nom,
				Type:   typePrincipal(partie.Header.Get("Content-Type")),
				Taille: len(contenu),
			})
			continue
		}

		sousTexte, sousHTML, sousPièces, sousMarkdown := analyserCorps(&mail.Message{
			Header: mail.Header(partie.Header),
			Body:   partie,
		})
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

func typePrincipal(valeur string) string {
	t, _, err := mime.ParseMediaType(valeur)
	if err != nil {
		return "application/octet-stream"
	}
	return t
}

// lire décode un contenu selon son encodage de transfert puis le charset
// annoncé.
func lire(lecteur io.Reader, encodage string) []byte {
	var brut io.Reader = lecteur

	switch {
	case strings.EqualFold(strings.TrimSpace(encodage), "base64"):
		décodé := base64.NewDecoder(base64.StdEncoding, lecteur)
		brut = décodé
	case strings.EqualFold(strings.TrimSpace(encodage), "quoted-printable"):
		brut = quotedprintable.NewReader(lecteur)
	}

	contenu, err := io.ReadAll(brut)
	if err != nil && len(contenu) == 0 {
		return nil
	}
	return []byte(normaliserFinDeLigne(string(contenu)))
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
