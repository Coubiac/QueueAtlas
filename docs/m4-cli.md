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
| `queueatlas admin create --directory <chemin> --username <nom> --password-stdin` valide | `Local administrator created` sur stdout | 0 |
| Syntaxe/identité/mot de passe refusés | Diagnostic fixe sur stderr | 2 |
| Compte déjà créé/stockage ou stdin indisponibles/échec de création | Diagnostic fixe sur stderr | 1 |
| `queueatlas check-config --config <chemin>` valide | `Configuration valid` et newline sur stdout | 0 |
| `queueatlas check-config --help` ou `-h` | Usage de la commande sur stdout | 0 |
| Configuration refusée | Champ/règle connus sur stderr, aucune valeur fournie | 2 |
| Configuration illisible ou entrée non régulière | Diagnostic fixe sur stderr, aucun chemin | 1 |
| `queueatlas db stats --config <chemin>` avec base compatible | Un objet JSON et newline sur stdout, six métadonnées | 0 |
| Base absente/illisible/incompatible ou diagnostic échoué | Diagnostic fixe sur stderr, aucune donnée partielle | 1 |
| `queueatlas db --help` ou `db stats --help` (aussi `-h`) | Usage sur stdout, sans lecture | 0 |
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

## Lot135 : db stats

```powershell
go run ./cmd/queueatlas db stats --config C:\chemin\queueatlas.yaml
```

Après compilation : `./queueatlas db stats --config /chemin/queueatlas.yaml` sur
Linux ou `.\queueatlas.exe db stats --config C:\chemin\queueatlas.yaml` sous
PowerShell. Un fichier YAML valide doit désigner une base QueueAtlas existante,
compatible, locale et protégée. La commande ne crée pas la base manquante ;
l'exemple du dépôt n'est donc pas une démonstration réussie sans base préalable.
Un chemin SQLite relatif est résolu par le chargeur depuis le répertoire lexical
du fichier YAML, comme pour check-config.

Syntaxe unique `db stats --config <chemin>`, option/valeur séparées. Pas de config
implicite, `--config=...`, option répétée, sous-commande inconnue ou argument
supplémentaire : usage stderr/code2 avant chargement. Les aides `db --help`/`-h`
et `db stats --help`/`-h` fonctionnent sans configuration ou base.

