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
| Sans commande, commande inconnue ou argument supplémentaire | Usage sur stderr, arguments non recopiés | 2 |
| Échec d'écriture de la sortie d'une commande valide | Diagnostic fixe sur stderr | 1 |

Le processus renvoie ces codes par `os.Exit`. `go run` interpose sa propre
exécution ; utiliser le binaire compilé pour observer directement le code2.
L'aide décrit les commandes implémentées. Cette commande fonctionne sans
configuration, ouverture de base ou démarrage d'une source/réseau.

## Vérifications et suite

Quatre tests128 : version/aide et séparation des flux, arguments refusés sans
recopie, erreur d'écriture, binaire réellement compilé avec étiquette synthétique
et codes0/2. Tests/vet ciblés, `go run ... version`, format/diff locaux Windows
réussis. La CI Linux exécute aussi ces tests via `go test ./...` ; une étape CLI
Windows dédiée est ajoutée. Les builds statiques existants compilent le point
d'entrée amd64/arm64. Publication/CI128 à vérifier après commit.

Prochain lot129 : contrat de configuration et valeurs par défaut, par petit lot
distinct. Chargement YAML durci, check-config, doctor/db stats, auth locale/API/Web
restent à développer. Exécutable de service et packaging/pilote restent M5.
Ce point d'entrée n'est pas une release installable ; MIT conservée, AD/OIDC après MVP.
