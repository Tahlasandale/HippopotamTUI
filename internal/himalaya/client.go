// Package himalaya enveloppe la CLI himalaya.
//
// Le TUI ne lit jamais le fichier de configuration de himalaya, qui contient des
// mots de passe en clair. Il ne parle qu'à la ligne de commande et n'en
// consomme que la sortie JSON, ce qui rend le découplage total : un jour, la
// config könnte être gérée par un gestionnaire de secrets sans que rien ici ne
// change.
package himalaya

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Drapeau est un drapeau IMAP.
type Drapeau string

const (
	PoussièreVolee   Drapeau = "seen"
	Répondu          Drapeau = "answered"
	Marqué           Drapeau = "flagged"
	DrapeauBrouillon Drapeau = "draft"
)

// Adresse est une adresse postale avec son nom d'affichage éventuel.
type Adresse struct {
	Nom   string `json:"name"`
	Email string `json:"email"`
}

// Format rend une adresse telle qu'elle doit être affichée : le nom quand il
// existe, l'adresse seule sinon, comme le fait `mail` depuis toujours.
func (a Adresse) Format() string {
	if a.Nom == "" {
		return a.Email
	}
	return fmt.Sprintf("%s <%s>", a.Nom, a.Email)
}

// Enveloppe est l'en-tête d'un message : de quoi l'afficher sans le télécharger.
type Enveloppe struct {
	ID        string       `json:"id"`
	MessageID string       `json:"message-id"`
	Drapeaux  []Indicateur `json:"flags"`
	Sujet     string       `json:"subject"`
	De        []Adresse    `json:"from"`
	Vers      []Adresse    `json:"to"`
	Cc        []Adresse    `json:"cc"`
	Date      string       `json:"date"`
	Taille    int          `json:"size"`
	// PiecesJointes est le drapeau calculé, celui que l'interface affiche.
	// PieceJointeRaw est le champ de himalaya, qui peut rester absent quand le
	// coût d'une requête supplémentaire n'a pas été payé.
	PiecesJointes  bool  `json:"pieces-jointes"`
	PieceJointeRaw *bool `json:"has-attachment"`
}

// Indicateur est un drapeau tel que himalaya le rend : la forme brute dépend du
// serveur, le nom IANA non. C'est toujours celui-ci qu'on compare.
type Indicateur struct {
	Brut string `json:"raw"`
	IANA string `json:"iana"`
}

// A indique si l'enveloppe porte le drapeau donné.
func (e Enveloppe) A(d Drapeau) bool {
	for _, x := range e.Drapeaux {
		if x.IANA == string(d) || x.Brut == string(d) {
			return true
		}
	}
	return false
}

// Expéditeur rend l'expéditeur principal, ou une chaîne vide.
func (e Enveloppe) Expéditeur() string {
	if len(e.De) == 0 {
		return ""
	}
	return e.De[0].Format()
}

// Destinataires rend l'expéditeur et les destinataires sous forme d'adresses
// brutes, Cc compris.
func (e Enveloppe) Destinataires() []string {
	var out []string
	for _, liste := range [][]Adresse{e.De, e.Vers, e.Cc} {
		for _, a := range liste {
			if a.Email != "" {
				out = append(out, a.Email)
			}
		}
	}
	return out
}

// Boite est une boîte aux lettres.
type Boite struct {
	ID    string `json:"id"`
	Nom   string `json:"name"`
	Role  string `json:"role"`
	Total *int   `json:"total"`
	NonLu *int   `json:"unread"`
}

// Compte est un compte défini dans la configuration de himalaya.
type Compte struct {
	Nom      string   `json:"name"`
	Defaut   bool     `json:"default"`
	Backends []string `json:"backends"`
}

// Origine décrit le message auquel une rédaction se rattache.
type Origine struct {
	// Kind vaut Nouveau, Reponse ou Transfert.
	Kind Genre
	// ID est l'identifiant du message source, vide pour un nouveau message.
	ID string
}

// Genre désigne le type de rédaction.
type Genre string

const (
	Nouveau     Genre = "nouveau"
	Reponse     Genre = "reponse"
	Transfert   Genre = "transfert"
	ReponseTout Genre = "reponse-tout"
)

// Brouillon est un message dont les champs sont prêts mais pas envoyés.
type Brouillon struct {
	De    string   `json:"from"`
	To    []string `json:"to"`
	Cc    []string `json:"cc"`
	Bcc   []string `json:"bcc"`
	Sujet string   `json:"subject"`
	Corps string   `json:"body"`
}

