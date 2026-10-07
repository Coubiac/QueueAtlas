# Premiers outils CLI — chantier M4

## Lot128 : version et point d'entrée

Sources dans [cmd/queueatlas](../cmd/queueatlas/main.go). Depuis le dépôt :

```powershell
go run ./cmd/queueatlas version
```

Sortie sur stdout, code0 :

```text
QueueAtlas dev
```

Un build ordinaire est identifié par `dev`. Un producteur de build peut fournir
son étiquette avec le linker Go ; ce champ identifie le build déclaré, il ne
certifie pas un tag Git ou une release publiée. Exemple local :

```powershell
go build -ldflags='-X main.version=local-test' -o queueatlas.exe ./cmd/queueatlas
.\queueatlas.exe version
```

Sur Linux : choisir `-o queueatlas`, puis `./queueatlas version`. Le nom du fichier
de sortie est choisi par l'appelant. La commande affiche `QueueAtlas local-test`
dans cet exemple. Les tests compilent dans un répertoire temporaire pour vérifier
que le linker et l'exécutable utilisent réellement l'étiquette synthétique.

## Arguments, flux et codes

| Invocation | Sortie | Code |
| --- | --- | ---: |
| `queueatlas version` | Version et newline sur stdout | 0 |
| `queueatlas --help` ou `queueatlas -h` | Usage sur stdout | 0 |
| `queueatlas check-config --config <chemin>` valide | `Configuration valid` et newline sur stdout | 0 |
| `queueatlas check-config --help` ou `-h` | Usage de la commande sur stdout | 0 |
| Configuration refusée | Champ/règle connus sur stderr, aucune valeur fournie | 2 |
| Configuration illisible ou entrée non régulière | Diagnostic fixe sur stderr, aucun chemin | 1 |
| Sans commande, commande inconnue ou argument supplémentaire | Usage sur stderr, arguments non recopiés | 2 |
| Échec d'écriture de la sortie d'une commande valide | Diagnostic fixe sur stderr | 1 |

Le processus renvoie ces codes par `os.Exit`. `go run` interpose sa propre
exécution ; utiliser le binaire compilé pour observer directement le code2.
L'aide décrit les commandes implémentées. Version/aide fonctionnent sans
configuration, ouverture de base ou démarrage d'une source/réseau.

## Lot131 : check-config

```powershell
go run ./cmd/queueatlas check-config --config examples/queueatlas.yaml
```

Ou après compilation : `./queueatlas check-config --config /chemin/config.yaml`
sur Linux, `.\queueatlas.exe check-config --config C:\chemin\config.yaml` sous
PowerShell. Mettre entre guillemets un chemin avec espaces. Cette commande exige
exactement `check-config --config <chemin>` ; aucun chemin par défaut ou recherche
de fichier implicite. `--config=...`, options inconnues/répétées, valeurs vides et
arguments supplémentaires sont refusés (usage stderr/code2) avant chargement.
`check-config --help`/`-h` n'ouvre aucun fichier. Une valeur de chemin est traitée
littéralement ; aucune lecture stdin, expansion ou substitution par la CLI.

Le chargement applique le [contrat YAML](configuration.md) : paramètres serveur
local/chemin SQLite, défauts et bornes. En succès, stdout contient seulement
`Configuration valid` ; stderr vide. Un contenu invalide donne code2 et le
diagnostic par champ/règle du chargeur (sans valeur/chemin/nom de champ inconnu).
Une erreur IO donne code1 et `queueatlas: cannot read configuration`. La commande
ne modifie pas le fichier config, ne crée/ouvre aucune DB et ne démarre pas
d'ingestion/serveur. Elle ne certifie ni droits/disque local DB, ni disponibilité
de port, ni authentification/aptitude au déploiement du futur serveur.

Les codes réels sont vérifiés sur le binaire compilé ; `go run` interpose sa propre
gestion d'erreur et ne transmet pas directement un code2 à l'appelant. Une erreur
d'écriture stdout d'une commande valide donne code1 et le diagnostic fixe existant.

## Vérifications et suite

Quatre tests128 : version/aide et séparation des flux, arguments refusés sans
recopie, erreur d'écriture, binaire réellement compilé avec étiquette synthétique
et codes0/2. Tests/vet ciblés, `go run ... version`, format/diff locaux Windows
réussis. La CI Linux exécute aussi ces tests via `go test ./...` ; une étape CLI
Windows dédiée est ajoutée. Les builds statiques existants compilent le point
d'entrée amd64/arm64. Lot128 publié dans [PR #30](https://github.com/Coubiac/QueueAtlas/pull/30)
sur `921155a6accf9714ba5420ccb3e0bec658c1a0b9` ;
[CI37559870551](https://github.com/Coubiac/QueueAtlas/actions/runs/37559870551)
entière réussie, trois jobs et SHA exact vérifiés. Étape Windows CLI réussie.

Lot129 : [contrat initial de configuration](configuration.md), défauts et validation
pure du serveur local/chemin SQLite, publié surd9a2fb8 dans #30,
[CI37562294743](https://github.com/Coubiac/QueueAtlas/actions/runs/37562294743)
entière réussie/trois jobs/SHA exact vérifiés. Lot130 publié sur8bba11e dans #30,
[CI37564852420](https://github.com/Coubiac/QueueAtlas/actions/runs/37564852420)
entière réussie/trois jobs/SHA exact et étape Windows config vérifiés.

Lot131 : trois tests CLI nouveaux (flux/codes/readonly, arguments/aide, sortie
défaillante) et test du binaire étendu à check-config/codes0/1/2, sans création DB.
Sept tests CLI/vet/format/diff et commande go run sur l'exemple passés sous Windows.
Lot131 publié sur53d4ae0 dans #30,
[CI37567003805](https://github.com/Coubiac/QueueAtlas/actions/runs/37567003805)
entière réussie/trois jobs/SHA exact et étape Windows CLI vérifiés. Lot132 :
[relecture du chantier](reviews/m4-cli-config.md) favorable, sans modification de
code ; #30 fusionnée sur118634f, CI finale37569190697/main37569292737 entièrement
réussies, branche CLI supprimée. Lot133 : [ouverture SQLite de diagnostic](sqlite-diagnostics.md)
en lecture seule publiée sur41fbf0f dans #31, CI37571495723 entière réussie.
Lectures bornées des métadonnées134 validées localement ; publication/CI encore à
terminer au moment du commit. Prochain lot135 : db stats --config pour ces seules
métadonnées, sans compteur de lignes ni diagnostic d'intégrité implicite.
Doctor/db stats, configuration des composants, auth locale/API/Web
restent à développer. Exécutable de service et packaging/pilote restent M5.
Ce point d'entrée n'est pas une release installable ; MIT conservée, AD/OIDC après MVP.
