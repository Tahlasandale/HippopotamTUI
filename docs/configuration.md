# Configuration

`hippo` se configure peu, et tout est facultatif : le programme est
utilisable sans aucun fichier.

Le fichier se trouve à cet emplacement, qui suit l'arbitrage XDG :

| | |
|---|---|
| Linux | `~/.config/hippo/config.toml` |
| macOS | `~/.config/hippo/config.toml` |

`hippo config path` affiche les trois chemins employés : configuration, cache et
journal.

## Créer le fichier

```sh
hippo config init
```

La commande écrit un fichier commenté, avec les valeurs en cours. `hippo config
afficher` montre ensuite la configuration réellement utilisée, et la valeur par
défaut de chaque clé.

## Les clés

```toml
# Chemin du binaire himalaya. À changer seulement s'il n'est pas dans le PATH.
himalaya = "himalaya"

# Éditeur utilisé pour écrire le corps d'un message. Vide : hippo prend $EDITOR,
# puis micro, nvim, vim, vi, dans cet ordre.
editor = ""

# Nombre de messages demandés à himalaya par page.
page_size = 25

# Minutes au-delà desquelles la liste affichée est rechargée depuis le serveur.
refresh_minutes = 5

# Jours avant que le carnet d'adresses soit reconstruit au démarrage.
contacts_refresh_days = 7

# Secondes avant qu'un appel réseau soit abandonné.
timeout_seconds = 90

# Surcharge ponctuelle d'une touche.
[keys]
quitter = "ctrl+x"
```

| Clé | Défaut | Ce qu'elle change |
|---|---|---|
| `himalaya` | `himalaya` | Le binaire appelé. Utile si himalaya est installé ailleurs que dans le `PATH`. |
| `editor` | *(vide)* | La commande qui édite le corps. Vide, la détection s'en charge. |
| `page_size` | `25` | Le nombre d'enveloppes par page. Une valeur plus élevée donne moins d'aller-retours, à condition que le cache tienne en mémoire. |
| `refresh_minutes` | `5` | Après combien de minutes la liste affichée est rechargée. `R` force le rafraîchissement. |
| `contacts_refresh_days` | `7` | À quel rythme le carnet est reconstruit au démarrage. |
| `timeout_seconds` | `90` | Délai au-delà duquel un appel réseau est abandonné. À augmenter sur une connexion lente, jamais à diminuer. |
| `keys` | *(vide)* | Surcharge de touches, voir [raccourcis](raccourcis.md). |

Une valeur invalide — une page de zéro, une durée négative — est ramenée à son
défaut plutôt que de faire échouer le lancement.

## Ce que `hippo` ne configure pas

Vos comptes, vos serveurs, vos mots de passe : ils sont dans la configuration de
himalaya, et `hippo` ne la lit pas. Voir `himalaya configure`, et le
[fichier de documentation de himalaya][config].

## Répertoire d'état

Le cache et le journal vivent sous `$XDG_STATE_HOME/hippo`, soit
`~/.local/state/hippo` :

| Fichier | Contenu |
|---|---|
| `cache.db` | une base [bbolt][bbolt] : pages de messages et carnet d'adresses |
| `hippo.log` | envois et erreurs, datés |

Le journal ne contient volontairement que peu de choses : ni navigation, ni
adresses, ni objets. Un journal de TUI ne doit pas devenir une seconde boîte
aux lettres sur le disque.

### Vider le cache

```sh
hippo cache purge
```

Sans conséquence : le cache sera reconstruit au prochain lancement, et le
carnet remoissonné au bout de sept jours, ou immédiatement avec `hippo contacts`.

### Voir ce qu'il contient

```sh
hippo cache statistiques
```

[config]: https://himalaya.org/docs/config.html
[bbolt]: https://github.com/etcd-io/bbolt