// Envoi est la rédaction finale : un squelette, les champs que le TUI a pu
// modifier et le fichier où l'utilisateur a écrit le corps.
type Envoi struct {
	Origine Origine
	De      string
	To      []string
	Cc      []string
	Bcc     []string
	Sujet   string
	Fichier string
}

// ErreurHimalaya porte le code de sortie et le message d'erreur de l'outil.
type ErreurHimalaya struct {
	Code int
	Msg  string
	// Commande est la ligne qui a échoué, utile au journal.
	Commande string
}

func (e *ErreurHimalaya) Error() string {
	return e.Msg
}

// Client est ce dont l'interface a besoin. Le mode démo en fournit une
// implémentation qui ne touche jamais au réseau.
type Client interface {
	Comptes(ctx context.Context) ([]Compte, error)
	Boites(ctx context.Context, compte string) ([]Boite, error)
	Enveloppes(ctx context.Context, compte, boite string, page, taille int) ([]Enveloppe, error)
	Rechercher(ctx context.Context, compte, boite, requete string, page, taille int) ([]Enveloppe, error)
	Message(ctx context.Context, compte, boite, id string) ([]byte, error)
	PoserDrapeau(ctx context.Context, compte, boite string, ids []string, d Drapeau) error
	RetirerDrapeau(ctx context.Context, compte, boite string, ids []string, d Drapeau) error
	Deplacer(ctx context.Context, compte, de, vers string, ids []string) error
	Supprimer(ctx context.Context, compte, boite string, ids []string) error
	Squelette(ctx context.Context, origine Origine, compte, boite string) (Brouillon, error)
	Enregistrer(ctx context.Context, env Envoi, compte, boite string) error
	Envoyer(ctx context.Context, env Envoi, compte, boite string) error
}

// Compile vérifie que CLI satisfait l'interface dont l'interface a besoin.
var _ Client = (*CLI)(nil)

// execer exécute une commande et renvoie sa sortie standard. Injecté pour
// pouvoir tester sans himalaya installé.
type execer func(ctx context.Context, nom string, args ...string) ([]byte, error)

// CLI est le client speakant la ligne de commande.
type CLI struct {
	// Bin est le chemin de l'exécutable himalaya.
	Bin string
	// ConfigFile, non vide, est passé en -c.
	ConfigFile string
	// Délai borne chaque appel réseau : une connexion IMAP qui traîne ne doit
	// pas figer le TUI.
	Délai time.Duration

	run execer
}

// runParDéfaut exécute la commande en capturant stderr, qui contient les seuls
// messages d'erreur exploitables.
func runParDéfaut(ctx context.Context, nom string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, nom, args...)
	var sortie, erreurs bytes.Buffer
	cmd.Stdout = &sortie
	cmd.Stderr = &erreurs

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(erreurs.String())
		if message == "" {
			message = strings.TrimSpace(sortie.String())
		}
		if message == "" {
			return nil, fmt.Errorf("%s : %w", nom, err)
		}
		code := 1
		var cible *exec.ExitError
		if errors.As(err, &cible) {
			code = cible.ExitCode()
		}
		return sortie.Bytes(), &ErreurHimalaya{Code: code, Msg: message, Commande: nom + " " + strings.Join(args, " ")}
	}
	return sortie.Bytes(), nil
}

// executeur renvoie l'exécuteur injecté, ou celui du système.
func (c *CLI) executeur() execer {
	if c.run != nil {
		return c.run
	}
	return runParDéfaut
}

// délaiImplicite borne un appel quand aucun délai n'a été fixé.
const délaiImplicite = 60 * time.Second

func (c *CLI) context(ctx context.Context) (context.Context, context.CancelFunc) {
	d := c.Délai
	if d <= 0 {
		d = délaiImplicite
	}
	return context.WithTimeout(ctx, d)
}

func (c *CLI) binaire() string {
	if c.Bin == "" {
		return "himalaya"
	}
	return c.Bin
}

// brut lance une commande sans interpréter la sortie.
func (c *CLI) brut(ctx context.Context, args ...string) ([]byte, error) {
	if c.ConfigFile != "" {
		args = append(args, "-c", c.ConfigFile)
	}
	ctx, cancel := c.context(ctx)
	defer cancel()
	return c.executeur()(ctx, c.binaire(), args...)
}

// json lance une commande et décode sa sortie.
func (c *CLI) json(ctx context.Context, cible any, args ...string) error {
	args = append(args, "--json")
	sortie, err := c.brut(ctx, args...)
	if err != nil {
		return err
	}
	if cible == nil {
		return nil
	}
	if err := json.Unmarshal(sortie, cible); err != nil {
		return fmt.Errorf("décodage de la sortie de « %s %s » : %w",
			c.binaire(), strings.Join(args, " "), err)
	}
	return nil
}

