// Package demo fournit un client himalaya entièrement hors ligne.
//
// Deux usages : contributor sans compte mail, et capture d'écran reproductible
// pour le README. Les données sont fabriquées à la volée, aucune donnée
// personnelle n'est embarquée.
package demo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
)

// Client est un himalaya.Client qui répond de mémoire.
type Client struct {
	env      []himalaya.Enveloppe
	parBoite map[string][]himalaya.Enveloppe
	effacés  map[string]bool
	drapeaux map[string]map[himalaya.Drapeau]bool
	envoyés  []himalaya.Envoi
	reçues   []string
}

// Nouveau fabrique un jeu de messages plausible.
func Nouveau() *Client {
	now := time.Now()
	journal := func(jours, heures int) string {
		return now.AddDate(0, 0, -jours).Add(-time.Duration(heures) * time.Hour).Format(time.RFC3339)
	}

	modèles := []struct {
		id, sujet, expéditeur, corps string
		jours, heures                int
		nonLu, pièce                 bool
	}{
		{"101", "Rapport de la commission scolaire", "Commission Scolaire <scolaire@example.org>",
			"Bonjour,\n\nVous trouverez ci-joint le rapport de la commission scolaire du trimestre.\n\nCordialement,", 0, 2, true, true},
		{"102", "Réunion du conseil municipal", "Mairie <mairie@example.org>",
			"Madame, Monsieur,\n\nLa réunion du conseil municipal se tiendra le jeudi à 19 h.\nL'ordre du jour est en pièce jointe.\n\nCordialement,", 0, 6, true, false},
		{"103", "Votre attestation d'inscription", "Scolarité <scolarite@example.org>",
			"Bonjour,\n\nVotre attestation est disponible dans votre espace.\n\nCordialement,", 1, 3, false, false},
		{"104", "Candidature reçue", "Emploi <emploi@example.org>",
			"Bonjour,\n\nNous accusons réception de votre candidature.\n\nCordialement,", 2, 1, false, false},
		{"105", "Facture du mois", "Énergie <energie@example.org>",
			"Bonjour,\n\nVotre facture est disponible.\n\nCordialement,", 3, 5, false, true},
		{"106", "Invitation à l'assemblée générale", "Association <contact@example.org>",
			"Madame, Monsieur,\n\nVous êtes convié à l'assemblée générale annuelle.\n\nCordialement,", 4, 2, false, false},
		{"107", "Newsletter du service", "Info <info@example.org>",
			"Bonjour,\n\nTrois choses à savoir ce mois-ci : les horaires, les travaux, les inscriptions.\n\nCordialement,", 6, 4, false, false},
		{"108", "Réponse à votre message", "Carla <carla@example.org>",
			"Bonjour,\n\nMerci pour votre message, je vous réponds dès que possible.\n\nÀ bientôt,", 7, 1, false, false},
		{"109", "Mot de passe expiré", "Sécurité <securite@example.org>",
			"Bonjour,\n\nVotre mot de passe expire dans trois jours.\n\nCordialement,", 9, 3, false, false},
		{"110", "Photo de la sortie", "Groupe <groupe@example.org>",
			"Bonjour,\n\nQuelques photos de la sortie de fin d'année.\n\nCordialement,", 12, 2, false, true},
	}

	c := &Client{
		parBoite: map[string][]himalaya.Enveloppe{},
		effacés:  map[string]bool{},
		drapeaux: map[string]map[himalaya.Drapeau]bool{},
	}

	for _, m := range modèles {
		env := himalaya.Enveloppe{
			ID:        m.id,
			MessageID: "<" + m.id + "@example.org>",
			Sujet:     m.sujet,
			De:        []himalaya.Adresse{adresse(m.expéditeur)},
			Vers:      []himalaya.Adresse{adresse("Orphin <orphin@protonmail.com>")},
			Date:      journal(m.jours, m.heures),
			Taille:    1024 + len(m.corps),
		}
		if !m.nonLu {
			env.Drapeaux = append(env.Drapeaux, drapeau(`\Seen`, "seen"))
		}
		if m.id == "104" {
			env.Drapeaux = append(env.Drapeaux, drapeau(`\Flagged`, "flagged"))
		}
		if m.pièce {
			vrai := true
			env.PieceJointeRaw = &vrai
			env.PiecesJointes = true
		}
		c.env = append(c.env, env)
		c.parBoite["Inbox"] = append(c.parBoite["Inbox"], env)
	}

	envoyés := []himalaya.Enveloppe{
		{
			ID:    "201",
			Sujet: "Re: Réunion du conseil municipal",
			De:    []himalaya.Adresse{adresse("Orphin <orphin@protonmail.com>")},
			Vers:  []himalaya.Adresse{adresse("Mairie <mairie@example.org>")},
			Date:  journal(0, 8),
		},
		{
			ID:    "202",
			Sujet: "Candidature",
			De:    []himalaya.Adresse{adresse("Orphin <orphin@protonmail.com>")},
			Vers:  []himalaya.Adresse{adresse("Emploi <emploi@example.org>")},
			Date:  journal(5, 4),
		},
	}
	c.parBoite["Sent"] = envoyés

	c.parBoite["Archive"] = []himalaya.Enveloppe{
		{ID: "301", Sujet: "Ancien message", De: []himalaya.Adresse{adresse("Vieux <vieux@example.org>")},
			Date: journal(40, 0), Drapeaux: []himalaya.Indicateur{drapeau(`\Seen`, "seen")}},
	}
	c.parBoite["Drafts"] = nil
	c.parBoite["Trash"] = nil

	return c
}

