# HippopotamTUI

[![Vérification](https://github.com/Tahlasandale/HippopotamTUI/actions/workflows/ci.yml/badge.svg)](https://github.com/Tahlasandale/HippopotamTUI/actions/workflows/ci.yml)

Un client de messagerie en ligne de commande, léger, rapide et entièrement
documenté en français.

Il ne fait pas de messagerie : c'est [himalaya][himalaya] qui s'en charge.
HippopotamTUI (`hippo`) lui ajoute une interface en terminal, un carnet
d'adresses reconstruit à partir de vos propres messages, une rédaction qui
s'ouvre dans votre éditeur habituel, et rien d'autre.

```
hippo                la boîte de réception
hippo --demo         une démonstration, sans aucun compte
```

```
┌─ La boîte de réception ─────────────────────────────────────────────────────────────────────────
│ hippo · proton / Inbox                                                                      1/10
│ ────────────────────────────────────────────────────────────────────────────────────────────────
│ • 21:23 Commission Scolaire    📎 Rapport de la commission scolaire
│ • 17:23 Mairie                 Réunion du conseil municipal
│   12/10 Scolarité              Votre attestation d'inscription
│ ★ 11/10 Emploi                 Candidature reçue
│   10/10 Énergie                📎 Facture du mois
│ ────────────────────────────────────────────────────────────────────────────────────────────────
│ ↑↓ naviguer  ·  ←→ boîte  ·  entrer lire  ·  r répondre  ·  a à tous  ·  t transférer  ·  …
└──────────────────────────────────────────────────────────────────────────────────────────────────
```

## Ce que le programme ne fait pas

C'est important, et c'est volontaire.

- **Il ne lit jamais votre configuration de himalaya.** Ce fichier contient vos
  mots de passe en clair. `hippo` ne lui demande que des listes et des actions,
  par la ligne de commande. Rien d'autre.
- **Il n'impose pas d'éditeur de texte.** Il appelle `$EDITOR`, ou `micro`,
  `nvim`, `vim` ou `vi`. Ce que vous utilisez déjà, avec vos raccourcis.
- **Il ne réécrit pas vos fils de discussion.** `In-Reply-To`, `References`, le
  `Re:` : c'est himalaya qui les pose. `hippo` prépare les champs, lui fait
  l'assemblage.
- **Il ne demande pas confirmation.** Supprimer met à la corbeille, ce qui est
  réversible. Une confirmation à chaque action, c'est une friction permanente
  pour une fois par jour qu'on appuie sur la mauvaise touche.

## Installation

### Avec Go

```sh
go install github.com/Tahlasandale/HippopotamTUI@latest
```

Le binaire s'installe dans `$(go env GOPATH)/bin`. Ajoutez ce répertoire à votre
`PATH` s'il n'y est pas déjà.

### Depuis une version

Des binaires sont publiés pour Linux, macOS et FreeBSD, en `amd64` et `arm64` :

```sh
curl -fsSL https://github.com/Tahlasandale/HippopotamTUI/releases/latest/download/install.sh | sh
```

### Ce qu'il faut avoir

- [himalaya][himalaya] v2, configuré, avec au moins un compte
- un terminal d'au moins 40 colonnes et 10 lignes

hippo ne configure pas himalaya à votre place : `himalaya configure` le fait très
bien, et vos identifiants ne doivent pas transiter par un programme de plus.

## Premiers pas

```sh
hippo                     # ouvrir la boîte de réception du compte par défaut
hippo contacts            # reconstruire le carnet d'adresses (une fois)
hippo --demo              # essayer sans compte, sans réseau
```

Le bandeau d'accueil n'apparaît qu'une fois. Ensuite, `?` donne tous les
raccourcis, et la barre du bas n'affiche que ceux de l'écran courant.

## Ce qu'il sait faire

- **lire** : liste paginée depuis le cache, défilement, message complet, parties
  multiples, HTML ramené à du texte, pièces jointes listées
- **agir** : lu / non lu, marque, corbeille, déplacement vers une boîte
- **écrire** : nouveau message, réponse, réponse à tous, transfert, brouillon
- **rechercher** : un constructeur de filtres guidé, en français
- **changer de boîte ou de compte** : sans jamais quitter l'interface
- **comprendre** : un journal de tous les appels faits à himalaya, et le détail
  d'une erreur quand ça échoue

Ce qu'il ne fait pas encore : joindre un fichier, télécharger une pièce jointe.
Ces deux actions existent côté himalaya, et le TUI n'a pas vocation à les
réinventer tout de suite.

## Documentation

| Document | Ce qu'on y trouve |
|---|---|
| [docs/raccourcis.md](docs/raccourcis.md) | Toutes les touches, écran par écran |
| [docs/composition.md](docs/composition.md) | Répondre, répondre à tous, transférer, autocomplétion, brouillons |
| [docs/configuration.md](docs/configuration.md) | Le fichier de configuration, clé par clé |
| [docs/architecture.md](docs/architecture.md) | Comment c'est fait, et pourquoi |
| [docs/depannage.md](docs/depannage.md) | Quand ça ne marche pas |
| [docs/captures/](docs/captures/) | Les écrans, régénérables par `hippo capture` |

Une page de manuel s'installe avec :

```sh
go install github.com/Tahlasandale/HippopotamTUI@latest
man $(go env GOPATH)/bin/hippo
```

## Développement

Le projet suit le TDD : aucun code avant un test qui échoue, et les tests font
partie du même commit que ce qu'ils testent.

```sh
make test        # tous les tests, avec le détecteur de courses
make couverture  # le détail par paquet
make compile     # le binaire
make captures    # régénère docs/captures/
make man         # régénère les pages de manuel
```

La coloration, les raccourcis, le mode démonstration et le journal de bord
existent en partie pour que l'interface soit testable sans terminal ni compte
mail. C'est aussi ce qui permet de régénérer la documentation sans capture
d'écran datée.

## Licence

MIT. Voir [LICENSE](LICENSE).

[himalaya]: https://github.com/soywiz/himalaya