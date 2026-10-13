// Package cache garde localement ce qui a déjà été téléchargé.
//
// Sur le bridge Proton, un aller-retour IMAP coûte facilement une seconde : sans
// cache, naviguer dans une boîte revient à attendre. Le cache sert donc deux
// fois : la liste reste instantanée, et les contacts moissonnés restent
// disponibles hors ligne.
//
// bbolt a été retenu parce qu'il est en Go pur, qu'un seul fichier suffit et que
// ce cache n'a aucune raison d'être relationnel : c'est du clé-valeur avec des
// transactions, rien de plus.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Tahlasandale/HippopotamTUI/internal/himalaya"
	bolt "go.etcd.io/bbolt"
)

// Contact est une adresse connue, prête à être proposée pendant la frappe.
type Contact struct {
	Nom   string `json:"nom"`
	Email string `json:"email"`
}

// Affichage rend le contact tel qu'il doit apparaître dans la liste de
// propositions : le nom quand il existe, l'adresse seule sinon.
func (c Contact) Affichage() string {
	if c.Nom == "" {
		return c.Email
	}
	return fmt.Sprintf("%s <%s>", c.Nom, c.Email)
}

// Store est le cache sur disque.
type Store struct {
	chemin string
	db     *bolt.DB
}

var (
	seauEnveloppes = []byte("enveloppes")
	seauHorodatage = []byte("horodatage")
	seauContacts   = []byte("contacts")
	refixeContacts = "contacts:"
)

// Ouvrir ouvre le cache, en créant le fichier et son répertoire au besoin.
func Ouvrir(chemin string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(chemin), 0o700); err != nil {
		return nil, fmt.Errorf("création du répertoire du cache : %w", err)
	}

	db, err := bolt.Open(chemin, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		// Un cache corrompu ne doit pas empêcher le TUI de démarrer : on le
		// signale et on repart d'un fichier neuf.
		if erreur := os.Remove(chemin); erreur != nil {
			return nil, fmt.Errorf("cache illisible %s : %w", chemin, err)
		}
		db, err = bolt.Open(chemin, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	}
	if err != nil {
		return nil, fmt.Errorf("ouverture du cache %s : %w", chemin, err)
	}

	store := &Store{chemin: chemin, db: db}
	for _, seau := range [][]byte{seauEnveloppes, seauHorodatage, seauContacts} {
		if err := db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists(seau)
			return err
		}); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialisation du cache : %w", err)
		}
	}
	return store, nil
}

// Chemin renvoie l'emplacement du fichier de cache.
func (s *Store) Chemin() string { return s.chemin }

// Fermer referme le fichier.
func (s *Store) Fermer() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Purger vide le cache. La commande « hippo contacts --oublier » s'en sert pour
// repartir d'un moisson propre.
func (s *Store) Purger() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, seau := range [][]byte{seauEnveloppes, seauHorodatage, seauContacts} {
			if err := tx.DeleteBucket(seau); err != nil && !errBoltSupprimé(err) {
				return err
			}
			if _, err := tx.CreateBucketIfNotExists(seau); err != nil {
				return err
			}
		}
		return nil
	})
}

func errBoltSupprimé(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

func cléEnveloppes(compte, boite string, page int) string {
	return fmt.Sprintf("%s\x00%s\x00%d", compte, boite, page)
}

func cléHorodatage(compte, boite string) string {
	return fmt.Sprintf("env\x00%s\x00%s", compte, boite)
}

// EnregistrerEnveloppes range une page d'enveloppes et date le rafraîchissement.
func (s *Store) EnregistrerEnveloppes(compte, boite string, page int, envs []himalaya.Enveloppe, à time.Time) error {
	if envs == nil {
		envs = []himalaya.Enveloppe{}
	}
	données, err := json.Marshal(envs)
	if err != nil {
		return fmt.Errorf("sérialisation des enveloppes : %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(seauEnveloppes).Put([]byte(cléEnveloppes(compte, boite, page)), données); err != nil {
			return err
		}
		return tx.Bucket(seauHorodatage).Put([]byte(cléHorodatage(compte, boite)), horodatage(à))
	})
}

// ToucherEnveloppes date un rafraîchissement sans nouveau contenu, ce qui arrive
// quand la boîte n'a pas bougé.
func (s *Store) ToucherEnveloppes(compte, boite string, à time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(seauHorodatage).Put([]byte(cléHorodatage(compte, boite)), horodatage(à))
	})
}

