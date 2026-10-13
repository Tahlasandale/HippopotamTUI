package rfc822

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, nom string) *Message {
	t.Helper()
	brut, err := os.ReadFile(filepath.Join("testdata", nom))
	if err != nil {
		t.Fatalf("fixture %s : %v", nom, err)
	}
	msg, err := Analyser(brut)
	if err != nil {
		t.Fatalf("Analyser(%s) : %v", nom, err)
	}
	return msg
}

func TestEnTêtesDUnMessageSimple(t *testing.T) {
	msg := fixture(t, "simple.eml")

	if msg.Sujet != "Un message tout simple" {
		t.Errorf("Sujet = %q", msg.Sujet)
	}
	if msg.De != "Alice Martin <alice@example.org>" {
		t.Errorf("De = %q", msg.De)
	}
	if msg.Vers != "Orphin <orphin@protonmail.com>" {
		t.Errorf("Vers = %q", msg.Vers)
	}
	if !strings.HasPrefix(msg.Corps, "Bonjour,") {
		t.Errorf("Corps = %q", msg.Corps)
	}
	if !strings.Contains(msg.Corps, "éàü") {
		t.Errorf("les accents du corps ont été perdus : %q", msg.Corps)
	}
	if msg.HTML != "" {
		t.Errorf("HTML = %q, veut vide pour un message sans partie HTML", msg.HTML)
	}

	attendu := time.Date(2024, 4, 17, 6, 6, 28, 0, time.FixedZone("CEST", 2*3600))
	if !msg.Date.Equal(attendu) {
		t.Errorf("Date = %v, veut %v", msg.Date, attendu)
	}
}

// l'objet encodé en RFC 2047 doit être décodé, pas affiché tel quel.

func TestObjetEncodé(t *testing.T) {
	msg := fixture(t, "alternative.eml")
	if msg.Sujet != "Café : réunion annulée" {
		t.Errorf("Sujet = %q, veut Café : réunion annulée", msg.Sujet)
	}
}

// le quoted-printable doit être décodé, les sauts de ligne normalisés.

func TestQuotedPrintable(t *testing.T) {
	msg := fixture(t, "alternative.eml")

	if !strings.Contains(msg.Corps, "café – Non") {
		t.Errorf("Corps = %q, veut le quoted-printable décodé", msg.Corps)
	}
	// une coupure douce relie deux lignes physiques sans laisser de retour à la
	// ligne : c'est le piège classique du quoted-printable.
	if !strings.Contains(msg.Corps, "volontairement coupée en deux lignes physiques.") {
		t.Errorf("Corps = %q, veut les deux lignes physiques recollées", msg.Corps)
	}
	if strings.Contains(msg.Corps, "=C3") {
		t.Errorf("Corps = %q, veut plus aucune séquence d'encodage", msg.Corps)
	}
	if strings.Contains(msg.Corps, "\r") {
		t.Error("les fins de ligne CRLF doivent devenir \\n")
	}
}

func TestPartiesMultiples(t *testing.T) {
	msg := fixture(t, "alternative.eml")

	if !strings.Contains(msg.HTML, "<b>réunion</b>") {
		t.Errorf("HTML = %q, veut la partie HTML conservée", msg.HTML)
	}
	if strings.Contains(msg.Corps, "<b>") {
		t.Errorf("Corps = %q, veut le texte brut et rien d'autre", msg.Corps)
	}
}

// un message en HTML seul doit être ramené à du texte lisible : le TUI affiche
// dans un terminal, il ne rend pas de page web.

