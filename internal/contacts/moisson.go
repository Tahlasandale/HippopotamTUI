package contacts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/cache"
	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
)

// Moissonneur reconstruit le carnet d'un compte en balayant ses boîtes.
type Moissonneur struct {
	Client himalaya.Client
	Cache  *cache.Store

	// ParPage est la taille des pages demandées à himalaya, et Maximum le
	// nombre de messages pris en compte au total.
	ParPage int
	Maximum int
}

// BoîtesÀBalayer renvoie les identifiants de boîte Sent et Inbox d'un compte.
//
// La recherche se fait sur le rôle déclaré par le serveur quand il existe, et
// sur le nom à défaut : Gmail ne signale pas le rôle « sent » mais sa boîte
// s'appelle toujours Sent.
func BoîtesÀBalayer(boîtes []himalaya.Boite) (sent, inbox string) {
	for _, b := range boîtes {
		switch {
		case b.Role == "sent" && sent == "":
			sent = b.ID
		case b.Role == "inbox" && inbox == "":
			inbox = b.ID
		}
	}
	for _, b := range boîtes {
		switch {
		case sent == "" && b.Role == "" && strings.EqualFold(b.Nom, "sent"):
			sent = b.ID
		case inbox == "" && b.Role == "" && strings.EqualFold(b.Nom, "inbox"):
			inbox = b.ID
		}
	}
	return sent, inbox
}

// ÀRafraîchir indique si le moisson d'un compte est trop vieux pour être réutilisé.
func (m *Moissonneur) ÀRafraîchir(compte string, jours int) bool {
	if m.Cache == nil {
		return true
	}
	_, quand, ok := m.Cache.Contacts(compte)
	if !ok {
		return true
	}
	if jours <= 0 {
		jours = 7
	}
	return time.Since(quand) > time.Duration(jours)*24*time.Hour
}

// Récolter balaye les boîtes Sent et Inbox et reconstruit le carnet.
//
// progression, si elle est fournie, reçoit le nombre de pages déjà parcourues et
// le nombre total attendu : c'est ce qui permet d'afficher une barre honnête au
// lieu d'un spinner qui tourne indéfiniment.
func (m *Moissonneur) Récolter(ctx context.Context, compte string, propres []string, progression func(faits, attendus int)) ([]Contact, error) {
	boîtes, err := m.Client.Boites(ctx, compte)
	if err != nil {
		return nil, fmt.Errorf("lecture des boîtes de %s : %w", compte, err)
	}

	sent, inbox := BoîtesÀBalayer(boîtes)
	cibles := []string{}
	for _, b := range []string{sent, inbox} {
		if b != "" {
			cibles = append(cibles, b)
		}
	}
	if len(cibles) == 0 {
		return nil, fmt.Errorf("aucune boîte exploitable dans le compte %s", compte)
	}

	parPage := m.ParPage
	if parPage <= 0 {
		parPage = 500
	}
	maximum := m.Maximum
	if maximum <= 0 {
		maximum = 2000
	}

	var enveloppes []himalaya.Enveloppe
	for _, boite := range cibles {
		restantes := maximum - len(enveloppes)
		if restantes <= 0 {
			break
		}
		pages := Pages(2000, parPage, restantes)
		if progression != nil {
			progression(len(enveloppes), maximum)
		}
		for page := 1; page <= pages; page++ {
			lot, err := m.Client.Enveloppes(ctx, compte, boite, page, parPage)
			if err != nil {
				return nil, fmt.Errorf("lecture de %s/%s : %w", compte, boite, err)
			}
			if len(lot) == 0 {
				break
			}
			enveloppes = append(enveloppes, lot...)
			if len(lot) < parPage {
				break
			}
		}
	}

	trouvés := Récolter(enveloppes, propres)
	if m.Cache != nil {
		if err := m.Cache.EnregistrerContacts(compte, versCache(trouvés), time.Now()); err != nil {
			return nil, fmt.Errorf("écriture du carnet local : %w", err)
		}
	}
	return trouvés, nil
}

// Lire relit le carnet mis en cache par le dernier moisson.
func (m *Moissonneur) Lire(compte string) []Contact {
	if m.Cache == nil {
		return nil
	}
	liste, _, ok := m.Cache.Contacts(compte)
	if !ok {
		return nil
	}
	trouvés := make([]Contact, 0, len(liste))
	for _, c := range liste {
		trouvés = append(trouvés, Contact{Nom: c.Nom, Email: c.Email})
	}
	return trouvés
}

func versCache(liste []Contact) []cache.Contact {
	out := make([]cache.Contact, 0, len(liste))
	for _, c := range liste {
		out = append(out, cache.Contact{Nom: c.Nom, Email: c.Email})
	}
	return out
}
