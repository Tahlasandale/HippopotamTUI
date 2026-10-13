package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/config"
	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	"github.com/Tahlasandale/HippopotamTUI/internal/rfc822"
	tea "github.com/charmbracelet/bubbletea"
)

// Les messages que le modèle reçoit de lui-même : une commande asynchrone
// renvoie toujours l'un de ceux-là, et jamais un []byte qu'il faudrait
// distinguer d'un autre []byte.

type (
	msgComptes struct {
		comptes   []himalaya.Compte
		identités []Identité
		err       error
	}
	msgBoîtes struct {
		boîtes []himalaya.Boite
		err    error
	}
	msgEnveloppes struct {
		enveloppes []himalaya.Enveloppe
		err        error
	}
	msgCorps struct {
		message *rfc822.Message
		err     error
	}
	msgBrouillon struct {
		brouillon himalaya.Brouillon
		origine   himalaya.Origine
		err       error
	}
	msgÉditée struct {
		texte string
		err   error
	}
	msgEnvoyé              struct{ err error }
	msgBrouillonEnregistré struct {
		err error
	}
	msgAction struct {
		quoi       string
		info       string
		err        error
		rafraîchir bool
	}
	msgCarnet struct{ n int }
)

// ------------------------------------------------------------------ commandes

func (m *Modèle) chargerComptes() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		comptes, err := m.client.Comptes(ctx)
		if err != nil {
			return msgComptes{err: err}
		}
		return msgComptes{comptes: comptes, identités: identitéDes(ctx, m.client, comptes)}
	}
}

// identitéDes apprend l'adresse d'expédition de chaque compte.
//
// On demande à himalaya un squelette vide : c'est un calcul local, sans réseau,
// et c'est la seule façon de connaître son adresse sans lire sa configuration —
// qui contient les mots de passe. Un compte qui répond mal est simplement écarté
// : on n'en tient pas au champ « De » pour le reste.
func identitéDes(ctx context.Context, client himalaya.Client, comptes []himalaya.Compte) []Identité {
	var identités []Identité
	for _, cpt := range comptes {
		br, err := client.Squelette(ctx, himalaya.Origine{Kind: himalaya.Nouveau}, cpt.Nom, "")
		if err != nil || br.De == "" {
			continue
		}
		identités = append(identités, Identité{Compte: cpt.Nom, Adresse: br.De})
	}
	return identités
}

func (m *Modèle) chargerBoîtes() tea.Cmd {
	return func() tea.Msg {
		boîtes, err := m.client.Boites(context.Background(), m.compte)
		return msgBoîtes{boîtes: boîtes, err: err}
	}
}

// chargerEnveloppes lit une page. Si le cache a quelque chose de récent et que
// rien ne force le réseau, il rend la main tout de suite : c'est ce qui rend la
// navigation instantanée.
func (m *Modèle) chargerEnveloppes(forcer bool) tea.Cmd {
	compte, boîte, page, taille := m.compte, m.boîte, m.page, m.cfg.PageSize

	if !forcer && m.cache != nil {
		if env, quand, ok := m.cache.Enveloppes(compte, boîte, page); ok &&
			time.Since(quand) < délaiRefreshing(m.cfg) {
			return func() tea.Msg { return msgEnveloppes{enveloppes: env} }
		}
	}

	m.occupé = true
	return func() tea.Msg {
		var (
			env []himalaya.Enveloppe
			err error
		)
		if requête := m.filtres.Requête(); requête != "" {
			env, err = m.client.Rechercher(context.Background(), compte, boîte, requête, page, taille)
		} else {
			env, err = m.client.Enveloppes(context.Background(), compte, boîte, page, taille)
		}
		if err == nil && m.cache != nil {
			m.cache.EnregistrerEnveloppes(compte, boîte, page, env, time.Now())
		}
		return msgEnveloppes{enveloppes: env, err: err}
	}
}

// ouvrirMessage télécharge le message et marque la ligne comme lue.
func (m *Modèle) ouvrirMessage() tea.Cmd {
	env := m.envelopeCourante()
	if env == nil {
		return nil
	}
	compte, boîte, id := m.compte, m.boîte, env.ID
	return func() tea.Msg {
		brut, err := m.client.Message(context.Background(), compte, boîte, id)
		if err != nil {
			return msgCorps{err: err}
		}
		message, err := rfc822.Analyser(brut)
		return msgCorps{message: message}
	}
}

