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

Ce contrat est indépendant de `Config` : **le chargeur YAML n'accepte pas encore
de section source**. Le lot140 ajoutera son chargement strict/borné ; résolution
des chemins et raccordement FileSource seront vérifiés aux étapes correspondantes.
Les commandes actuelles conservent leur comportement. Aucun composant n'est lancé
par ce lot.

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
