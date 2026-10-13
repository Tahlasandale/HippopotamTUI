# Écrire un message

## Ce qui se passe quand on appuie sur une touche

| Touche | Ce qui arrive |
|---|---|
| `c` | le formulaire s'ouvre, vide, et attend |
| `r`, `a`, `t` | le formulaire s'ouvre, puis votre éditeur s'ouvre aussitôt sur la citation |

Répondre, c'est presque toujours écrire : l'éditeur s'ouvre donc sans attendre.
Un message neuf, lui, mérite qu'on regarde d'abord ses destinataires et son
objet — et rien ne s'ouvre dans le dos.

Le formulaire tient en quatre champs :

```
┌─ La rédaction d'une réponse ────────────────────────────────────────────────────────────────────
│ Réponse                                                                                         │
│ ────────────────────────────────────────────────────────────────────────────────────────────────
│ De :   Orphin <orphin@protonmail.com>                                                            │
│ À :    Commission Scolaire <scolaire@example.org>                                                │
│ Cc :                                                                                             │
│ Objet : Re: Rapport de la commission scolaire                                                    │
│                                                                                                 │
│ > Rapport de la commission scolaire                                                              │
│                                                                                                 │
│ ctrl+s envoyer  ·  ctrl+r éditer  ·  échap abandonner                                            │
└──────────────────────────────────────────────────────────────────────────────────────────────────
```

`↑` `↓` ou `tab` passent d'un champ au suivant. `ctrl+r` ouvre le corps dans
votre éditeur, `ctrl+s` envoie, `échap` abandonne.

## Le champ « De »

C'est le premier champ, parce que c'est la première question : de quel compte
part ce message ? `←` et `→` passent d'un compte à l'autre.

```
De :   Orphin <orphin@protonmail.com>          ← →    De :   Meute Saint-Francois <meute@…>
```

Choisir une identité **change aussi le compte qui envoie**. Ce n'est pas une
détail d'affichage : himalaya sort par le serveur du compte actif, et faire
passer un message Gmail par le SMTP de Proton échouerait au dernier moment,
après toute la rédaction. Le compte choisi et l'adresse qu'il porte vont donc
ensemble.

Pour connaître vos adresses sans lire votre configuration — qui contient vos
mots de passe — `hippo` demande un squelette vide à himalaya au démarrage. C'est
un calcul local, sans réseau.

Pendant une rédaction, ce champ et la touche `S` font la même chose : les deux
portent sur l'envoi, ni l'un ni l'autre ne change la boîte que vous consultez.

## L'éditeur

Il ne s'ouvre que sur le corps, et le fichier ne contient rien d'autre :

```
Bonjour,

> Citation du message d'origine.
```

Les destinataires et l'objet ne sont pas dans ce fichier. Ils se règlent dans le
formulaire, où l'autocomplétion des adresses existe. Le fichier est supprimé
dans tous les cas, même si l'éditeur plante.

`ctrl+r` rouvre l'éditeur quand le corps ne vous convient pas, autant de fois
que vous voulez.

## Répondre

`r` répond à l'expéditeur. Si le message porte un `Reply-To`, c'est à cette
adresse qu'on répond, comme dans tous les clients.

## Répondre à tous

`a` reprend tous les destinataires. Deux règles, et elles sont les mêmes que
partout ailleurs :

- **ceux qui étaient en `À` restent en `À`**, ceux qui étaient en `Cc` restent en
  `Cc`. Répondre à tous ne transforme pas la conversation en liste de diffusion
- **vos propres adresses disparaissent** des deux listes, quelle que soit leur
  casse. C'est ce qui évite de se répondre à soi-même dans un fil où vous avez
  écrit

Une réponse à un message que **vous avez vous-même envoyé** est traitée comme
une réponse à tous : le seul expéditeur est vous, et la cible utile est
l'ensemble des destinataires d'origine. Un transfert, lui, ne reprend personne :
son destinataire est choisi à la main.

Le doublon est éliminé : une adresse présente en `À` et en `Cc` n'apparaît
qu'une fois.

## L'objet

Un objet qui commence déjà par `Re:` ne reçoit pas un second préfixe. `Re :
Invité` et `Re: Re : Invité` deviennent tous les deux `Re: Invité`.

C'est une retouche faite par `hippo` sur le résultat de himalaya, qui n'a pas
cette délicatesse. himalaya reste maître de `In-Reply-To` et `References` : si
vous changez l'objet dans le formulaire, le fil reste correctement rattaché.

## L'autocomplétion

himalaya n'a pas de carnet d'adresses. `hippo` en reconstruit un à partir de
vos messages : les boîtes `Envoyés` et `Réception` de chaque compte, deux mille
messages les plus récents au maximum, une fois par semaine.

On y trouve donc les gens à qui vous écrivez, ce qui est exactement ce qu'on
propose dans une réponse. Rien ne quitte la machine : les adresses sont dans le
cache local, sous `$XDG_STATE_HOME/hippo/cache.db`.

Pour reconstruire le carnet tout de suite :

```sh
hippo contacts
```

Le carnet est **moissonné**, pas appris : un contact que vous ne voyez plus
disparaît de la liste. C'est volontaire — un carnet figé devient une liste de
personnes que vous ne connaissez plus.

### La recherche

Tapez dans le champ `À` ou `Cc`, la liste se resserre :

- un préfixe l'emporte toujours sur une correspondance plus lâche : « ali »
  propose « Alice » avant « Aline »
- la recherche est **par sous-séquence** : « almt » trouve « Alice Martin »
- la casse et les accents ne comptent pas : « elodie » trouve « Élodie »
- taper une adresse propose l'adresse seule, taper un nom propose
  `Nom <adresse>`

Un préfixe `@` est obligatoire pour qu'une saisie devienne un destinataire. Une
adresse tapée à moitié est ignorée plutôt qu'envoyée en morceaux.

## Le transfert

`t` ouvre un formulaire sans destinataire : un transfert ne se devine pas, il se
choisit. Le message d'origine est encadré d'un séparateur, avec l'expéditeur,
l'objet et la date.

## Les brouillons

Quitter la rédaction avec `échap` enregistre un brouillon **si et seulement si**
le formulaire contient quelque chose. Un formulaire vidé n'enregistre rien : il
n'y a pas de corbeille de coquilles vides dans votre boîte de brouillons.

Ce qui n'est pas fait, et pourquoi : un brouillon n'est pas enregistré dès
l'ouverture de la rédaction. himalaya ne rend pas l'identifiant du brouillon
qu'il vient de créer, et sans cet identifiant on ne peut pas le supprimer de
façon fiable — on laisserait une copie à chaque envoi. Le brouillon est donc
écrit au moment où il sert vraiment : quand on abandonne.

## Les pièces jointes

Hors de la v1. La commande himalaya correspondante existe et fonctionne :

```sh
himalaya message send --attach fichier.pdf
```

## La signature

`hippo` n'en gère aucune : c'est himalaya qui l'ajoute, avec la configuration
que vous avez déjà. Dans votre `config.toml` de himalaya :

```toml
[accounts.proton]
signature = "Orphin"
```

Si la signature n'apparaît pas dans vos envois, c'est que cette ligne n'est pas
dans la configuration de himalaya — `hippo` n'y touche pas.