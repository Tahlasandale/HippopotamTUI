package himalaya

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// Le client est construit avec un faux exécuteur : aucun appel réseau, aucune
// dépendance à la présence de l'exécutable himalaya pendant les tests.

// fauxExec enregistre les commandes et renvoie des réponses préparées.
type fauxExec struct {
	appels     [][]string
	réponses   map[string]string
	dernière   []string
	codeErreur int
}

func (f *fauxExec) run(_ context.Context, nom string, args ...string) ([]byte, error) {
	ligne := append([]string{nom}, args...)
	f.appels = append(f.appels, ligne)
	f.dernière = ligne

	if f.codeErreur != 0 {
		return []byte("échec simulé\n"), &ErreurHimalaya{
			Code: 1,
			Msg:  "échec simulé",
		}
	}

	for cle, valeur := range f.réponses {
		if strings.Contains(strings.Join(ligne, " "), cle) {
			return []byte(valeur), nil
		}
	}
	return nil, errors.New("aucune réponse préparée pour " + strings.Join(ligne, " "))
}

func clientAvec(f *fauxExec) *CLI {
	return &CLI{Bin: "himalaya", run: f.run}
}

func TestComptes(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{
		"account list": `{"accounts":[{"name":"proton","default":true,"backends":["imap","smtp"]},{"name":"gmail","default":false,"backends":["imap","smtp"]}]}`,
	}}
	c := clientAvec(f)

	comptes, err := c.Comptes(context.Background())
	if err != nil {
		t.Fatalf("Comptes : %v", err)
	}
	if len(comptes) != 2 {
		t.Fatalf("len = %d, veut 2", len(comptes))
	}
	if comptes[0].Nom != "proton" || !comptes[0].Defaut {
		t.Errorf("compte 0 = %+v, veut proton par défaut", comptes[0])
	}
	if len(f.appels[0]) == 0 || f.appels[0][1] != "account" {
		t.Errorf("appel = %v, veut account list", f.appels[0])
	}
}

// le compte par défaut est celui que himalaya déclare tel quel : le TUI ne
// devine rien à partir de l'ordre des clés.

func TestCompteParDéfaut(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{
		"account list": `{"accounts":[{"name":"gmail","default":false},{"name":"proton","default":true}]}`,
	}}
	comptes, err := clientAvec(f).Comptes(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	nom, ok := CompteParDéfaut(comptes)
	if !ok || nom != "proton" {
		t.Errorf("CompteParDéfaut = %q, %v ; veut proton, true", nom, ok)
	}

	vides, _ := clientAvec(&fauxExec{réponses: map[string]string{"account list": `{"accounts":[]}`}}).Comptes(context.Background())
	if _, ok := CompteParDéfaut(vides); ok {
		t.Error("CompteParDéfaut ne devrait rien retourner sur une liste vide")
	}
}

func TestBoites(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{
		"mailbox list": `{"mailboxes":[{"id":"Inbox","name":"Inbox","role":"inbox"},{"id":"Sent","name":"Sent"}]}`,
	}}
	c := clientAvec(f)

	boites, err := c.Boites(context.Background(), "proton")
	if err != nil {
		t.Fatalf("Boites : %v", err)
	}
	if len(boites) != 2 {
		t.Fatalf("len = %d, veut 2", len(boites))
	}
	if boites[0].Role != "inbox" {
		t.Errorf("role = %q, veut inbox", boites[0].Role)
	}

	appel := strings.Join(f.appels[0], " ")
	for _, attendu := range []string{"--json", "-a", "proton"} {
		if !strings.Contains(appel, attendu) {
			t.Errorf("l'appel %q ne contient pas %q", appel, attendu)
		}
	}
}