// marquerLu passe le message courant en « vu », sans quitter l'écran de lecture.
func (m *Modèle) marquerLu() tea.Cmd {
	env := m.envelopeCourante()
	if env == nil || env.A(himalaya.PoussièreVolee) {
		return nil
	}
	return m.basculerDrapeau(himalaya.PoussièreVolee, true)
}

func (m *Modèle) basculerDrapeau(d himalaya.Drapeau, poser bool) tea.Cmd {
	env := m.envelopeCourante()
	if env == nil {
		return nil
	}
	// L'état local change tout de suite : l'utilisateur voit la ligne réagir à
	// la frappe, et l'appel réseau ne sert qu'à propager.
	m.appliquerDrapeau(env, d, poser)

	compte, boîte, id := m.compte, m.boîte, env.ID
	quotidien := len(m.enveloppes) == 1
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		if poser {
			err = m.client.PoserDrapeau(ctx, compte, boîte, []string{id}, d)
		} else {
			err = m.client.RetirerDrapeau(ctx, compte, boîte, []string{id}, d)
		}
		if err != nil {
			return msgAction{quoi: "changement de drapeau", err: err, rafraîchir: true}
		}
		if quotidien {
			// Marquer comme lu en ouvrant un message ne doit pas refaire un
			// aller-retour complet sur la boîte : l'état local a déjà bougé.
			return msgAction{quoi: "changement de drapeau"}
		}
		return msgAction{quoi: "changement de drapeau", info: drapeauAnnoncé(d, poser), rafraîchir: true}
	}
}

func drapeauAnnoncé(d himalaya.Drapeau, poser bool) string {
	switch d {
	case himalaya.PoussièreVolee:
		if poser {
			return "Marqué comme lu."
		}
		return "Marqué comme non lu."
	case himalaya.Marqué:
		if poser {
			return "Message marqué."
		}
		return "Marquage retiré."
	}
	return ""
}

func (m *Modèle) supprimerMessage() tea.Cmd {
	env := m.envelopeCourante()
	if env == nil {
		return nil
	}
	compte, boîte, id := m.compte, m.boîte, env.ID
	m.retirerEnveloppe(id)
	return func() tea.Msg {
		if err := m.client.Supprimer(context.Background(), compte, boîte, []string{id}); err != nil {
			return msgAction{quoi: "Suppression", err: err}
		}
		return msgAction{quoi: "Suppression", info: "Message mis à la corbeille."}
	}
}

func (m *Modèle) déplacerMessage(vers string) tea.Cmd {
	env := m.envelopeCourante()
	if env == nil {
		return nil
	}
	compte, de, id := m.compte, m.boîte, env.ID
	m.retirerEnveloppe(id)
	return func() tea.Msg {
		if err := m.client.Deplacer(context.Background(), compte, de, vers, []string{id}); err != nil {
			return msgAction{quoi: "Déplacement", err: err}
		}
		return msgAction{quoi: "Déplacement", info: fmt.Sprintf("Déplacé vers %s.", vers)}
	}
}

// préparerBrouillon demande à himalaya de poser le squelette : en-têtes de
// threading, objet préfixé, corps quoté, destinataires.
func (m *Modèle) préparerBrouillon(origine himalaya.Origine) tea.Cmd {
	compte, boîte := m.compte, m.boîte
	if origine.Kind == himalaya.Nouveau {
		boîte = ""
	}

	// himalaya met une à deux secondes à répondre. Sans rien afficher, l'écran
	// semble figé, et on appuie sur une touche au hasard.
	m.occupé = true
	m.attente = "préparation"
	return func() tea.Msg {
		br, err := m.client.Squelette(context.Background(), origine, compte, boîte)
		return msgBrouillon{brouillon: br, origine: origine, err: err}
	}
}