// Enveloppes relit une page en cache. La dernière valeur indique si la page
// existe : une boîte réellement vide et une page jamais téléchargée ne se
// confondent pas.
func (s *Store) Enveloppes(compte, boite string, page int) ([]himalaya.Enveloppe, time.Time, bool) {
	var (
		envs    []himalaya.Enveloppe
		quand   time.Time
		trouvée bool
	)

	err := s.db.View(func(tx *bolt.Tx) error {
		brut := tx.Bucket(seauEnveloppes).Get([]byte(cléEnveloppes(compte, boite, page)))
		if brut == nil {
			return nil
		}
		trouvée = true
		if err := json.Unmarshal(brut, &envs); err != nil {
			trouvée = false
			return nil
		}
		// Une enveloppe relue du cache peut n'avoir que le champ brut de
		// himalaya : on reporte l'indicateur comme à la réception.
		himalaya.Normaliser(envs)
		quand = relireHorodatage(tx.Bucket(seauHorodatage).Get([]byte(cléHorodatage(compte, boite))))
		return nil
	})
	if err != nil || !trouvée {
		return nil, time.Time{}, false
	}
	return envs, quand, true
}

// HorodatageEnveloppes donne la date du dernier rafraîchissement d'une boîte.
func (s *Store) HorodatageEnveloppes(compte, boite string) (time.Time, bool) {
	var (
		quand  time.Time
		trouvé bool
	)
	s.db.View(func(tx *bolt.Tx) error {
		brut := tx.Bucket(seauHorodatage).Get([]byte(cléHorodatage(compte, boite)))
		if brut != nil {
			quand = relireHorodatage(brut)
			trouvé = quand.After(time.Time{})
		}
		return nil
	})
	return quand, trouvé
}

func horodatage(à time.Time) []byte {
	return []byte(à.UTC().Format(time.RFC3339Nano))
}

func relireHorodatage(brut []byte) time.Time {
	if brut == nil {
		return time.Time{}
	}
	quand, err := time.Parse(time.RFC3339Nano, string(brut))
	if err != nil {
		return time.Time{}
	}
	return quand
}

// EnregistrerContacts remplace l'index d'un compte. Les contacts sont triés
// avant écriture pour que la lecture n'ait plus rien à décider.
func (s *Store) EnregistrerContacts(compte string, contacts []Contact, à time.Time) error {
	triés := append([]Contact(nil), contacts...)
	sort.Slice(triés, func(i, j int) bool {
		gauche, droite := strings.ToLower(triés[i].Affichage()), strings.ToLower(triés[j].Affichage())
		return gauche < droite
	})

	données, err := json.Marshal(triés)
	if err != nil {
		return fmt.Errorf("sérialisation des contacts : %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		// On repart d'un index propre : un contact qui n'existe plus ne doit pas
		// survivre à un nouveau moisson.
		cursor := tx.Bucket(seauContacts).Cursor()
		var àSupprimer [][]byte
		for k, _ := cursor.Seek([]byte(refixeContacts)); k != nil; k, _ = cursor.Next() {
			if !strings.HasPrefix(string(k), refixeContacts+compte+"\x00") {
				continue
			}
			àSupprimer = append(àSupprimer, append([]byte(nil), k...))
		}
		for _, clé := range àSupprimer {
			if err := tx.Bucket(seauContacts).Delete(clé); err != nil {
				return err
			}
		}

		if err := tx.Bucket(seauContacts).Put([]byte(refixeContacts+compte), données); err != nil {
			return err
		}
		return tx.Bucket(seauHorodatage).Put([]byte("contacts\x00"+compte), horodatage(à))
	})
}

// Contacts relit l'index d'un compte avec la date du moisson.
func (s *Store) Contacts(compte string) ([]Contact, time.Time, bool) {
	var (
		liste  []Contact
		quand  time.Time
		trouvé bool
	)

	err := s.db.View(func(tx *bolt.Tx) error {
		brut := tx.Bucket(seauContacts).Get([]byte(refixeContacts + compte))
		if brut == nil {
			return nil
		}
		trouvé = true
		if err := json.Unmarshal(brut, &liste); err != nil {
			trouvé = false
			return nil
		}
		quand = relireHorodatage(tx.Bucket(seauHorodatage).Get([]byte("contacts\x00" + compte)))
		return nil
	})
	if err != nil || !trouvé {
		return nil, time.Time{}, false
	}
	return liste, quand, true
}
