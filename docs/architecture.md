# Architecture

Ce document explique comment le programme est construit, et surtout **pourquoi**.
Une décision d'architecture qui n'est pas expliquée est une décision qu'on
reprend dans six mois sans savoir si elle est encore la bonne.

## Vue d'ensemble

```
hippo (cmd/hippo)
└── internal/cli          la ligne de commande : drapeaux, sous-commandes, aide
    └── internal/ui       le TUI : un modèle bubbletea, des vues, un clavier
        ├── internal/himalaya    l'appel à la CLI, et ses types
        ├── internal/rfc822      le décodage des octets RFC 5322
        ├── internal/redaction   la logique pure : destinataires, filtres
        ├── internal/contacts    le carnet moissonné et la correspondance
        ├── internal/cache       le stockage local, en bbolt
        ├── internal/config      les préférences et les chemins
        └── internal/demo        un client qui ne touche jamais au réseau
```

Chaque paquet dépend de ceux du dessous, jamais l'inverse. `redaction` ne sait
rien du terminal, `rfc822` ne sait rien de himalaya, `cache` ne sait rien de
l'interface. C'est ce qui rend le programme testable morceau par morceau.

## Cinq décisions, et leurs raisons

### 1. Ne jamais lire la configuration de himalaya

La configuration de himalaya contient les mots de passe en clair. Un programme
qui la lit est un programme qui, le jour où il est compromis, donne les mots de
passe avec.

`hippo` ne fait que lancer la commande et lire sa sortie JSON. Il n'a aucune
raison de connaître un serveur IMAP, un port, ou une chaîne de caractères qui
ressemble à un secret. Le seul coût est de dépendre d'un outil externe — un
coût que le projet assume entièrement, puisque c'est himalaya qui travaille.

Ce choix a un corollaire : si un jour on veut gérer les secrets avec
`pass` ou `secret-tool`, c'est la configuration de himalaya qui change, et ce
programme n'a pas besoin d'être touché.

### 2. Décoder les octets bruts plutôt que le JSON de himalaya

Deux façons de lire un message :

```sh
himalaya message read --json   # la structure interne de l'outil
himalaya message read --raw    # les octets RFC 5322 du message
```

Le JSON de himalaya est verbeux et peu structuré pour l'usage : énumérations
Rust sérialisées, décalages d'octets dans les en-têtes, index de parties à
recouper. Rien ne garantit que cette forme soit stable d'une version à l'autre,
et elle change sans préavis quand l'outil évolue.

Les octets bruts, eux, sont normisés par un protocole qui a trente ans. La
bibliothèque standard suffit à les lire : `net/mail`, `mime`, `mime/quotedprintable`,
`mime/multipart`.

Le bénéfice concret est double : le TUI n'est pas cassé par une mise à jour de
himalaya, et les tests d'entrée sont de vrais fichiers `.eml` — écrits à la main,
versionnés dans `internal/rfc822/testdata/`, sans aucune donnée personnelle dans
un dépôt public.

Ce choix a un coût assumé : le décodage des charsets et du quoted-printable est
à écrire. Il est écrit, et testé.

### 3. himalaya assemble, `hippo` prépare

Une réponse pose `In-Reply-To`, `References` et `Re:`. Réinventer ça serait
l'erreur la plus facile à commettre et la plus coûteuse : un fil de discussion
mal rattaché disparaît du fil, et le rattrapage est manuel.

Le chemin est donc : himalaya produit les champs décodés, `hippo` les présente et
laisse l'utilisateur les corriger, puis himalaya réassemble et envoie.

```go
br, _ := client.Squelette(ctx, Origine{Kind: Reponse, ID: "1550"}, compte, boite)
// br.To, br.Cc, br.Sujet, br.Corps : déjà calculés par himalaya
// l'utilisateur corrige, l'éditeur est appelé, puis :
client.Envoyer(ctx, env, compte, boite)
```

Deux retouches sont faites par `hippo`, parce que ce sont des details d'affichage
et pas de protocole : un objet qui ne doit pas s'empiler (`Re: Re : x` →
`Re: x`), et une citation de secours quand himalaya renvoie un corps vide, ce qui
arrive avec certaines parties MIME mal formées.

### 4. bbolt plutôt que SQLite

Le cache est du clé-valeur : une page de messages, un index de contacts, des
horodatages. Rien à quoi poser une requête SQL.

