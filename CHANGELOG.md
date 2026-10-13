# Journal des versions

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), et le
projet suit [le versionnement sémantique](https://semver.org/lang/fr/).

## [Non publié]

### Modifié

- **Rédaction** : le formulaire s'ouvre d'abord, l'éditeur attend. Écrire un
  message neuf ne lance plus micro dans le dos ; répondre le lance toujours, parce
  qu'une réponse s'écrit presque toujours.
- **Rédaction** : le fichier ouvert dans l'éditeur ne contient plus que le corps.
  Les en-têtes qui y étaient écrits étaient retirés au retour sans un mot ;
  ils se règlent dans le formulaire, où l'autocomplétion existe.
- **Rédaction** : le formulaire porte un champ « De » qui dit de quel compte le
  message part, et se change aux flèches. Choisir une identité change aussi le
  compte qui envoie, pour que l'adresse et le serveur d'envoi concordent.
- **Rédaction** : le titre du formulaire dit ce qu'on fait — Réponse, Réponse à
  tous, Transfert — au lieu de « Nouveau message » partout.
- **Rédaction** : la barre du bas tient sur une ligne, sans troncature, et
  contient les trois seules actions : envoyer, éditer, abandonner.
- **Préparation** : une rédaction annonce son préparation. L'écran ne semble
  plus figé pendant une à deux secondes.

## [0.1.0] — 2026-10-13

Première version : un pager de messagerie complet, au-dessus de himalaya.

### Ajouté

- **Liste** : affichage paginé depuis le cache, défilement aux flèches,
  changement de boîte aux flèches ou par sélecteur, changement de compte
- **Message** : lecture du corps complet, parties multiples, HTML ramené à du
  texte, rendu Markdown quand le message le mérite, pièces jointes listées,
  défilement et message suivant sans repasser par la liste
- **Actions** : lu / non lu, marque, corbeille, déplacement vers une boîte
- **Rédaction** : nouveau message, réponse, réponse à tous, transfert, dans
  l'éditeur du système ; autocomplétion des adresses ; brouillon conservé à
  l'abandon
- **Carnet** : moissonnage des boîtes Envoyés et Réception de chaque compte,
  correspondance par sous-séquence, insensible à la casse et aux accents
- **Filtres** : constructeur guidé de six critères combinés par « et », aperçu de
  la requête réellement envoyée à himalaya
- **Diagnostic** : journal de tous les appels, bandeau d'erreur non bloquant,
  commande de cache
- **Configuration** : fichier TOML facultatif suivant l'arbitrage XDG, surcharge
  ponctuelle de touches
- **Mode démonstration** : `hippo --demo`, sans compte ni réseau
- **Documentation** : README, six documents, captures d'écran régénérables par
  `hippo capture`, page de manuel générée par `hippo man`
- **Développement** : TDD, CI sur Linux et macOS, captures vérifiées par la CI

### Corrigé

- Décodage MIME des messages dont les parties sont mal formées : paramètre sans
  valeur, multipart annoncé inexistant, en-têtes placés dans le corps, pièce
  jointe sans nom de fichier
- Objet de réponse qui n'empile plus les préfixes : « Re: Re : x » devient
  « Re: x »
- Citation de secours quand himalaya renvoie un squelette vide

[Non publié]: https://github.com/Tahlasandale/HippopotamTUI/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Tahlasandale/HippopotamTUI/releases/tag/v0.1.0