// comum communique le compte et la boîte aux commandes qui les acceptent. Ces
// options suivent le sous-commande, comme le veut l'usage de himalaya.
func comum(compte, boite string) []string {
	var args []string
	if compte != "" {
		args = append(args, "-a", compte)
	}
	if boite != "" {
		args = append(args, "-m", boite)
	}
	return args
}

// Comptes liste les comptes de la configuration de himalaya.
func (c *CLI) Comptes(ctx context.Context) ([]Compte, error) {
	var sortie struct {
		Comptes []Compte `json:"accounts"`
	}
	if err := c.json(ctx, &sortie, "account", "list"); err != nil {
		return nil, err
	}
	return sortie.Comptes, nil
}

// CompteParDéfaut renvoie le compte déclaré par défaut. À défaut, le premier
// déclaré, ce qui est l'ordre du fichier de configuration.
func CompteParDéfaut(comptes []Compte) (string, bool) {
	for _, cpt := range comptes {
		if cpt.Defaut {
			return cpt.Nom, true
		}
	}
	if len(comptes) > 0 {
		return comptes[0].Nom, true
	}
	return "", false
}

// Boites liste les boîtes d'un compte.
func (c *CLI) Boites(ctx context.Context, compte string) ([]Boite, error) {
	var sortie struct {
		Boites []Boite `json:"mailboxes"`
	}
	if err := c.json(ctx, &sortie, append([]string{"mailbox", "list"}, comum(compte, "")...)...); err != nil {
		return nil, err
	}
	return sortie.Boites, nil
}

// Normaliser reporte l'indicateur de pièce jointe sur le champ que l'interface
// utilise, sans écraser la valeur déjà connue d'une enveloppe relue du cache.
func Normaliser(enveloppes []Enveloppe) []Enveloppe {
	for i := range enveloppes {
		if enveloppes[i].PieceJointeRaw != nil {
			enveloppes[i].PiecesJointes = *enveloppes[i].PieceJointeRaw
		}
	}
	return enveloppes
}

// Enveloppes liste une page d'enveloppes, les plus récentes d'abord.
func (c *CLI) Enveloppes(ctx context.Context, compte, boite string, page, taille int) ([]Enveloppe, error) {
	var sortie struct {
		Enveloppes []Enveloppe `json:"envelopes"`
	}
	args := append([]string{"envelope", "list", "-p", itoa(page), "-s", itoa(taille)}, comum(compte, boite)...)
	if err := c.json(ctx, &sortie, args...); err != nil {
		return nil, err
	}
	return Normaliser(sortie.Enveloppes), nil
}

// Rechercher applique une requête à une boîte. La requête est transmise mot pour
// mot : c'est le langage de himalaya qui fait foi, le TUI n'y ajoute rien.
func (c *CLI) Rechercher(ctx context.Context, compte, boite, requete string, page, taille int) ([]Enveloppe, error) {
	var sortie struct {
		Enveloppes []Enveloppe `json:"envelopes"`
	}
	args := []string{"envelope", "search", "-p", itoa(page), "-s", itoa(taille)}
	if requete != "" {
		args = append(args, requete)
	}
	args = append(args, comum(compte, boite)...)
	if err := c.json(ctx, &sortie, args...); err != nil {
		return nil, err
	}
	return Normaliser(sortie.Enveloppes), nil
}

// Message renvoie les octets RFC 5322 du message. Le TUI les décode lui-même
// plutôt que de dépendre du schéma JSON interne de himalaya, qui est un détail
// d'implémentation de l'outil.
func (c *CLI) Message(ctx context.Context, compte, boite, id string) ([]byte, error) {
	args := append([]string{"message", "read", "--raw", id}, comum(compte, boite)...)
	return c.brut(ctx, args...)
}

// PoserDrapeau ajoute un drapeau en conservant les autres.
func (c *CLI) PoserDrapeau(ctx context.Context, compte, boite string, ids []string, d Drapeau) error {
	return c.actionSur(ctx, compte, boite, ids, "flag", "add", d)
}

// RetirerDrapeau retire un drapeau en conservant les autres.
func (c *CLI) RetirerDrapeau(ctx context.Context, compte, boite string, ids []string, d Drapeau) error {
	return c.actionSur(ctx, compte, boite, ids, "flag", "remove", d)
}