func TestRepliSurHTML(t *testing.T) {
	msg := fixture(t, "htmlseul.eml")

	for _, attendu := range []string{"Le programme", "Ouverture", "Finale", "Un", "Deux", "À bientôt & cordialement"} {
		if !strings.Contains(msg.Corps, attendu) {
			t.Errorf("Corps = %q, veut contenir %q", msg.Corps, attendu)
		}
	}
	for _, unwanted := range []string{"<h1>", "<style>", "color: red", "alert(", "ceci ne doit pas apparaître"} {
		if strings.Contains(msg.Corps, unwanted) {
			t.Errorf("Corps = %q, ne doit pas contenir %q", msg.Corps, unwanted)
		}
	}
}

// le HTML doit rester un texte à paragraphes, pas une suite de mots accolés.

func TestRepliSurHTMLGardeLesBlocs(t *testing.T) {
	msg := fixture(t, "htmlseul.eml")

	if !strings.Contains(msg.Corps, "\n") {
		t.Errorf("Corps = %q, veut des retours à la ligne entre les blocs", msg.Corps)
	}
	if strings.Contains(msg.Corps, "  ") {
		t.Errorf("Corps = %q, veut les espaces multiples réduits", msg.Corps)
	}
}

// les pièces jointes sont listées, jamais décodées dans le texte : hors périmètre
// de la v1, mais l'utilisateur doit les voir.

func TestPiècesJointes(t *testing.T) {
	msg := fixture(t, "attaches.eml")

	if len(msg.Pièces) != 2 {
		t.Fatalf("len(Pieces) = %d, veut 2", len(msg.Pièces))
	}
	if msg.Pièces[0].Nom != "attestation.pdf" {
		t.Errorf("Nom = %q", msg.Pièces[0].Nom)
	}
	if msg.Pièces[0].Type != "application/pdf" {
		t.Errorf("Type = %q", msg.Pièces[0].Type)
	}
	// 20 caractères de base64 font 15 octets une fois décodés.
	if msg.Pièces[0].Taille != 15 {
		t.Errorf("Taille = %d, veut 15 octets une fois le base64 décodé", msg.Pièces[0].Taille)
	}
	if msg.Pièces[1].Nom != "logo.png" {
		t.Errorf("Nom = %q, une pièce inline compte aussi", msg.Pièces[1].Nom)
	}
}

// le corps textuel d'un message en plusieurs parties ne doit pas être pollué par
// les pièces jointes.

func TestCorpsIgnorantLesPièces(t *testing.T) {
	msg := fixture(t, "attaches.eml")

	if !strings.Contains(msg.Corps, "attestation est en pièce jointe") {
		t.Errorf("Corps = %q", msg.Corps)
	}
	if strings.Contains(msg.Corps, "JVBERi0x") || strings.Contains(msg.Corps, "iVBORw0") {
		t.Errorf("Corps = %q, ne doit contenir aucune pièce jointe", msg.Corps)
	}
	// le contenu était en iso-8859-1 : l'accent de « À conserver » doit survivre.
	if !strings.Contains(msg.Corps, "À conserver") {
		t.Errorf("Corps = %q, veut le charset iso-8859-1 converti", msg.Corps)
	}
}

func TestDateAbsente(t *testing.T) {
	msg, err := Analyser([]byte("Subject: Sans date\r\n\r\ncoupé\r\n"))
	if err != nil {
		t.Fatalf("Analyser : %v", err)
	}
	if !msg.Date.IsZero() {
		t.Errorf("Date = %v, veut la date nulle", msg.Date)
	}
	if strings.TrimSpace(msg.Corps) != "coupé" {
		t.Errorf("Corps = %q", msg.Corps)
	}
}

func TestMessageInvalide(t *testing.T) {
	if _, err := Analyser([]byte("ceci n'est pas un message du tout")); err == nil {
		t.Fatal("Analyser aurait dû échouer sur une entrée sans en-tête")
	}
}

// le rendu Markdown ne doit se demander que si la partie est réellement balisée
// ou si le message est du HTML : du texte brut ne doit jamais être réinterprété.