// drapeau construit l'indicateur de drapeau tel que himalaya le rend.
func drapeau(brut, iana string) himalaya.Indicateur {
	return himalaya.Indicateur{Brut: brut, IANA: iana}
}

func adresse(valeur string) himalaya.Adresse {
	nom, email := "", valeur
	if début, fin := strings.Index(valeur, "<"), strings.Index(valeur, ">"); début >= 0 && fin > début {
		nom = strings.TrimSpace(valeur[:début])
		email = valeur[début+1 : fin]
	}
	return himalaya.Adresse{Nom: nom, Email: email}
}

var comptes = []himalaya.Compte{
	{Nom: "proton", Defaut: true, Backends: []string{"imap", "smtp"}},
	{Nom: "gmail", Backends: []string{"imap", "smtp"}},
}

var boîtes = []himalaya.Boite{
	{ID: "Inbox", Nom: "Inbox", Role: "inbox"},
	{ID: "Sent", Nom: "Sent"},
	{ID: "Archive", Nom: "Archive"},
	{ID: "Drafts", Nom: "Drafts"},
	{ID: "Trash", Nom: "Trash"},
}

// Comptes liste les comptes fictifs.
func (c *Client) Comptes(context.Context) ([]himalaya.Compte, error) { return comptes, nil }

// Boites liste les boîtes fictives, avec un compte de non-lus plausible.
func (c *Client) Boites(_ context.Context, _ string) ([]himalaya.Boite, error) {
	petites := len(c.parBoite["Inbox"])
	out := make([]himalaya.Boite, 0, len(boîtes))
	for _, b := range boîtes {
		copie := b
		total := len(c.parBoite[b.ID])
		nonLu := 0
		for _, env := range c.parBoite[b.ID] {
			if !env.A(himalaya.PoussièreVolee) {
				nonLu++
			}
		}
		copie.Total = &total
		if b.Role == "inbox" {
			nonLu = petites - 3
		}
		copie.NonLu = &nonLu
		out = append(out, copie)
	}
	return out, nil
}

// Enveloppes rend une page du jeu de démonstration.
func (c *Client) Enveloppes(_ context.Context, _, boite string, page, taille int) ([]himalaya.Enveloppe, error) {
	return paginer(c.parBoite[boite], page, taille), nil
}

// Rechercher applique la seule chose que le mode démo sache faire : ne garder
// que les non lus quand on le demande.
func (c *Client) Rechercher(_ context.Context, _, boite, requete string, page, taille int) ([]himalaya.Enveloppe, error) {
	liste := c.parBoite[boite]
	if strings.Contains(requete, "not flag seen") {
		var filtrés []himalaya.Enveloppe
		for _, env := range liste {
			if !env.A(himalaya.PoussièreVolee) {
				filtrés = append(filtrés, env)
			}
		}
		liste = filtrés
	}
	return paginer(liste, page, taille), nil
}

func paginer(liste []himalaya.Enveloppe, page, taille int) []himalaya.Enveloppe {
	début := (page - 1) * taille
	if début >= len(liste) {
		return nil
	}
	fin := début + taille
	if fin > len(liste) {
		fin = len(liste)
	}
	copie := append([]himalaya.Enveloppe(nil), liste[début:fin]...)
	return himalaya.Normaliser(copie)
}