const jsonEnveloppes = `{"envelopes":[
{"id":"1550","message-id":"<a@b>","flags":[{"raw":"\\Seen","iana":"seen"}],"subject":"Bonjour","from":[{"name":"Alice","email":"alice@example.org"}],"to":[{"name":null,"email":"orphin@protonmail.com"}],"date":"2026-10-13T22:14:32+02:00","size":819,"has-attachment":true},
{"id":"1549","message-id":"<c@d>","flags":[],"subject":"Sans drapeaux","from":[{"email":"bob@example.org"}],"to":[],"date":"2026-10-12T10:00:00+02:00","size":1024}]}`

func TestEnveloppes(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"envelope list": jsonEnveloppes}}
	c := clientAvec(f)

	envs, err := c.Enveloppes(context.Background(), "proton", "Inbox", 2, 50)
	if err != nil {
		t.Fatalf("Enveloppes : %v", err)
	}
	if len(envs) != 2 {
		t.Fatalf("len = %d, veut 2", len(envs))
	}
	if envs[0].ID != "1550" || envs[0].Sujet != "Bonjour" {
		t.Errorf("enveloppe 0 = %+v", envs[0])
	}
	if !envs[0].A(PoussièreVolee) {
		t.Error("1550 devrait porter le drapeau seen")
	}
	if envs[0].PiecesJointes != true {
		t.Errorf("PiecesJointes = %v, veut true", envs[0].PiecesJointes)
	}
	if envs[1].A(PoussièreVolee) {
		t.Error("1549 ne porte aucun drapeau")
	}

	appel := strings.Join(f.appels[0], " ")
	for _, attendu := range []string{"envelope", "list", "-m", "Inbox", "-p", "2", "-s", "50"} {
		if !strings.Contains(appel, attendu) {
			t.Errorf("l'appel %q ne contient pas %q", appel, attendu)
		}
	}
}

// une recherche sans requête se comporte comme une liste : himalaya accepte les
// deux, mais on n'envoie pas de paramètres vides.

func TestRechercher(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"envelope search": jsonEnveloppes}}
	c := clientAvec(f)

	envs, err := c.Rechercher(context.Background(), "proton", "Inbox", `from alice and not flag seen`, 1, 25)
	if err != nil {
		t.Fatalf("Rechercher : %v", err)
	}
	if len(envs) != 2 {
		t.Fatalf("len = %d, veut 2", len(envs))
	}

	appel := f.appels[0]
	if appel[1] != "envelope" || appel[2] != "search" {
		t.Errorf("appel = %v, veut envelope search", appel)
	}
	trouvé := false
	for _, a := range appel {
		if a == "from alice and not flag seen" {
			trouvé = true
		}
	}
	if !trouvé {
		t.Errorf("la requête n'a pas été transmise telle quelle : %v", appel)
	}
}

func TestMessageBrut(t *testing.T) {
	brut := "Subject: Test\r\n\r\ncorps\r\n"
	f := &fauxExec{réponses: map[string]string{"message read": brut}}

	octets, err := clientAvec(f).Message(context.Background(), "proton", "Inbox", "1550")
	if err != nil {
		t.Fatalf("Message : %v", err)
	}
	if string(octets) != brut {
		t.Errorf("corps = %q, veut %q", octets, brut)
	}

	appel := strings.Join(f.appels[0], " ")
	if strings.Contains(appel, "--json") {
		t.Errorf("la lecture doit rester brute, sans --json : %q", appel)
	}
	if !strings.Contains(appel, "--raw") {
		t.Errorf("l'appel %q ne demande pas --raw", appel)
	}
}

func TestPoserEtRetirerDrapeau(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"flag": `{"success":true}`}}
	c := clientAvec(f)
	ctx := context.Background()

	if err := c.PoserDrapeau(ctx, "proton", "Inbox", []string{"1", "2"}, "flagged"); err != nil {
		t.Fatalf("PoserDrapeau : %v", err)
	}
	if err := c.RetirerDrapeau(ctx, "proton", "Inbox", []string{"1"}, "seen"); err != nil {
		t.Fatalf("RetirerDrapeau : %v", err)
	}

	if appel := f.appels[0]; appel[1] != "flag" || appel[2] != "add" {
		t.Errorf("appel 0 = %v, veut flag add", appel)
	}
	rejoint := strings.Join(f.appels[0], " ")
	if !strings.Contains(rejoint, "1 2") {
		t.Errorf("les deux identifiants ne sont pas transmis : %q", rejoint)
	}
	if appel := f.appels[1]; appel[2] != "remove" {
		t.Errorf("appel 1 = %v, veut flag remove", appel)
	}
}

