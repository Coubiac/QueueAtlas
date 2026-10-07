# Configuration — contrat initial M4

Lots129–130 : `internal/config` définit un socle de paramètres, une validation pure
et le chargement YAML. La CLI `check-config --config <chemin>` utilise ce chargeur
depuis le lot131 ; le serveur reste à implémenter. Un
[fichier d'exemple](../examples/queueatlas.yaml) est chargé dans les tests.

## Valeurs par défaut et bornes

| Champ | Défaut | Règle initiale |
| --- | --- | --- |
| `server.listen` | `127.0.0.1:8080` | IP loopback littérale IPv4/IPv6, port numérique de1 à65535 |
| `server.read_header_timeout` | `5s` | De `1ms` à `1m`, inclus |
| `server.idle_timeout` | `1m` | De `1ms` à `10m`, inclus |
| `server.shutdown_timeout` | `15s` | De `1ms` à `1m`, inclus |
| `storage.path` | `queueatlas.db` | Chemin de fichier selon l'OS, absolu ou relatif |

`Defaults()` renvoie une valeur indépendante par appel. Le chargeur part
de ces valeurs et remplace seulement les champs présents ; un zéro explicite
reste invalide. `Config.Validate()` ne corrige ni ne normalise la configuration.
Ces délais sont des limites de contrat initiales, pas des mesures de performance
ni des garanties de comportement HTTP : le serveur reste à implémenter.

L'adresse exige la notation `IP:port` ou `[IPv6]:port`. `localhost`, bind vide,
wildcards, IP distantes et adresses avec zone sont refusés, sans résolution DNS.
Les adresses IPv4 mappées en IPv6 sont traitées comme leur IP IPv4. Aucune écoute
distante n'est proposée tant que les contrôles d'authentification et TLS ne sont
pas configurables ensemble. Le loopback ne dispense jamais le futur serveur de
protéger ses routes de données par le compte local (ADR-006).

## Chemin SQLite

Le chargeur résout un chemin relatif depuis le **répertoire du fichier de
configuration** et renvoie un chemin absolu destiné au stockage.
Le défaut portable vise une configuration dans un répertoire d'état privé ; le
futur paquet Linux devra fournir explicitement un chemin sous `/var/lib/queueatlas`
pour une configuration installée dans `/etc/queueatlas`.

Sont refusés : vide, espaces en début/fin, caractères de contrôle, chemin sans
nom de fichier (`.`, `..`, racine, séparateur final), `:memory:`, URI `file:` ou
contenant `://`, partages UNC (`\\` ou `//` en début). Les chemins ne sont ni
ouverts ni créés ; un parent absent est accepté syntaxiquement. Un chemin local
peut néanmoins pointer vers un montage réseau, un répertoire, un lien, ou un
emplacement sans droits suffisants : cette validation ne certifie rien de cela.
Le stockage conserve ses contrôles d'ouverture ; le diagnostic applicatif et
le déploiement devront vérifier droits, fichier régulier et disque local/WAL.

## Diagnostics, vérifications et suite

Une erreur permet `errors.Is(err, config.ErrInvalid)` et identifie le premier
champ refusé avec sa règle. Elle ne contient ni valeur fournie ni erreur brute
du parseur d'adresse. Les tests emploient uniquement des données synthétiques.

Cinq tests ciblés couvrent : défauts valides/indépendants et configuration zéro,
IP/ports et absence de mutation, bornes inclusives/délais zéro, syntaxe des chemins
sans création d'état, absence de valeurs privées dans les diagnostics. Tests,
vet, format et diff locaux Windows réussis. Étape config Windows ajoutée à la CI ;
les jobs Linux existants couvrent le package par test/vet/build statique.
Lot129 publié sur `d9a2fb8fe8d50718cbda30b2b0f9ed9b26cc4954` dans #30 ;
[CI37562294743](https://github.com/Coubiac/QueueAtlas/actions/runs/37562294743)
entière réussie, trois jobs/SHA exact et étape Windows config vérifiés.

## Lot130 : chargement YAML strict et borné

`Load(path)` lit un fichier régulier puis ferme son descripteur, en succès comme
en échec. `Decode(reader, baseDir)` traite un lecteur avec un répertoire de base
absolu fourni par l'appelant. L'un comme l'autre renvoient une configuration zéro
sur toute erreur ; aucun état partiellement accepté ne peut être utilisé.

- Au plus **64 Kio**, commentaires compris : lecture de 65 537 octets au maximum
  pour détecter un dépassement, avant parsing. UTF-8 exigé, exactement un document
  mapping ; fichier vide/commentaires seuls/null refusés, `{}` accepte les défauts.
- Structure vérifiée après parsing : profondeur maximale 4 depuis le nœud document,
  au plus 128 nœuds (clés comprises). Les alias ne sont pas développés ; ancres,
  alias, merge keys, champs inconnus et doublons sont refusés.
- Sections `server`/`storage` : mappings ; champs et valeurs : scalaires textuels.
  Null, booléen/nombre implicite, séquence, mapping à la place d'une valeur ou tag
  personnalisé sont refusés. Une durée porte une unité (`5s`, `1m`, etc.) puis
  respecte les bornes du contrat129. Une chaîne explicitement typée est acceptée.
- Validation du chemin fourni **avant** résolution, puis de la configuration
  résolue. Les chemins relatifs Windows dépendant d'un lecteur (`C:fichier.db`) ou
  enracinés sans lecteur (`\fichier.db`) sont refusés. `..` explicite est permis :
  aucune restriction au répertoire de config n'est revendiquée. Variables
  d'environnement, `~` et templates ne sont jamais développés.
- Diagnostics fixes avec champ/règle connus : `ErrInvalid` pour contenu refusé,
  `ErrRead` pour IO. Aucun nom de champ inconnu, chemin privé, valeur YAML ou erreur
  brute du lecteur/parseur n'est recopié. Le chargement n'ouvre jamais la base.

Le répertoire est celui du chemin lexical absolu fourni à `Load`, même si le
fichier est un lien. La lecture ne certifie ni les permissions/ACL du fichier
config ni sa stabilité face à un remplacement concurrent. Le contrôle de fichier
régulier avant/après ouverture n'est pas un verrou ; la borne d'octets n'est pas
une échéance pour un lecteur ou système de fichiers bloquant.

Le parseur est figé sur `go.yaml.in/yaml/v3 v3.0.5` dans go.mod/go.sum ; lecture en
`yaml.Node`, contrôle du schéma sans conversion implicite ou expansion d'alias.
[Documentation du module](https://pkg.go.dev/go.yaml.in/yaml/v3@v3.0.5) et
[source/licences](https://github.com/yaml/go-yaml/tree/v3.0.5) consultées : fichiers
sous MIT et Apache-2.0 ; la licence MIT de QueueAtlas reste inchangée. L'inventaire
et les notices de distribution restent au jalon M5.

Sept tests130 : chargement/fermeture/défauts sans création DB, valeurs/durées/chemins
et absence d'expansion, entrées ambiguës/types/nulls et diagnostics privés, bornes
d'octets/structure, erreurs IO et répertoire de base, chemins Windows ambigus,
exemple du dépôt. Suite config (douze tests), vet/format/diff locaux Windows passés.
L'étape Windows config existante et les jobs Linux couvrent le chargeur ;
Lot130 publié sur `8bba11eb3215ab3386dd511702920c348694c8fd` dans #30 ;
[CI37564852420](https://github.com/Coubiac/QueueAtlas/actions/runs/37564852420)
entière réussie, trois jobs/SHA exact et étape Windows config vérifiés.

Lot131 : [CLI check-config](m4-cli.md#lot131--check-config), codes0/1/2, diagnostics
sanitisés, tests du binaire sans création DB ; publié sur53d4ae0,
[CI37567003805](https://github.com/Coubiac/QueueAtlas/actions/runs/37567003805)
entière réussie/trois jobs/SHA exact vérifiés. [Relecture132](reviews/m4-cli-config.md)
favorable ; #30 fusionnée sur118634f, CI finale37569190697/main37569292737
entièrement réussies. [Ouverture SQLite de diagnostic133](sqlite-diagnostics.md)
et métadonnées134 publiées/CI verte ; [db stats135](m4-cli.md#lot135--db-stats) publié
surcae194c, CI37574528679 entière réussie. Relecture136 favorable, #31 fusionnée
sur918ef0c, CI finale37576814504/main37576942301 entières réussies.
[doctor137](m4-cli.md#lot137--doctor) réutilise ce chargeur puis le lecteur SQLite
readonly pour vérifier seulement la configuration et la compatibilité existante.
Les sources, CIDR/domaines, rétention et paramètres d'authentification demanderont
des contrats séparés selon les composants raccordés. `serve`,
auth/API/Web restent à développer. AD/OIDC après MVP, MIT conservée.

## Lot139 : contrat pur d'une source fichier

`config.FileSource` décrit une entrée fichier en Go. `FileSourceDefaults()` fournit
les paramètres facultatifs ; ID, Name et Path restent obligatoires. `Validate()`
refuse le premier champ invalide via `ErrInvalid` et une règle fixe, sans recopier
les valeurs. La valeur est copiée, sans slice/map/pointeur partagé.

Au lot139 ce contrat était indépendant de `Config`. Le lot140 le charge désormais
dans la section facultative `source` décrite ci-dessous ; le raccordement à
FileSource reste au lot141. Aucun composant n'est lancé par validation/chargement.

| Champ Go | Défaut | Contrat |
| --- | --- | --- |
| ID | Aucun, requis | 1–128octets ASCII ; premier caractère alphanumérique, puis alphanumérique, `.`, `_` ou `-` |
| Name | Aucun, requis | 1–128octets UTF-8, sans espaces périphériques ni caractères de contrôle |
| TrustedHost | Vide | Facultatif ; identifiant d'instance ASCII, syntaxe de ID, au plus255octets |
| Path | Aucun, requis | Chemin littéral de fichier selon l'OS, 1–4096octets UTF-8 |
| StartAt | beginning | beginning ou end seulement |
| PollInterval | 1s | 10ms–1m inclus |
| RotationGrace | 30s | 10ms–24h inclus |
| ResumeOrigins | 1000 | 1–1000 inclus, budget des états parcourus |
| ResumeEntries | 2000 | 1–2000 inclus, budget partagé des entrées de répertoires |

Les délais/budgets et modes réutilisent les constantes de la bibliothèque FileSource.
Contrairement aux zéros qui sélectionnent des défauts dans son constructeur,
le contrat applicatif exige que les champs présents soient valides : durées/budgets
zéro et StartAt vide sont refusés. Le futur chargeur remplacera seulement les champs
présents après application des défauts. Les politiques de rejeu d'un checkpoint zéro
restent strictes ; aucune option applicative de relaxation n'est ajoutée.

ID et TrustedHost sont des clés opaques sensibles à la casse, sans normalisation
ou résolution DNS ; TrustedHost ne certifie pas un hôte lu dans les logs. Le stockage
existant utilise TrustedHost comme instance si renseigné, sinon l'ID de source.
L'opérateur devra garder ces clés stables et attribuer correctement les instances ;
le contrat d'une seule valeur ne contrôle pas les doublons entre sources.

Path accepte absolu ou relatif, `..`, espaces internes et texte littéral `${...}`,
`~` ou templates, sans expansion/résolution. Refus : vide, espaces périphériques,
contrôles, UTF-8 invalide, dépassement4096octets, absence de nom de fichier,
séparateur final, URI, `:memory:`, UNC et caractères joker `*`/`?`. Sous Windows,
chemin relatif à un lecteur (`C:fichier.log`) ou enraciné sans lecteur (`\fichier.log`)
refusé. La borne syntaxique ne garantit pas les limites physiques de l'OS.
Aucun fichier/parent n'est ouvert, créé ou vérifié ; répertoire, lien, montage réseau,
droits, lisibilité et format des lignes ne sont pas attestés.

beginning conserve le départ normal. end sera transmis au FileSource existant :
bootstrap sans historique uniquement, frontière LF complète, reprises/rotations
inchangées selon [ADR-010](adr/ADR-010-initial-file-end.md). La validation du mot
end ne démontre pas ces conditions physiques ou durables.

Quatre nouveaux tests139 : défauts indépendants/obligatoires, identité/nom/instance
et confidentialité, chemins sans IO/mutation/expansion et ambiguïtés Windows,
bornes inclusives/débordements/zéros/modes. Seize tests config, vet/format/diff
locaux Windows passés ; CI de publication à vérifier après commit. Aucun changement
du chargeur, de la CLI, du stockage ou de l'ingestion ; pas de dépendance nouvelle.

Validation139 effective : cfd2b39 publié dans #33,
[CI37585141938](https://github.com/Coubiac/QueueAtlas/actions/runs/37585141938)
entière réussie/trois jobs/SHA exact ; les attentes139 ci-dessus sont le snapshot
prépublication. La même PR est réutilisée pour le chargement140 et la suite.

## Lot140 : section YAML source facultative

Un mapping racine `source` décrit **une source fichier**. Section absente :
`Config.Source == nil`, aucune source créée implicitement, anciennes configurations
acceptées. Section présente : ID/nom/chemin obligatoires, autres champs selon
FileSourceDefaults. Un mapping vide/null, une séquence ou une seconde section
source sont refusés ; sources multiples et autres types restent à développer.

[Exemple synthétique](../examples/queueatlas-source.yaml) :

```yaml
source:
  id: synthetic-postfix
  name: Synthetic Postfix
  path: missing-parent/synthetic-mail.log
  start_at: beginning
  poll_interval: 1s
  rotation_grace: 30s
  resume_origins: 1000
  resume_entries: 2000
```

Champ facultatif supplémentaire : `trusted_host`, texte d'instance selon139.
Type implicite fichier ; aucun champ type ou allow_zero_checkpoint accepté.
Les textes et durées suivent les scalaires textuels stricts130. Les budgets sont
des **entiers décimaux non quotés**, sans signe, préfixe hex/octal, séparateur,
zéro initial superflu ou conversion implicite ; même `!!int '10'` est refusé.
Leurs valeurs doivent être positives et dans les bornes139. Seuls les champs
absents conservent les défauts : valeurs null/zéro/mode vide refusées.

Bornes globales inchangées : 64Kio, 128nœuds, profondeur4, un seul document UTF-8.
Ancres/alias/merge keys, doublons, champs inconnus et tags personnalisés refusés.
Erreurs ErrInvalid avec champs/règles connus ; aucune valeur, clé inconnue ou
erreur brute YAML n'est imprimée. Toute erreur renvoie `Config{}`, Source nil.

Validation avant résolution de Path, résolution relative depuis le répertoire
lexical absolu du YAML, puis validation complète incluant la longueur résolue.
Un chemin absolu garde sa valeur ; `..` est permis, aucune restriction au répertoire
de config revendiquée. Variables/tilde/templates restent littéraux. Aucune vérification
de la source physique, de ses droits/format, ou lecture de journal/checkpoint.
Chaque chargement alloue sa propre valeur Source ; copier un Config en Go ne fait
pas une copie profonde du pointeur, l'appelant reste propriétaire de ses paramètres.

`check-config --config examples/queueatlas-source.yaml` accepte cet exemple même
si le journal est absent, sans créer/ouvrir de source ou base. Le résultat de doctor
reste limité à config et compatibilité SQLite ; il ne certifie pas la lisibilité
d'une source désormais configurée. Construction/ingestion et diagnostic élargi
restent ultérieurs.

Cinq nouveaux tests140 : chargement/defaults/absence sans IO, valeurs/chemins et
indépendance, refus sûrs/config zéro, limites globales conservées, exemple.
Vingt-et-un tests config et quinze tests CLI passés Windows ; binaire compilé
étendu avec une config source et journal absent, codes/effets/confidentialité vérifiés.
Vet/format/diff ciblés et commande check-config sur l'exemple passés. Stockage,
FileSource, dépendances et workflow inchangés ; publication/CI140 à terminer au commit.

Validation140 effective : 455148a publié dans #33,
[CI37588258338](https://github.com/Coubiac/QueueAtlas/actions/runs/37588258338)
entière réussie/trois jobs/SHA exact ; les attentes140 ci-dessus sont le snapshot
prépublication, terminé.

## Lot141 : conversion vers FileSource

Après Load/Decode et contrôle de `Config.Source != nil`,
`Config.Source.LibraryConfig()` renvoie une valeur `filesource.Config` indépendante.
La méthode revalide tous les champs et exige un chemin absolu déjà résolu ;
un chemin relatif validé lexicalement au lot139 est refusé ici. Aucun défaut,
normalisation, résolution, accès aux dépendances ou IO pendant la conversion.
Toute erreur renvoie la valeur bibliothèque zéro et ErrInvalid/champ-règle fixe,
sans valeur fournie. L'appelant doit traiter l'erreur avant d'utiliser le résultat.

Copie exacte ID/nom/instance facultative, kind=file, chemin, mode beginning/end
typé, délais et budgets. Les deux valeurs sont indépendantes après conversion.
ResumePolicy reste zéro strict : AllowZeroCheckpoint=false, sans paramètre YAML
pour assouplir la reprise. TrustedHost et end conservent les limites139/140 :
aucune preuve d'hôte, de fichier lisible ou de conditions physiques/durables.

Trois nouveaux tests : copie/défauts/valeurs explicites/indépendance, revalidation
et erreurs sûres/zéro, acceptation par le constructeur FileSource beginning/end.
Le test du constructeur utilise un journal absent et des dépendances sentinelles,
sans lecture d'état, normalisation, fichier créé ou Run. La méthode de production
ne construit aucun composant ; l'ingestion applicative reste à développer.
Vingt-quatre tests config/vet/format/diff locaux Windows passés ; publication/CI141
à vérifier après commit dans #33, puis revue/clôture142 du chantier139–141.

Validation141 effective : 223849c publié dans #33,
[CI37591355873](https://github.com/Coubiac/QueueAtlas/actions/runs/37591355873)
entière réussie/trois jobs/SHA exact revérifiés à la reprise142 ; attentes141
prépublication terminées. [Relecture142](reviews/m4-source-config.md) favorable
au chantier139–141, sans changement de code ; publication/CI finale/fusion/main
à terminer au commit de clôture. Le raccordement à l'ingestion reste ultérieur.
