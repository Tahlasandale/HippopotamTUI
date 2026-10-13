# Dépannage

## Rien ne s'affiche

hippo demande au moins 40 colonnes et 10 lignes. En dessous, il affiche un
message qui le dit — sauf dans un terminal vraiment minuscule.

Si l'écran est figé sans le moindre texte, le terminal est probablement un
`dumb` :

```sh
echo $TERM
```

`xterm-256color`, `screen-256color` ou `tmux-256color` conviennent. Un terminal
sans truecolor n'est pas un problème : hippo descend tout seul en 256 puis en 16
couleurs.

## « himalaya : command not found »

hippo appelle himalaya, il ne l'embarque pas. Deux vérifications :

```sh
which himalaya
himalaya account list
```

Si le premier ne renvoie rien, himalaya n'est pas dans le `PATH`. Si le second
échoue, c'est la configuration de himalaya qui pose problème — et c'est le cas le
plus fréquent : `himalaya configure` et son
[mode diagnostic](https://himalaya.org/docs/troubleshooting.html).

Si himalaya est installé mais pas dans le `PATH` :

```toml
# ~/.config/hippo/config.toml
himalaya = "/chemin/vers/himalaya"
```

## L'interface met trois secondes à réagir

Un aller-retour IMAP sur le bridge Proton coûte facilement une seconde. Le cache
masque cela, mais seulement s'il est alimenté. Pour vérifier :

```sh
hippo cache statistiques
```

Si la colonne des messages est vide pour la boîte que vous consultez, la page
n'a jamais été enregistrée. Les causes possibles :

- `refresh_minutes` à 0 ou négatif : la valeur est ramenée à 5, donc ce n'est pas
  ça
- un cache en lecture seule, ou un répertoire d'état qui ne peut pas être écrit
- une boîte dont le nom change à chaque lecture

Dans les deux derniers cas, vider le cache et relancer :

```sh
hippo cache purge
```

## La liste est vide alors qu'il y a des messages

Vérifiez que vous êtes dans la bonne boîte. `←` et `→` changent de boîte, `m`
ouvre le sélecteur, et la barre d'en-tête dit toujours où l'on est.

Si une liste filtrée est vide alors que la boîte ne l'est pas, c'est que des
filtres sont actifs. Le bandeau d'en-tête les rappelle : `filtre : …`. `/` les
modifie, `échap` les annule.

## Un message s'affiche vide

Quelques défauts réels ont déjà été corrigés, mais un message peut être vraiment
étrange. Le journal dit ce que hippo a demandé :

```sh
J
```

Comparez avec un appel direct :

```sh
himalaya message read --raw -a proton -m Inbox <identifiant>
```

Si himalaya renvoie des octets et que hippo n'affiche rien, c'est un bug de
décodage : il mérite un rapport, avec le fichier `.eml` anonymisé en pièce jointe.

## L'autocomplétion ne propose rien

Le carnet est construit à partir de vos messages. S'il est vide :

```sh
hippo contacts
```

Puis vérifiez :

- des messages dans les boîtes `Envoyés` et `Réception` : sans correspondant,
  il n'y a rien à proposer
- que `hippo contacts` ne signale pas une erreur de lecture

Le carnet n'est pas reconstruit à chaque lancement : une fois par semaine, pour
ne pas faire cinq cents requêtes à un serveur de messagerie au démarrage. Après un
`hippo contacts`, `R` recharge la liste.

## Le champ « De » ne propose qu'un compte

C'est normal quand on n'a qu'un compte de messagerie. Si vous en avez plusieurs
et qu'un seul apparaît, c'est que les identités n'ont pas toutes été apprises.

Elles sont découvertes au démarrage en demandant un squelette vide à himalaya —
un calcul local, sans réseau, et sans lire votre configuration. Un compte qui
échoue à ce moment-là est simplement écarté de la liste ; le champ reste
affiché et n'est pas modifiable, sans conséquence : le message part par le
compte actif.

Pour vérifier ce qu'himalaya répond :

```sh
himalaya message compose --json -a proton
himalaya message compose --json -a gmail
```

## L'éditeur ne s'ouvre pas

Dans l'ordre :

1. `echo $EDITOR` — s'il est vide, hippo cherche `micro`, `nvim`, `vim`, `vi`
2. si aucun de ces binaires n'existe, mettez-le dans la configuration :
   ```toml
   editor = "nvim"
   ```
3. si l'éditeur s'ouvre et se ferme aussitôt, c'est qu'il renvoie une erreur :
   regardez ce qu'elle dit dans le bandeau

Un éditeur qui sort sur une erreur n'envoie rien, et le brouillon est conservé.

## Les couleurs sont bizarres

```sh
echo $COLORTERM     # truecolor, 256color, ou rien
echo $TERM
```

Avec `TERM=screen-256color` dans tmux, les couleurs 24 bits sont désactivées :
hippo retombe sur 256 couleurs, ce qui est correct mais moins fin.

`NO_COLOR` désactive complètement la couleur, comme le veut la convention.

## Les accents disparaissent dans la liste

C'est un problème d'encodage du terminal, pas de hippo. Vérifiez que
`LANG` ou `LC_ALL` vaut `fr_FR.UTF-8`, `C.UTF-8` ou équivalent.

## Démarrer sans toucher à rien

```sh
hippo --demo
```

Le mode démonstration ne lit aucun compte, ne fait aucun appel réseau et n'écrit
rien sur le disque. Il sert aussi à tester une capture d'écran.

## Repartir de zéro

```sh
hippo cache purge
rm -rf ~/.config/hippo
hippo contacts
```

Trois étapes : le cache, la configuration, le carnet. Vos comptes sont chez
himalaya et ne sont pas touchés.
## « Error: stream stopped responding after 60s »

Ce message vient de himalaya, pas de hippo : c'est lui qui coupe une connexion
qui ne répond plus. Souvent, la liste des messages fonctionne et l'ouverture d'un
message échoue — c'est un `FETCH` qui ne revient pas.

Sur un compte derrière un bridge local comme [monoxide][monoxide], c'est
généralement le bridge qui ne répond plus :

```sh
systemctl --user status monoxide
journalctl --user -u monoxide -n 50
```

Relancer le bridge suffit généralement. En attendant, `J` dans l'interface montre
quels appels ont réussi juste avant.

## « Error: FETCH returned no body for the requested message »

himalaya demande un corps et le serveur répond qu'il n'y en a pas. Ça arrive
avec certaines catégories de messages, notamment sur Gmail. Le message reste
listable, il n'est simplement pas lisible.

Ce n'est pas contournable depuis hippo : le corps n'existe pas côté serveur.

## Un message prend très longtemps à s'ouvrir

Le délai par défaut est de 90 secondes, au-delà duquel hippo abandonne. Sur une
connexion lente mais vivante, augmentez-le plutôt que de l'abaisser :

```toml
# ~/.config/hippo/config.toml
timeout_seconds = 180
```

Baisser ce délai n'aide pas : le message d'erreur de himalaya, qui dit pourquoi
la connexion a échoué, ne peut arriver que si hippo attend plus longtemps que lui.

[monoxide]: https://github.com/Tahlasandale/monoxide