// lancerÉditeur ouvre l'éditeur choisi sur un fichier temporaire et rend son
// contenu. C'est la seule écriture disque du programme hors cache : l'éditeur
// fait le travail qu'il fait bien, avec ses raccourcis, ses couleurs et son
// gestionnaire de fichiers.
func (m *Modèle) lancerÉditeur() tea.Cmd {
	if m.formulaire == nil {
		return nil
	}
	éditeur := m.cfg.Editor
	if éditeur == "" {
		éditeur = détecterÉditeur(os.Getenv("EDITOR"))
	}

	contenu := m.formulaire.Corps
	brouillon := m.formulaire.versEnvoi("")

	return func() tea.Msg {
		répertoire, err := os.MkdirTemp("", "hippo-")
		if err != nil {
			return msgÉditée{err: err}
		}
		defer os.RemoveAll(répertoire)

		fichier := filepath.Join(répertoire, "message.eml")
		if err := os.WriteFile(fichier, []byte(contenuDeLEditeur(brouillon, contenu)), 0o600); err != nil {
			return msgÉditée{err: err}
		}

		commande := strings.Fields(éditeur)
		if len(commande) == 0 {
			return msgÉditée{err: fmt.Errorf("aucun éditeur trouvé : definez editor dans la configuration ou $EDITOR")}
		}
		commande = append(commande, fichier)

		cmd := exec.Command(commande[0], commande[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return msgÉditée{err: fmt.Errorf("%s : %w", commande[0], err)}
		}

		lu, err := os.ReadFile(fichier)
		if err != nil {
			return msgÉditée{err: err}
		}
		return msgÉditée{texte: contenuRelu(string(lu))}
	}
}

// envoyer écrit le corps dans un fichier temporaire puis confie l'envoi à
// himalaya, qui réécrit le Re: et les en-têtes de threading.
func (m *Modèle) envoyer() tea.Cmd {
	if m.formulaire == nil {
		return nil
	}
	env := m.formulaire.versEnvoi("")
	compte, boîte := m.compte, m.boîte
	if env.Origine.Kind == himalaya.Nouveau {
		boîte = ""
	}

	return func() tea.Msg {
		répertoire, err := os.MkdirTemp("", "hippo-")
		if err != nil {
			return msgEnvoyé{err: err}
		}
		defer os.RemoveAll(répertoire)

		fichier := filepath.Join(répertoire, "corps.txt")
		if err := os.WriteFile(fichier, []byte(m.formulaire.Corps), 0o600); err != nil {
			return msgEnvoyé{err: err}
		}
		env.Fichier = fichier

		return msgEnvoyé{err: m.client.Envoyer(context.Background(), env, compte, boîte)}
	}
}

// enregistrerBrouillon range la rédaction en brouillon : c'est le filet quand on
// quitte la rédaction sans envoyer.
func (m *Modèle) enregistrerBrouillon() tea.Cmd {
	if m.formulaire == nil {
		return nil
	}
	env := m.formulaire.versEnvoi("")
	compte, boîte := m.compteDEnvoi(), m.boîte
	if env.Origine.Kind == himalaya.Nouveau {
		boîte = ""
	}

	return func() tea.Msg {
		répertoire, err := os.MkdirTemp("", "hippo-")
		if err != nil {
			return msgBrouillonEnregistré{err: err}
		}
		defer os.RemoveAll(répertoire)

		fichier := filepath.Join(répertoire, "corps.txt")
		if err := os.WriteFile(fichier, []byte(m.formulaire.Corps), 0o600); err != nil {
			return msgBrouillonEnregistré{err: err}
		}
		env.Fichier = fichier

		return msgBrouillonEnregistré{err: m.client.Enregistrer(context.Background(), env, compte, boîte)}
	}
}

// moissonnerCarnet reconstruit le carnet des contacts en tâche de fond, une
// seule fois par semaine et par compte. L'utilisateur ne le voit pas : c'est ce
// qui rend l'autocomplétion disponible le jour où il en a besoin.
func (m *Modèle) moissonnerCarnet() tea.Cmd {
	if m.carnet == nil || m.démo {
		return nil
	}
	m.carnet.ParPage = 500
	m.carnet.Maximum = 2000
	compte := m.compte
	jours := m.cfg.ContactsRefreshDays

	return func() tea.Msg {
		if !m.carnet.ÀRafraîchir(compte, jours) {
			return msgCarnet{n: len(m.carnet.Lire(compte))}
		}
		m.progression = "moisson des contacts"
		trouvés, err := m.carnet.Récolter(context.Background(), compte, m.adressesPropres(), nil)
		if err != nil {
			// Le carnet est un confort, pas une fonction : son échec ne doit
			// jamais se voir dans l'interface.
			return msgCarnet{n: len(m.carnet.Lire(compte))}
		}
		return msgCarnet{n: len(trouvés)}
	}
}

// rechargerCarnet met à jour la liste de contacts en mémoire après un moisson.
func (m *Modèle) rechargerCarnet() {
	if m.carnet == nil {
		return
	}
	m.contacts = m.carnet.Lire(m.compte)
	if m.formulaire != nil {
		m.formulaire.contacts = m.contacts
	}
}

// adressesPropres liste les adresses qui nous appartiennent, pour ne pas se
// proposer à soi-même.
func (m *Modèle) adressesPropres() []string {
	var out []string
	for _, c := range m.comptes {
		out = append(out, c.Nom)
	}
	if m.formulaire != nil && m.formulaire.De != "" {
		out = append(out, m.formulaire.De)
	}
	return out
}

// ------------------------------------------------------------------ aide-mémoire

func détecterÉditeur(valeur string) string {
	if valeur != "" {
		return valeur
	}
	for _, candidat := range []string{"micro", "nvim", "vim", "vi", "nano"} {
		if _, err := exec.LookPath(candidat); err == nil {
			return candidat
		}
	}
	return ""
}

// contenuDeLEditeur écrit le fichier que l'éditeur ouvre.
//
// Il ne contient **que le corps**. Les en-têtes ne sont plus écrits dedans, et
// c'est volontaire : ils se règlent dans le formulaire, où l'autocomplétion des
// adresses existe. Les faire apparaître dans l'éditeur donnait l'impression
// qu'on pouvait les y changer, et ils étaient sans doute retirés en silence au
// retour — le pire des deux mondes.
func contenuDeLEditeur(env himalaya.Envoi, corps string) string {
	if corps == "" {
		return ""
	}
	return corps
}

// contenuRelu prend le fichier de l'éditeur tel quel, aux fins de ligne près.
// Rien n'est retiré : ce qui est écrit dans l'éditeur est ce qui partira.
func contenuRelu(source string) string {
	return strings.TrimSpace(normaliserLignes(source))
}

func normaliserLignes(source string) string {
	return strings.ReplaceAll(strings.ReplaceAll(source, "\r\n", "\n"), "\r", "\n")
}

// marquerLuSansRafraîchir met à jour la ligne courante sans repasser par le
// réseau : ouvrir un message ne doit pas attendre un aller-retour IMAP.
func (m *Modèle) marquerLuSansRafraîchir() {
	env := m.envelopeCourante()
	if env == nil || env.A(himalaya.PoussièreVolee) {
		return
	}
	m.appliquerDrapeau(env, himalaya.PoussièreVolee, true)
}

// appliquerDrapeau change l'état local d'un drapeau. Le modèle reste la vérité
// de ce qui est affiché ; le serveur rattrapera ensuite.
func (m *Modèle) appliquerDrapeau(env *himalaya.Enveloppe, d himalaya.Drapeau, poser bool) {
	kept := env.Drapeaux[:0]
	for _, x := range env.Drapeaux {
		if x.IANA != string(d) {
			kept = append(kept, x)
		}
	}
	if poser {
		kept = append(kept, himalaya.Indicateur{IANA: string(d)})
	}
	env.Drapeaux = kept
}

// retirerEnveloppe retire une ligne de la liste affichée.
func (m *Modèle) retirerEnveloppe(id string) {
	for i, env := range m.enveloppes {
		if env.ID != id {
			continue
		}
		m.enveloppes = append(m.enveloppes[:i], m.enveloppes[i+1:]...)
		m.censureCurseur()
		return
	}
}

// changerBoîte sélectionne une autre boîte et recharge la liste.
func (m *Modèle) changerBoîte(id string) tea.Cmd {
	m.boîte = id
	m.page = 1
	m.curseur = 0
	m.décalage = 0
	return m.chargerEnveloppes(false)
}

// changerCompte repart sur l'autre compte, boîte de réception comprise.
func (m *Modèle) changerCompte(nom string) tea.Cmd {
	m.compte = nom
	m.boîte = ""
	m.page = 1
	m.curseur = 0
	m.décalage = 0
	return tea.Batch(m.chargerBoîtes(), m.moissonnerCarnet())
}

func délaiRefreshing(cfg config.Config) time.Duration {
	if cfg.RefreshMinutes <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(cfg.RefreshMinutes) * time.Minute
}
