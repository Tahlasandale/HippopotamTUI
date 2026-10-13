package himalaya

import (
	"context"
	"os"
	"testing"

	"github.com/Tahlasandale/HippopotamTUI/internal/rfc822"
)

// TestContreLeVraiHimalaya confronte le client à la vraie boîte de réception.
// C'est une vérification de bout en bout, pas un test unitaire : elle touche le
// réseau et elle n'a de sens que sur une machine où himalaya est configuré.
//
// Elle est donc hors du chemin par défaut. Pour la lancer :
//
//	HIPPO_TEST_REEL=1 go test ./internal/himalaya/ -run ContreLeVrai -v
//
// C'est ce test qui a trouvé les trois défauts de décodage MIME corrigés en
// 0.1.0, et il mérite d'être lancé après toute modification du client.
func TestContreLeVraiHimalaya(t *testing.T) {
	if os.Getenv("HIPPO_TEST_REEL") == "" {
		t.Skip("définir HIPPO_TEST_REEL=1 pour utiliser un vrai compte mail")
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
