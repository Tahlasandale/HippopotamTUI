# Raccourcis

La navigation se fait aux flèches, parce qu'on la fait tout le temps et qu'elle
doit être prévisible. Les actions se font sur des lettres, parce qu'on les fait
peu et qu'il faut les retenir.

Les raccourcis `vim` (`j`, `k`, `h`, `l`, `g`, `G`) restent acceptés partout où
une flèche est attendue : taper `j` coûte moins cher que déplacer la main, et
personne n'a à choisir entre son terminal et son confort.

## La liste

| Touche | Effet |
|---|---|
| `↑` `↓` ou `k` `j` | ligne précédente, ligne suivante |
| `←` `→` ou `h` `l` | boîte précédente, boîte suivante |
| `début` `fin` ou `g` `G` | première, dernière ligne |
| `pg↑` `pg↓` | écran précédent, écran suivant |
| `entrée` | ouvrir le message |
| `M` | message suivant, sans repasser par la liste |
| `v` | marquer lu / non lu |
| `F` | marquer / démarquer |
| `d` ou `suppr` | mettre à la corbeille |
| `s` | déplacer vers une boîte |
| `m` | changer de boîte |
| `S` | changer de compte |
| `c` | écrire un nouveau message |
| `r` | répondre |
| `a` | répondre à tous |
| `t` | transférer |
| `/` | filtres guidés |
| `R` | rafraîchir depuis le serveur |
| `J` | journal des appels |
| `?` | aide |
| `q` | quitter |

`←` et `→` changent de boîte sans passer par le sélecteur : c'est le geste le
plus rapide pour aller de la réception aux envoyés.

## Le message

| Touche | Effet |
|---|---|
| `↑` `↓` | défiler |
| `←` `→` | message précédent, message suivant |
| `entrée` ou `n` | message suivant |
| `début` `fin` | haut, bas du message |
| `pg↑` `pg↓` | écran précédent, écran suivant |
| `v` | marquer lu / non lu |
| `F` | marquer / démarquer |
| `d` | mettre à la corbeille |
| `s` | déplacer vers une boîte |
| `r` `a` `t` | répondre, à tous, transférer |
| `q` ou `échap` | retour à la liste |

## La rédaction

Quatre champs : `De`, `À`, `Cc`, `Objet`.

| Touche | Effet |
|---|---|
| `↑` `↓` ou `tab` | champ précédent, suivant |
| `←` `→` | sur le champ `De`, changer de compte d'envoi ; ailleurs, rien |
| `entrée` | accepter l'adresse proposée sous le curseur |
| `ctrl+n` `ctrl+p` | choisir une autre proposition |
| `ctrl+r` | écrire le corps dans l'éditeur |
| `ctrl+s` | envoyer |
| `échap` | abandonner, en gardant un brouillon |

Sur `c`, l'éditeur ne s'ouvre pas tout seul : le formulaire d'abord. Sur `r`,
`a` et `t`, il s'ouvre aussitôt, parce qu'une réponse s'écrit presque toujours.

Le corps ne s'édite pas dans le TUI, il s'édite dans votre éditeur — c'est là que
vos raccourcis, vos corrections orthographiques et votre gestionnaire de fichiers
fonctionnent normalement.

## Les filtres

| Touche | Effet |
|---|---|
| `↑` `↓` | critère précédent, suivant |
| `←` `→` | cocher « non lus », faire glisser une date d'un mois |
| `espace` | cocher « non lus » |
| `u` | vider le critère courant |
| `entrée` | appliquer et revenir à la liste |
| `échap` | annuler |

Les critères sont combinés par « et ». La requête réellement envoyée à himalaya
est affichée en bas du panneau : il n'y a jamais de magie entre ce qu'on croit
avoir demandé et ce qui est parti.

## Les sélecteurs

Boîtes, comptes et destination d'un déplacement partagent le même sélecteur.

| Touche | Effet |
|---|---|
| `↑` `↓` | se déplacer dans la liste |
| `entrée` | valider |
| `échap` | annuler |

## L'aide et le journal

`?` et `J` s'ouvrent n'importe où. `échap` ou `q` referme.

## Surcharger une touche

Dans `~/.config/hippo/config.toml` :

```toml
[keys]
quitter = "ctrl+x"
```

La clé est le nom de l'action, la valeur une touche ou une séquence. La liste
des noms est celle de ce document. Une action jamais surchargée continue
d'utiliser sa touche d'origine : rien ne casse si le nom est mal orthographié.