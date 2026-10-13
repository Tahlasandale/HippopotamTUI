package ui

import (
	"fmt"
	"strings"

	"github.com/Tahlasandale/HippopotamTUI/internal/contacts"
	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	"github.com/Tahlasandale/HippopotamTUI/internal/redaction"
	"github.com/Tahlasandale/HippopotamTUI/internal/rfc822"
	"github.com/muesli/reflow/wordwrap"
)

// Champ est une ligne du formulaire de rédaction.
type Champ int

const (
	ChampDe Champ = iota
	ChampÀ
	ChampCc
	ChampSujet
	nbChamps
)

var nomsChamps = map[Champ]string{
	ChampDe:    "De",
	ChampÀ:     "À",
	ChampCc:    "Cc",
	ChampSujet: "Objet",
}

// Identité est l'adresse d'expédition d'un compte, avec le compte qui porte le
// serveur d'envoi.
//
// Les deux vont ensemble : choisir une adresse sans changer de compte reviendrait
// à faire passer un message Gmail par le SMTP de Proton, et l'envoi échouerait
// au dernier moment, après toute la rédaction.
type Identité struct {
	Compte  string
	Adresse string
}

// formulaire est l'écran de rédaction : l'identité d'expédition, les
// destinataires, l'objet, et le corps déjà quoté par himalaya puis retravaillé
// dans l'éditeur choisi.
type formulaire struct {
	origine   himalaya.Origine
	De        string
	Compte    string
	identités []Identité
	To        []string
	Cc        []string
	Sujet     string
	Corps     string

	champ    Champ
	saisie   string
	contacts []contacts.Contact
	proposés []contacts.Contact
	choisi   int
	modifié  bool
	ouvert   bool
}

// CompléterBrouillon répare ce que himalaya n'a pas su faire.
//
// Deux cas, tous deux rencontrés dans la vraie vie. D'abord l'objet : un message
// qui commence déjà par « Re : » ne doit pas produire « Re: Re : ». Ensuite le
// corps : himalaya ne sait pas lire certaines parties MIME mal formées et renvoie
// alors un squelette vide ; le message affiché, lui, a déjà été décodé par le
// TUI, autant s'en servir pour construire la citation.
func CompléterBrouillon(br himalaya.Brouillon, origine himalaya.Origine, message *rfc822.Message) himalaya.Brouillon {
	switch origine.Kind {
	case himalaya.Reponse, himalaya.ReponseTout:
		br.Sujet = redaction.SujetReponse(br.Sujet)
	case himalaya.Transfert:
		br.Sujet = redaction.SujetTransfert(br.Sujet)
	}

	if strings.TrimSpace(br.Corps) == "" && message != nil {
		switch origine.Kind {
		case himalaya.Reponse, himalaya.ReponseTout:
			br.Corps = redaction.Citer(message.Corps)
		case himalaya.Transfert:
			date := ""
			if !message.Date.IsZero() {
				date = message.Date.Format("02/01/2006 à 15:04")
			}
			br.Corps = redaction.CiterPourTransfert(message.Corps, message.De, message.Sujet, date)
		}
	}
	return br
}

// nouveauFormulaire initialise le formulaire à partir du squelette rendu par
// himalaya. La rédaction s'ouvre immédiatement dans l'éditeur : c'est le geste
// qu'on attend d'un « répondre », et l'utilisateur retrouve sesrepères dedans.
func nouveauFormulaire(br himalaya.Brouillon, origine himalaya.Origine, carnet []contacts.Contact, identités []Identité, compte string) *formulaire {
	f := &formulaire{
		origine:   origine,
		De:        br.De,
		identités: identités,
		To:        br.To,
		Cc:        br.Cc,
		Sujet:     br.Sujet,
		Corps:     br.Corps,
		contacts:  carnet,
		ouvert:    true,
	}
	f.retenirCompte(compte, br.De)

	// Le champ De est actif : c'est le premier ce qu'on veut savoir d'un
	// message, avant même ses destinataires.
	f.entrerChamp(ChampDe)
	return f
}