func TestDeplacerEtSupprimer(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"message": `{"success":true}`}}
	c := clientAvec(f)
	ctx := context.Background()

	if err := c.Deplacer(ctx, "proton", "Inbox", "Archive", []string{"7"}); err != nil {
		t.Fatalf("Deplacer : %v", err)
	}
	if err := c.Supprimer(ctx, "proton", "Inbox", []string{"7"}); err != nil {
		t.Fatalf("Supprimer : %v", err)
	}

	appel := strings.Join(f.appels[0], " ")
	if !strings.Contains(appel, "move") || !strings.Contains(appel, "--to Archive") {
		t.Errorf("appel 0 = %q, veut message move --to Archive", appel)
	}
	if appel := f.appels[1]; appel[2] != "delete" {
		t.Errorf("appel 1 = %v, veut message delete", appel)
	}
}

const jsonBrouillon = `{"from":"Orphin <orphin@protonmail.com>","to":["a@x.org"],"cc":["b@x.org"],"bcc":[],"subject":"Re: Test","body":"> cite\n"}`

func TestSqueletteReponse(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"message reply": jsonBrouillon}}
	c := clientAvec(f)

	br, err := c.Squelette(context.Background(), Origine{Kind: Reponse, ID: "1550"}, "proton", "Inbox")
	if err != nil {
		t.Fatalf("Squelette : %v", err)
	}
	if br.Sujet != "Re: Test" {
		t.Errorf("Sujet = %q, veut Re: Test", br.Sujet)
	}
	if !reflect.DeepEqual(br.Cc, []string{"b@x.org"}) {
		t.Errorf("Cc = %v", br.Cc)
	}
	if br.Corps != "> cite\n" {
		t.Errorf("Corps = %q", br.Corps)
	}

	appel := f.appels[0]
	if appel[1] != "message" || appel[2] != "reply" || appel[3] != "1550" {
		t.Errorf("appel = %v, veut message reply 1550", appel)
	}
}

func TestSqueletteNouveauMessage(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"message compose": jsonBrouillon}}
	c := clientAvec(f)

	if _, err := c.Squelette(context.Background(), Origine{Kind: Nouveau}, "proton", ""); err != nil {
		t.Fatalf("Squelette : %v", err)
	}
	if f.appels[0][1] != "message" || f.appels[0][2] != "compose" {
		t.Errorf("appel = %v, veut message compose", f.appels[0])
	}
}

// l'envoi reprend la commande du squelette, en lui passant les champs que le
// TUI a pu modifier : c'est himalaya qui réécrit le Re: et les en-têtes de
// threading, jamais le TUI.

func TestEnvoiRemplaceDestinataires(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"message reply": `{"success":true}`}}
	c := clientAvec(f)

	env := Envoi{
		Origine: Origine{Kind: Reponse, ID: "1550"},
		To:      []string{"alice@example.org"},
		Cc:      []string{"bob@example.org"},
		Sujet:   "Re: Test",
		Fichier: "/tmp/hippo-corps.txt",
	}
	if err := c.Envoyer(context.Background(), env, "proton", "Inbox"); err != nil {
		t.Fatalf("Envoyer : %v", err)
	}

	appel := f.appels[0]
	rejoint := strings.Join(appel, " ")
	for _, attendu := range []string{"message reply 1550", "--to alice@example.org", "--cc bob@example.org",
		"-s Re: Test", "--body-file /tmp/hippo-corps.txt", "--send"} {
		if !strings.Contains(rejoint, attendu) {
			t.Errorf("l'appel %q ne contient pas %q", rejoint, attendu)
		}
	}
}