En succès : un objet JSON compact, newline, stderr vide/code0. Champs stables :
`schema_version`, `sqlite_version`, `journal_mode`, `page_size`, `page_count`,
`free_page_count`. Valeurs définies dans le [contrat de métadonnées](sqlite-diagnostics.md#lot134--métadonnées-au-même-snapshot).
Ce sont des pages logiques incluant le WAL validé, pas des compteurs de messages,
une occupation physique du disque ou une preuve d'intégrité/sauvegarde. Aucun
chemin, nom de table, identité, adresse ou journal n'est imprimé.

Configuration invalide : diagnostic champ/règle existant, code2. Fichier config
illisible : `queueatlas: cannot read configuration`, code1. Base absente/illisible
ou entrée non régulière : `queueatlas: cannot open database for diagnostics`, code1.
Version/historique incompatible : `queueatlas: database schema is not supported for diagnostics`,
code1. Autre échec de lecture : `queueatlas: cannot read database diagnostic metadata`,
code1. Expiration du contexte SQLite : `queueatlas: database diagnostic timed out`,
code1. Messages fixes, aucun pilote/valeur/chemin recopié, stdout vide sur ces
échecs. La connexion est fermée avant toute sortie réussie ; échec de fermeture
signalé par message fixe/code1. Échec stdout : diagnostic partagé/code1, une sortie
partielle reste possible si le flux échoue après avoir accepté des octets.

La phase SQLite partage un contexte coopératif de10secondes après le chargement,
avec busy5s du lecteur. Ce n'est pas une deadline dure pour tout IO/chargement.
Aucune création/migration/checkpoint applicatif, ni ingestion/serveur démarré.
SQLite peut créer/utiliser les auxiliaires WAL/SHM ; limites de chemins/droits/
compatibilité133–134 conservées.

## Lot137 : doctor

```powershell
go run ./cmd/queueatlas doctor --config C:\chemin\queueatlas.yaml
```

Après compilation : `./queueatlas doctor --config /chemin/queueatlas.yaml` sur
Linux ou `.\queueatlas.exe doctor --config C:\chemin\queueatlas.yaml` sous
PowerShell. La configuration doit désigner une base QueueAtlas existante ; un
chemin SQLite relatif est résolu depuis le répertoire lexical du YAML.

Syntaxe stricte `doctor --config <chemin>` : option/valeur séparées, pas de chemin
implicite, option répétée ou argument supplémentaire. Usage stderr/code2 avant
chargement en cas d'arguments refusés ; `doctor --help`/`-h` donne usage stdout/code0
sans configuration ni base.

Succès : stdout contient exactement cet objet compact et une newline, stderr
vide/code0 :

```json
{"configuration":"valid","database":"compatible"}
```

`configuration` indique que le chargeur strict existant a accepté le YAML et ses
paramètres. `database` indique que le lecteur en mode lecture seule a vérifié
la version7 et la présence de l'historique requis ; la connexion est fermée avant
la sortie. Ce résultat décrit les vérifications effectuées à l'ouverture. Il ne
certifie ni intégrité/authenticité de la base, ni disponibilité d'un service ou
aptitude au déploiement ; droits/chemins et auxiliaires SQLite restent soumis aux
[limites du lecteur](sqlite-diagnostics.md). Aucun chemin, identité, journal ou
valeur de configuration n'est imprimé. Aucun compteur ou scan supplémentaire.

Configuration invalide : diagnostic champ/règle sûr existant, code2. Config
illisible : `queueatlas: cannot read configuration`, code1. Base absente/illisible,
non régulière ou corrompue : `queueatlas: cannot open database for diagnostics`,
code1. Schéma incompatible : `queueatlas: database schema is not supported for diagnostics`,
code1. Expiration : `queueatlas: database diagnostic timed out`, code1. Fermeture
échouée : `queueatlas: cannot close database diagnostic connection`, code1.
Ces erreurs n'impriment pas de résultat partiel sur stdout. Échec d'écriture stdout :
`queueatlas: cannot write output`, code1 ; le flux peut avoir accepté des octets.

Le contexte coopératif SQLite est de10secondes après le chargement ; il ne couvre
pas les IO de configuration et n'est pas une deadline dure. Aucune création,
migration, checkpoint applicatif, ingestion, connexion réseau ou service démarré.
SQLite peut utiliser/créer des auxiliaires WAL/SHM. La commande ne lit pas les
métadonnées de pages de `db stats` : elle vérifie seulement la compatibilité.

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
Lectures bornées134 publiées sur8e9008b,
[CI37572091851](https://github.com/Coubiac/QueueAtlas/actions/runs/37572091851)
entière réussie, trois jobs/SHA exact vérifiés. Lot135 : quatre tests CLI nouveaux
et binaire étendu à db stats/codes0/1/2 ; onze tests CLI et vet/format/diff passés
sous Windows. Lot135 publié surcae194c,
[CI37574528679](https://github.com/Coubiac/QueueAtlas/actions/runs/37574528679)
entière réussie, trois jobs/SHA exact revérifiés à la reprise136.
[Relecture136](reviews/m4-diagnostics.md) favorable ; #31 fusionnée sur918ef0c,
[CI finale37576814504](https://github.com/Coubiac/QueueAtlas/actions/runs/37576814504)
et [CI main37576942301](https://github.com/Coubiac/QueueAtlas/actions/runs/37576942301)
entières réussies, trois jobs/SHA exact vérifiés ; branche diagnostics supprimée.

Lot137 : quatre nouveaux tests doctor et test du binaire étendu aux codes0/1/2,
JSON à deux champs fixes, refus sans création/migration et base/config inchangées.
Quinze tests CLI, vet/format/diff Windows passés. Code de stockage/chargeur et
workflow inchangés ; lot137 publié sur d1feeb9 dans #32,
[CI37579618027](https://github.com/Coubiac/QueueAtlas/actions/runs/37579618027)
entière réussie/trois jobs/SHA exact revérifiés à la reprise138.
[Relecture138](reviews/m4-doctor.md) favorable sur ce diagnostic initial, sans
modification de code ; #32 fusionnée sur0d2cad4,
[CI finale37581947764](https://github.com/Coubiac/QueueAtlas/actions/runs/37581947764)
et [CI main37582076645](https://github.com/Coubiac/QueueAtlas/actions/runs/37582076645)
entières réussies/trois jobs/SHA exact, branche doctor supprimée.
La vérification des sources/formats/checkpoints/lacunes du cadrage reste ultérieure.
Le [contrat source139](configuration.md#lot139--contrat-pur-dune-source-fichier)
était une valeur Go indépendante. Le [chargeur140](configuration.md#lot140--section-yaml-source-facultative)
accepte désormais une section facultative source pour une entrée fichier.
`check-config --config examples/queueatlas-source.yaml` valide l'exemple synthétique,
même avec journal absent, sans lancer d'ingestion. Doctor conserve son périmètre
config/compatibilité SQLite et ne certifie pas la lisibilité de la source.
YAML140 publié sur455148a dans #33,
[CI37588258338](https://github.com/Coubiac/QueueAtlas/actions/runs/37588258338)
entière réussie/trois jobs/SHA exact. La [conversion141](configuration.md#lot141--conversion-vers-filesource)
est disponible en bibliothèque : copie indépendante revalidée, chemin absolu et
reprise stricte. Aucun appel CLI ni démarrage applicatif ; doctor garde ses limites.
Vingt-quatre tests config/vet/format/diff Windows passés ; conversion141 publiée
sur223849c, [CI37591355873](https://github.com/Coubiac/QueueAtlas/actions/runs/37591355873)
entière réussie/trois jobs/SHA exact. [Relecture142](reviews/m4-source-config.md)
favorable ; publication/CI finale/fusion/main encore à terminer au commit142.
Clôture142 effective : #33 fusionnée sur19843d6, CI finale37594338289/main37594545438
entières réussies, branche sources supprimée. Le [contrat auth143](local-auth.md)
prépare identité locale/coûts Argon2id en bibliothèque ; l'initialisation CLI est
disponible au146, décrite ci-dessous. Route login et sessions restent à développer.
Configuration des composants, auth locale/API/Web
restent à développer. Exécutable de service et packaging/pilote restent M5.
Ce point d'entrée n'est pas une release installable ; MIT conservée, AD/OIDC après MVP.

## Lot146 : admin create

Après build, syntaxe unique (ordre des options requis) :

```text
queueatlas admin create --directory <répertoire-existant> --username <identifiant> --password-stdin
```

`admin --help` et `admin create --help` (aussi `-h`) affichent l'aide sans IO.
Le répertoire doit exister, être privé/fiable et accessible à l'utilisateur du
processus ; Linux exige aucun droit groupe/autres (normalement0700), record0600.
La CLI ne crée/chmode pas les parents et ne modifie pas les ACL Windows.
Chemin relatif évalué depuis le cwd, indépendamment du YAML/SQLite.

Exemple **Bash sur Linux**, dans un shell sans traçage `set -x`, après préparation
d'un répertoire privé. Le secret est fourni sans echo par le shell et transmis par
son builtin printf ; aucune valeur de secret littérale dans la commande/historique :

```bash
unset qa_password
read -r -s -p 'Mot de passe généré ou passphrase : ' qa_password
printf '\n' >&2
builtin printf '%s' "$qa_password" | ./queueatlas admin create --directory ./queueatlas-auth --username operator --password-stdin
qa_status=$?
unset qa_password
printf 'Code de sortie : %s\n' "$qa_status"
```

Employer un mot de passe généré par un gestionnaire ou une longue passphrase
différente des exemples publics. Le pipe doit fermer son entrée : EOF requis.
Sous Windows, fournir également un pipe/fichier **UTF-8** depuis une source privée,
avec ACL adaptées ; aucune saisie console directe, option password ou lecture
de variable d'environnement n'est prévue. Une conversion d'encodage par le shell
peut changer le secret ; vérifier le fournisseur stdin utilisé.

Entrée :15..256points de code UTF-8/1024octets max ; au plus un LF/CRLF final retiré,
CR/LF restants refusés. Espaces et autres octets acceptés conservés pour le hash.
Liste locale initiale finie de valeurs courantes/dérivées, comparaisons complètes
sans casse/espaces autour, pas de contrôle réseau ou composition imposée ; limites
et détails dans [auth locale](local-auth.md#lot146--initialisation-cli).

Code0 : `Local administrator created`, stderr vide ; fichier local-admin.json avec
identité/hash uniquement, sel frais et création atomique sans remplacement145.
Code2 : syntaxe/identité/mot de passe ou liste refusés ; raison fixe sans valeur.
Code1 : IO/hash/création, compte déjà présent, ou sortie échouée. Aucun diagnostic
n'affiche chemin/nom/hash/secret. Compte existant/corrompu refusé avant lecture stdin,
compte manquant dans stockage privé seul autorise la création.

Une erreur de finalisation tardive indique explicitement qu'un compte est déjà
créé ; inspecter le stockage avant de réessayer. Une erreur stdout ne supprime pas
le compte publié. Il n'existe aucune commande de reset/remplacement dans ce lot.
Lecture bornée1027octets, mais pas de délai pour un pipe local bloqué. Mémoire shell/
copies/effacement physique non garantis ; buffer CLI effacé au mieux. Cette commande
initialise le compte ; aucun serveur, login, cookie ou accès Web encore disponible.

Correction147 après relecture : les secrets constitués uniquement d'espaces
ASCII/Unicode sont refusés par la politique de création (code2), sans hash/fichier.
Les espaces d'une passphrase acceptée sont conservés ; les comptes existants ne
changent pas. Tests ciblés CLI/binaire/auth/vet Windows passés, CI finale/fusion du
chantier à terminer au commit147. [Relecture](reviews/m4-local-account.md) et
conditions requises avant login détaillées ; prochaine étape148 sessions bornées.