`bbolt` est en Go pur, tient dans un seul fichier, écrit dans une transaction, et
ne demande ni `cgo` ni serveur. Le programme se compile avec `CGO_ENABLED=0` et
s'installe sans bibliothèque à reconstruire. SQLite aurait apporté une base
relationnelle complète que rien ici n'utilise.

Le cache n'est pas une vérité : c'est une accélération. Tout ce qu'il contient
peut être reconstruit depuis himalaya. Un cache corrompu est donc supprimé et
recréé au lieu de faire échouer le lancement.

### 5. Le terminal n'est pas le centre du programme

L'interface est pilotable sans terminal : le mode démonstration fournit un jeu de
messages en mémoire, et `hippo capture` rend chaque écran dans un fichier.

Trois raisons, dans l'ordre d'importance :

1. **les tests**. Les tests d'interface simulent des touches et lisent l'état du
   modèle. Ni terminal, ni compte mail, ni network. Ils sont rapides et
   reproductibles.
2. **la documentation**. Les écrans de ce dépôt sont le texte que le programme
   produit réellement. Quand l'affichage change, `hippo capture` les régénère :
   la documentation ne peut pas mentir.
3. **contribuer**. On peut ouvrir le projet sans avoir de compte mail.

## Le modèle

Un seul modèle `ui.Modèle` décrit tout l'état : compte courant, boîte, page,
liste d'enveloppes, curseur, message affiché, formulaire, écran, message
d'erreur.

Les écrans ne sont pas des modèles bubbletea séparés. Un TUI comme celui-ci n'a
pas cinq panes à faire vivre en parallèle, et cinq modèles qui se parlent
finissent toujours par se désynchroniser. Un seul modèle, un `Update` central, et
des vues qui lisent cet état.

Les opérations longues sont des commandes : elles rendent la main tout de suite
et renvoient un résultat que `Update` applique. C'est ce qui permet d'afficher un
message pendant que l'appel réseau qui l'a coûté se termine encore, et d'afficher
une barre d'état honnête au lieu d'un écran figé.

## Le cache et la fraîcheur

| Écran | Appelle le réseau quand |
|---|---|
| Naviguer dans la liste | jamais, si le cache a moins de `refresh_minutes` |
| Ouvrir un message | toujours |
| Marquer, déplacer, supprimer | toujours, et la ligne change avant le retour |
| Changer de boîte, de compte, de filtres | toujours |
| Ouvrir l'éditeur | toujours |

La ligne sur laquelle on agit change à l'écran avant que l'appel ne parte. On
voit ce qu'on a fait, et l'attente n'est jamais une absence de réaction.

## Ce qui manque, et pourquoi

- **pièces jointes à l'envoi et au téléchargement**. himalaya sait déjà faire les
  deux. Les faire dans le TUI, c'est un sélecteur de fichiers à écrire et
  maintenir pour une opération qui a déjà une commande.
- **multi-sélection**. Vider une boîte de cent messages, un par un, est pénible.
  Mais la multi-sélection dans un pager gêne plus qu'elle n'aide à l'usage
  quotidien, et elle double l'interface. Les actions himalaya acceptent déjà
  plusieurs identifiants : c'est le premier ajout naturel quand le besoin se
  fait sentir.
- **rédaction dans le TUI**. On l'écrit, mais pas bien. Tant qu'on a un éditeur
  dans le `PATH`, l'y utiliser est meilleur.
- **langue anglaise**. L'interface est en français, comme la documentation.
  Trancher autrement demande de labourer un fichier de traduction, pas de
  réécrire des chaînes.

## Tests

| Paquet | Ce qui est testé |
|---|---|
| `config` | défauts, chemins XDG, fichier partiel, surcharge de touches |
| `himalaya` | arguments envoyés, décodage du JSON, remontée des erreurs |
| `rfc822` | décodage MIME sur de vrais `.eml`, y compris des messages cassés |
| `redaction` | destinataires d'une réponse, requête de filtres, troncature |
| `contacts` | moisson, dédoublonnage, correspondance par sous-séquence |
| `cache` | persistance, clés distinctes, cache corrompu |
| `ui` | navigation, actions, rédaction, filtres, aide, journal |

Les tests d'interface utilisent le client de démonstration. Ils simulent les
touches et exécutent les commandes, comme le ferait la boucle bubbletea, ce qui
les garde rapides et déterministes.