// Message fabrique un message complet à partir du jeu de démonstration.
func (c *Client) Message(_ context.Context, _, boite, id string) ([]byte, error) {
	var env himalaya.Enveloppe
	for _, candidate := range c.parBoite[boite] {
		if candidate.ID == id {
			env = candidate
		}
	}
	if env.ID == "" {
		return nil, fmt.Errorf("aucun message %s dans la démonstration", id)
	}

	entêtes := fmt.Sprintf("Message-Id: <%s@example.org>\r\nDate: %s\r\nFrom: %s\r\nTo: %s\r\nSubject: %s\r\n",
		id, env.Date, env.Expéditeur(), env.Vers[0].Format(), env.Sujet)

	var corps strings.Builder
	corps.WriteString("Bonjour,\r\n\r\n")
	corps.WriteString(fmt.Sprintf("Ceci est le message de démonstration numéro %s, dans la boîte %s.\r\n\r\n", id, boite))
	corps.WriteString("Il permet d'avoir une liste pleine, un message à lire, un fil de\r\n")
	corps.WriteString("discussion à reprendre, le tout sans toucher au réseau.\r\n\r\n")
	corps.WriteString("Cordialement,\r\n")
	corps.WriteString(env.Expéditeur())

	parties := entêtes + "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + corps.String()
	if env.PiecesJointes {
		avecPièces := entêtes +
			"MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"demo\"\r\n\r\n" +
			"--demo\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + corps.String() + "\r\n" +
			"--demo\r\nContent-Type: application/pdf; name=\"rapport.pdf\"\r\n" +
			"Content-Transfer-Encoding: base64\r\n" +
			"Content-Disposition: attachment; filename=\"rapport.pdf\"\r\n\r\n" +
			"JVBERi0xLjQKJUZBS1BPUg==\r\n--demo--\r\n"
		parties = avecPièces
	}
	return []byte(parties), nil
}

// Squelette fabrique un brouillon cohérent avec le jeu de démonstration.
func (c *Client) Squelette(_ context.Context, origine himalaya.Origine, _, boite string) (himalaya.Brouillon, error) {
	moi := "Orphin <orphin@protonmail.com>"

	if origine.Kind == himalaya.Nouveau {
		return himalaya.Brouillon{De: moi}, nil
	}

	for _, env := range c.parBoite[boite] {
		if env.ID != origine.ID {
			continue
		}
		br := himalaya.Brouillon{
			De:    moi,
			Sujet: "Re: " + env.Sujet,
			To:    []string{env.Expéditeur()},
			Cc:    []string{},
			Corps: "> " + env.Sujet + "\r\n",
		}
		if origine.Kind == himalaya.Transfert {
			br.Sujet = "Fwd: " + env.Sujet
			br.To = nil
			br.Corps = "-------- Message transféré --------\r\n" + env.Sujet + "\r\n"
		}
		return br, nil
	}
	return himalaya.Brouillon{}, fmt.Errorf("aucun message %s dans la démonstration", origine.ID)
}

// PoserDrapeau et RetirerDrapeau modifient le jeu en mémoire.
func (c *Client) PoserDrapeau(_ context.Context, _, boite string, ids []string, d himalaya.Drapeau) error {
	return c.ajouterDrapeau(boite, ids, d, true)
}

func (c *Client) RetirerDrapeau(_ context.Context, _, boite string, ids []string, d himalaya.Drapeau) error {
	return c.ajouterDrapeau(boite, ids, d, false)
}

func (c *Client) ajouterDrapeau(boite string, ids []string, d himalaya.Drapeau, présent bool) error {
	for i, env := range c.parBoite[boite] {
		if !contient(ids, env.ID) {
			continue
		}
		var kept []himalaya.Indicateur
		for _, x := range env.Drapeaux {
			if x.IANA != string(d) {
				kept = append(kept, x)
			}
		}
		if présent {
			kept = append(kept, drapeau(string(d), string(d)))
		}
		c.parBoite[boite][i].Drapeaux = kept
	}
	return nil
}

func contient(liste []string, valeur string) bool {
	for _, item := range liste {
		if item == valeur {
			return true
		}
	}
	return false
}

// Deplacer déplace un message d'une boîte à l'autre dans le jeu.
func (c *Client) Deplacer(_ context.Context, _, de, vers string, ids []string) error {
	for _, id := range ids {
		for i, env := range c.parBoite[de] {
			if env.ID == id {
				c.parBoite[vers] = append(c.parBoite[vers], env)
				c.parBoite[de] = append(c.parBoite[de][:i], c.parBoite[de][i+1:]...)
				break
			}
		}
	}
	return nil
}

// Supprimer efface un message du jeu.
func (c *Client) Supprimer(_ context.Context, _, boite string, ids []string) error {
	for _, id := range ids {
		c.effacés[id] = true
		for i, env := range c.parBoite[boite] {
			if env.ID == id {
				c.parBoite[boite] = append(c.parBoite[boite][:i], c.parBoite[boite][i+1:]...)
				break
			}
		}
	}
	return nil
}

// Enregistrer note le brouillon sans rien écrire : la démonstration n'écrit nulle
// part.
func (c *Client) Enregistrer(_ context.Context, env himalaya.Envoi, _, _ string) error {
	c.reçues = append(c.reçues, "brouillon "+env.Sujet)
	return nil
}

// Envoyer note le message comme envoyé, sans rien transmettre.
func (c *Client) Envoyer(_ context.Context, env himalaya.Envoi, _, _ string) error {
	c.envoyés = append(c.envoyés, env)
	return nil
}

var _ himalaya.Client = (*Client)(nil)
