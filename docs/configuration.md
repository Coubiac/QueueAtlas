# Configuration — contrat initial M4

Lots129–130 : `internal/config` définit un socle de paramètres, une validation pure
et le chargement YAML. Le package n'est pas encore raccordé à la CLI ou à un
serveur. Un [fichier d'exemple](../examples/queueatlas.yaml) est chargé dans les tests.

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
publication/CI130 à vérifier après commit.

Prochain lot131 : commande CLI `check-config --config <chemin>`, flux/codes de sortie
et tests du binaire, sans démarrage de composant. `serve` reste à développer.
Les sources, CIDR/domaines, rétention et paramètres d'authentification demanderont
des contrats séparés selon les composants raccordés. `check-config`, `serve`,
doctor/db stats, auth/API/Web restent à développer. AD/OIDC après MVP, MIT conservée.