// retenirCompte choisit le compte d'envoi par défaut. La liste des identités
// fait foi quand elle connaît le compte ; sinon on s'appuie sur l'adresse que
// himalaya a posée dans le squelette.
func (f *formulaire) retenirCompte(compte, adresse string) {
	for _, identité := range f.identités {
		if identité.Compte == compte {
			f.Compte, f.De = identité.Compte, identité.Adresse
			return
		}
	}
	f.Compte, f.De = compte, adresse
}

// changerIdentité passe à l'identité suivante ou précédente. Sans liste
// d'identités, il ne se passe rien : il n'y a rien à choisir.
func (f *formulaire) changerIdentité(delta int) bool {
	if len(f.identités) < 2 {
		return false
	}

	actuel := 0
	for i, identité := range f.identités {
		if identité.Compte == f.Compte {
			actuel = i
			break
		}
	}

	suivant := (actuel + delta + len(f.identités)) % len(f.identités)
	f.Compte, f.De = f.identités[suivant].Compte, f.identités[suivant].Adresse
	return true
}

// estSélecteur signale les champs dont la valeur se choisit plutôt qu'elle ne
// se tape.
func (f *formulaire) estSélecteur(c Champ) bool { return c == ChampDe }

// ChampSuivant désigne le champ suivant : les trois champs d'en-tête tournent en
// boucle. Le corps ne s'y trouve pas, il s'édite dans l'éditeur et se lit ici.
func (f *formulaire) ChampSuivant() Champ { return (f.champ + 1) % nbChamps }

// ChampPrécédent ramène au champ précédent, lui aussi en boucle.
func (f *formulaire) ChampPrécédent() Champ { return (f.champ + nbChamps - 1) % nbChamps }

// buffer rend le contenu editable d'un champ sous forme de texte : c'est ce que
// la saisie modifie, et ce qui est recalculé en liste d'adresses à la sortie.
func (f *formulaire) buffer(c Champ) string {
	switch c {
	case ChampDe:
		return f.De
	case ChampÀ:
		return strings.Join(f.To, ", ")
	case ChampCc:
		return strings.Join(f.Cc, ", ")
	case ChampSujet:
		return f.Sujet
	}
	return ""
}

// entrerChamp pose le champ actif et charge son contenu dans la saisie.
func (f *formulaire) entrerChamp(c Champ) {
	f.champ = c
	f.saisie = f.buffer(c)
	f.choisi = 0
	f.actualiserPropositions()
}

// valider écrit la saisie courante dans les champs du formulaire.
func (f *formulaire) valider() {
	switch f.champ {
	case ChampDe:
		return
	case ChampÀ:
		f.To = analyserAdresses(f.saisie)
	case ChampCc:
		f.Cc = analyserAdresses(f.saisie)
	case ChampSujet:
		f.Sujet = f.saisie
	}
	f.modifié = true
}

// analyserAdresses découpe une saisie d'adresses séparées par virgules,
// points-virgules ou espaces : on peut enchaîner plusieurs destinataires à la
// clavier sans lever les doigts.
func analyserAdresses(valeur string) []string {
	var out []string
	for _, morceau := range strings.FieldsFunc(valeur, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	}) {
		morceau = strings.TrimSpace(morceau)
		if morceau != "" && strings.Contains(morceau, "@") {
			out = append(out, morceau)
		}
	}
	return out
}

// actualiserPropositions recalcule la liste d'adresses proposées sous le champ.
func (f *formulaire) actualiserPropositions() {
	if f.champ != ChampÀ && f.champ != ChampCc {
		f.proposés = nil
		f.choisi = 0
		return
	}
	f.proposés = contacts.Correspondance(f.contacts, f.saisie, 6)
	if f.choisi >= len(f.proposés) {
		f.choisi = 0
	}
}

// propositionCourante est celle qu'Entrée ou Tab valide.
func (f *formulaire) propositionCourante() (contacts.Contact, bool) {
	if f.choisi < 0 || f.choisi >= len(f.proposés) {
		return contacts.Contact{}, false
	}
	return f.proposés[f.choisi], true
}