// Deplacer déplace des messages d'une boîte à une autre.
func (c *CLI) Deplacer(ctx context.Context, compte, de, vers string, ids []string) error {
	args := append([]string{"message", "move", "--to", vers}, ids...)
	args = append(args, comum(compte, de)...)
	return c.ignorerSortie(ctx, args)
}

// Supprimer efface des messages. himalaya les met d'abord à la corbeille : rien
// n'est perdu, c'est pourquoi le TUI ne demande pas confirmation.
func (c *CLI) Supprimer(ctx context.Context, compte, boite string, ids []string) error {
	args := append([]string{"message", "delete"}, ids...)
	args = append(args, comum(compte, boite)...)
	return c.ignorerSortie(ctx, args)
}

func (c *CLI) actionSur(ctx context.Context, compte, boite string, ids []string, mot ...any) error {
	args := make([]string, 0, len(mot)+4)
	for _, m := range mot {
		args = append(args, fmt.Sprint(m))
	}
	args = append(args, ids...)
	args = append(args, comum(compte, boite)...)
	return c.ignorerSortie(ctx, args)
}

// ignorerSortie lance une commande dont la sortie n'a pas d'intérêt, sans exiger
// qu'elle soit du JSON valide.
func (c *CLI) ignorerSortie(ctx context.Context, args []string) error {
	_, err := c.brut(ctx, args...)
	return err
}

// Squelette demande à himalaya de préparer une rédaction : en-têtes de
// threading, objet préfixé, corps déjà quoté, destinataires déduits.
func (c *CLI) Squelette(ctx context.Context, origine Origine, compte, boite string) (Brouillon, error) {
	var args []string
	if origine.Kind == Nouveau {
		args = []string{"message", "compose"}
	} else {
		args = []string{"message", commande(origine.Kind), origine.ID}
	}
	args = append(args, comum(compte, boite)...)

	var br Brouillon
	if err := c.json(ctx, &br, args...); err != nil {
		return Brouillon{}, err
	}
	return br, nil
}

func commande(g Genre) string {
	switch g {
	case Transfert:
		return "forward"
	case ReponseTout:
		return "reply"
	default:
		return "reply"
	}
}

//	argsRedaction construit les arguments d'écriture communs à l'enregistrement
//
// d'un brouillon et à l'envoi.
func (c *CLI) argsRedaction(env Envoi, compte, boite string) ([]string, error) {
	if len(env.To) == 0 && len(env.Cc) == 0 && len(env.Bcc) == 0 {
		return nil, ErrPasDeDestinataire
	}

	var args []string
	if env.Origine.Kind == Nouveau {
		args = []string{"message", "compose"}
	} else {
		args = []string{"message", commande(env.Origine.Kind), env.Origine.ID}
	}

	if env.De != "" {
		args = append(args, "--from", env.De)
	}
	args = append(args, addresses("--to", env.To)...)
	args = append(args, addresses("--cc", env.Cc)...)
	args = append(args, addresses("--bcc", env.Bcc)...)
	if env.Sujet != "" {
		args = append(args, "-s", env.Sujet)
	}
	if env.Fichier != "" {
		args = append(args, "--body-file", env.Fichier)
	}
	return append(args, comum(compte, boite)...), nil
}

func addresses(indicateur string, liste []string) []string {
	var out []string
	for _, a := range liste {
		if strings.TrimSpace(a) != "" {
			out = append(out, indicateur, a)
		}
	}
	return out
}

// ErrPasDeDestinataire est levée avant tout appel : himalaya échouerait, mais
// mieux vaut un message clair dans le bandeau d'erreur.
var ErrPasDeDestinataire = errors.New("aucun destinataire : le message n'a personne à qui arriver")

// Enregistrer écrit la rédaction dans la boîte de brouillons sans l'envoyer.
func (c *CLI) Enregistrer(ctx context.Context, env Envoi, compte, boite string) error {
	args, err := c.argsRedaction(env, compte, boite)
	if err != nil {
		return err
	}
	args = append(args, "--save", boiteBrouillon(boite))
	return c.ignorerSortie(ctx, args)
}

// Envoyer part le message. copie, non vide, nomme la boîte où conserver un
// exemplaire.
func (c *CLI) Envoyer(ctx context.Context, env Envoi, compte, boite string) error {
	args, err := c.argsRedaction(env, compte, boite)
	if err != nil {
		return err
	}
	args = append(args, "--send")
	return c.ignorerSortie(ctx, args)
}

func boiteBrouillon(boite string) string {
	if boite == "" {
		return "Drafts"
	}
	return boite
}

func itoa(n int) string { return fmt.Sprint(n) }
