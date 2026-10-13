package himalaya

import (
	"context"
	"testing"

	"github.com/Tahlasandale/HippopotamTUI/internal/rfc822"
)

// TestContreLeVraiHimalaya n'est utile que sur une machine où himalaya est
// configuré : c'est une vérification de bout en bout, pas un test unitaire.
func TestContreLeVraiHimalaya(t *testing.T) {
	if testing.Short() {
		t.Skip("besoin d'un vrai compte mail")
	}
	ctx := context.Background()
	c := &CLI{Bin: "himalaya"}

	comptes, err := c.Comptes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nom, _ := CompteParDéfaut(comptes)
	t.Logf("comptes : %s (défaut %s)", noms(comptes), nom)

	boîtes, err := c.Boites(ctx, nom)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range boîtes {
		t.Logf("boîte %-10s rôle=%s", b.ID, b.Role)
	}

	envs, err := c.Enveloppes(ctx, nom, "Inbox", 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range envs {
		t.Logf("enveloppe %s vu=%v « %s »", e.ID, e.A(PoussièreVolee), e.Sujet)
	}

	if len(envs) > 0 {
		brut, err := c.Message(ctx, nom, "Inbox", envs[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		msg, err := rfc822.Analyser(brut)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("message « %s » de %s, %d lignes de corps, %d pièces, markdown=%v",
			msg.Sujet, msg.De, len(msg.Corps), len(msg.Pièces), msg.EstMarkdown())
		t.Logf("date=%v", msg.Date)

		br, err := c.Squelette(ctx, Origine{Kind: Reponse, ID: envs[0].ID}, nom, "Inbox")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("squelette : de=%q à=%v objet=%q corps=%d caractères",
			br.De, br.To, br.Sujet, len(br.Corps))
	}
}

func noms(comptes []Compte) string {
	out := ""
	for _, c := range comptes {
		out += c.Nom + " "
	}
	return out
}