// accepterProposition ajoute l'adresse choisie et vide la saisie en cours.
func (f *formulaire) accepterProposition() bool {
	proposition, ok := f.propositionCourante()
	if !ok {
		return false
	}

	// La saisie en cours est d'abord versée dans la liste : taper
	// « alice, almt » puis valider la proposition doit donner les deux
	// destinataires, et pas seulement le second.
	f.valider()

	switch f.champ {
	case ChampÀ:
		f.To = annexer(f.To, proposition.Email)
	case ChampCc:
		f.Cc = annexer(f.Cc, proposition.Email)
	}
	f.saisie = f.buffer(f.champ)
	f.proposés = nil
	f.choisi = 0
	f.modifié = true
	return true
}

// annexer ajoute une adresse sans jamais la dupliquer.
func annexer(liste []string, adresse string) []string {
	for _, déjà := range liste {
		if strings.EqualFold(déjà, adresse) {
			return liste
		}
	}
	return append(liste, adresse)
}

// valide dit si la rédaction peut partir.
func (f *formulaire) valide() bool {
	return len(f.To)+len(f.Cc) > 0 && strings.TrimSpace(f.Sujet) != ""
}

// vide dit si la rédaction ne contient rien : c'est le cas où l'utilisateur a tout
// vidé et où il ne faut surtout pas laisser de brouillon.
func (f *formulaire) vide() bool {
	return len(f.To) == 0 && len(f.Cc) == 0 &&
		strings.TrimSpace(f.Sujet) == "" && strings.TrimSpace(f.Corps) == ""
}

// versEnvoi construit la rédaction transmise à himalaya. himalaya reste seul
// maître des en-têtes : le TUI ne touche ni au Re: ni aux références.
func (f *formulaire) versEnvoi(fichier string) himalaya.Envoi {
	return himalaya.Envoi{
		Origine: f.origine,
		De:      f.De,
		To:      f.To,
		Cc:      f.Cc,
		Sujet:   f.Sujet,
		Fichier: fichier,
	}
}

// valeurAffichée rend le champ actif tel qu'il est : c'est la saisie en cours,
// complétée des adresses déjà validées.
// affichée rend un champ : la saisie en cours pour le champ actif, la valeur
// déjà validée pour les autres. Sans cette distinction, les trois champs
// afficheraient la même chose.
func (f *formulaire) affichée(c Champ) string {
	if c == ChampDe {
		return f.De
	}
	if f.champ == c {
		return f.saisie
	}
	return f.buffer(c)
}

func listerAdresses(liste []string, enCours string) string {
	out := strings.Join(liste, ", ")
	if enCours != "" {
		if out != "" {
			out += ", "
		}
		out += enCours
	}
	return out
}

// corpsEnveloppé rend le corps avec un repli automatique sur la largeur, pour
// qu'aucune ligne ne déborde du terminal.
func corpsEnveloppé(corps string, largeur int) []string {
	if largeur < 20 {
		largeur = 20
	}
	return strings.Split(wordwrap.String(corps, largeur), "\n")
}

// rendreFormulaire produit les lignes du formulaire prêtes à l'affichage.
func rendreFormulaire(f *formulaire, largeur int) []string {
	étiquette := func(nom, valeur string, actif bool) string {
		gauche := fmt.Sprintf("%-6s ", nom+" :")
		droite := couperColonnes(valeur, max0(largeur-8))
		if actif {
			return stSelection.Render(remplir(gauche+droite, largeur))
		}
		if valeur == "" {
			droite = stLigneCourte.Render(droite)
		}
		return stEtiquette.Render(gauche) + droite
	}

	lignes := make([]string, 0, nbChamps)
	for c := Champ(0); c < nbChamps; c++ {
		lignes = append(lignes, étiquette(nomsChamps[c], f.affichée(c), f.champ == c))
	}

	corps := corpsEnveloppé(f.Corps, largeur)
	if len(corps) == 0 || (len(corps) == 1 && corps[0] == "") {
		corps = []string{stLigneCourte.Render("(vide — ctrl+r pour l'écrire dans l'éditeur)")}
	}
	lignes = append(lignes, "")
	lignes = append(lignes, corps...)
	return lignes
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