func TestEnvoiNouveauMessageEnregistreEnBrouillon(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"message compose": `{"success":true}`}}
	c := clientAvec(f)

	env := Envoi{
		Origine: Origine{Kind: Nouveau},
		To:      []string{"alice@example.org"},
		Fichier: "/tmp/hippo-corps.txt",
	}
	if err := c.Enregistrer(context.Background(), env, "proton", ""); err != nil {
		t.Fatalf("Enregistrer : %v", err)
	}

	rejoint := strings.Join(f.appels[0], " ")
	if !strings.Contains(rejoint, "message compose") {
		t.Errorf("appel = %q, veut message compose", rejoint)
	}
	if !strings.Contains(rejoint, "--save Drafts") {
		t.Errorf("l'appel %q ne sauvegarde pas en brouillon", rejoint)
	}
	if strings.Contains(rejoint, "--send") {
		t.Error("un brouillon ne doit jamais partir")
	}
}

// une liste de destinataires vide ne doit pas produire un --to sans valeur, que
// himalaya refuserait.

func TestEnvoiSansDestinataire(t *testing.T) {
	f := &fauxExec{réponses: map[string]string{"message compose": `{"success":true}`}}
	c := clientAvec(f)

	err := c.Envoyer(context.Background(), Envoi{Origine: Origine{Kind: Nouveau}, Fichier: "/tmp/x"}, "proton", "")
	if err == nil {
		t.Fatal("l'envoi sans destinataire aurait dû échouer")
	}
	if !strings.Contains(err.Error(), "destinataire") {
		t.Errorf("erreur = %q, veut une mention de destinataire", err)
	}
	if len(f.appels) != 0 {
		t.Errorf("aucun appel ne devait être lancé, vu : %v", f.appels)
	}
}

// l'erreur de himalaya doit remonter telle quelle : le bandeau d'erreur du TUI
// affiche le message de l'outil, pas un « exit status 1 ».

func TestErreurPropagée(t *testing.T) {
	f := &fauxExec{codeErreur: 1}
	_, err := clientAvec(f).Comptes(context.Background())
	if err == nil {
		t.Fatal("Comptes aurait dû échouer")
	}
	if !strings.Contains(err.Error(), "échec simulé") {
		t.Errorf("erreur = %q, veut le message d'himalaya", err)
	}
}

func TestCompteParDéfautPremierCompteSansDéfaut(t *testing.T) {
	comptes := []Compte{{Nom: "gmail"}, {Nom: "proton"}}
	nom, ok := CompteParDéfaut(comptes)
	if !ok || nom != "gmail" {
		t.Errorf("CompteParDéfaut = %q, %v ; veut le premier compte en repli", nom, ok)
	}
}

// l'exécuteur par défaut est bien celui du système, en capturant stderr pour
// que le TUI puisse l'afficher.

func TestExécuteurParDéfaut(t *testing.T) {
	c := &CLI{Bin: "/bin/echo"}
	lancer := c.executeur()
	sortie, err := lancer(context.Background(), "echo", "bonjour")
	if err != nil {
		t.Fatalf("run : %v", err)
	}
	if strings.TrimSpace(string(sortie)) != "bonjour" {
		t.Errorf("sortie = %q", sortie)
	}
	// un binaire absent doit produire une erreur qui nomme le binaire : c'est le
	// texte que le bandeau d'erreur du TUI affichera.
	var cible *exec.Error
	_, err = lancer(context.Background(), "hippo-binaire-inexistant")
	if err == nil {
		t.Fatal("un binaire inexistant doit produire une erreur")
	}
	if !errors.As(err, &cible) {
		t.Errorf("erreur = %T, veut une erreur d'exécution", err)
	}
	if !strings.Contains(err.Error(), "hippo-binaire-inexistant") {
		t.Errorf("erreur = %q, veut le nom du binaire", err)
	}
}