func TestFormatage(t *testing.T) {
	if fixture(t, "simple.eml").EstMarkdown() {
		t.Error("un texte brut n'est pas du Markdown")
	}
	if !fixture(t, "htmlseul.eml").EstMarkdown() {
		t.Error("un message HTML a interest à être rendu avec glamour")
	}
}

func TestSansSujet(t *testing.T) {
	msg, err := Analyser([]byte("From: a@b.c\r\n\r\ncorps"))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Sujet != "" {
		t.Errorf("Sujet = %q, veut vide", msg.Sujet)
	}
}

// une partie peut déclarer deux fois Content-Type — un multipart puis un texte —
// et c'est le second qui décrit réellement le contenu. La Norme ne prévoit rien
// pour ce cas, mais les envois réels le produisent, et afficher un message vide
// serait pire que de lire ce qui est là.
func TestContentTypeEnDouble(t *testing.T) {
	brut := "Message-Id: <liste@example.org>\r\n" +
		"From: <mailer@lists.example.org>\r\n" +
		"Subject: Confirmation\r\n" +
		"Content-Type: multipart/mixed;\r\n boundary=externe\r\n" +
		" charset=utf-8\r\n" +
		"\r\n" +
		"--externe\r\n" +
		"Content-Type: multipart/mixed;\r\n boundary=interne\r\n" +
		" charset=utf-8\r\n" +
		"\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"Content-Type: text/plain; charset=UTF-8; format=flowed\r\n" +
		"\r\n" +
		"Votre inscription est confirm=C3=A9e.\r\n" +
		"--interne--\r\n" +
		"--externe--\r\n"

	msg, err := Analyser([]byte(brut))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg.Corps, "Votre inscription est confirmée.") {
		t.Errorf("Corps = %q, le texte était pourtant bien là", msg.Corps)
	}
	if msg.De != "mailer@lists.example.org" {
		t.Errorf("De = %q, veut l'adresse sans chevrons", msg.De)
	}
}

// plusieurs destinataires sur un seul en-tête.
func TestDestinatairesMultiples(t *testing.T) {
	brut := "From: a@b.c\r\nTo: Alice <alice@example.org>, bob@example.org\r\nCc: \"Carla\" <carla@example.org>\r\nSubject: x\r\n\r\ncorps\r\n"
	msg, err := Analyser([]byte(brut))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Vers != "Alice <alice@example.org>, bob@example.org" {
		t.Errorf("Vers = %q", msg.Vers)
	}
	if msg.Cc != "Carla <carla@example.org>" {
		t.Errorf("Cc = %q", msg.Cc)
	}
}

// mime.ParseMediaType est strict : il refuse « boundary=x charset=y ». La
// frontière doit pourtant être retrouvée, sinon le message s'affiche avec ses
// propres en-têtes.
func TestParamètreSansValeur(t *testing.T) {
	typeMedia, params := typeEtParams("multipart/mixed; boundary=abc123 charset=utf-8")

	if typeMedia != "multipart/mixed" {
		t.Errorf("typeMedia = %q", typeMedia)
	}
	if params["boundary"] != "abc123" {
		t.Errorf("boundary = %q, veut abc123", params["boundary"])
	}
}

// une pièce jointe sans nom de fichier reste une pièce jointe.
func TestPièceJointeSansNom(t *testing.T) {
	brut := "Content-Type: multipart/mixed; boundary=c\r\n\r\n" +
		"--c\r\nContent-Type: text/plain\r\n\r\ncorps\r\n" +
		"--c\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQ=\r\n" +
		"--c--\r\n"

	msg, err := Analyser([]byte(brut))
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Pièces) != 1 {
		t.Fatalf("Pièces = %+v, veut une pièce", msg.Pièces)
	}
	if msg.Pièces[0].Type != "application/pdf" {
		t.Errorf("Type = %q", msg.Pièces[0].Type)
	}
	if !strings.Contains(msg.Corps, "corps") {
		t.Errorf("Corps = %q", msg.Corps)
	}
}
