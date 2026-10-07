# Configuration — contrat initial M4

Lot129 : `internal/config` définit un socle de paramètres et une validation pure.
Il ne charge aucun fichier YAML et n'est pas encore raccordé à la CLI ou à un
serveur. Les noms ci-dessous sont les noms réservés pour le futur chargement YAML.

## Valeurs par défaut et bornes

| Champ | Défaut | Règle initiale |
| --- | --- | --- |
| `server.listen` | `127.0.0.1:8080` | IP loopback littérale IPv4/IPv6, port numérique de1 à65535 |
| `server.read_header_timeout` | `5s` | De `1ms` à `1m`, inclus |
| `server.idle_timeout` | `1m` | De `1ms` à `10m`, inclus |
| `server.shutdown_timeout` | `15s` | De `1ms` à `1m`, inclus |
| `storage.path` | `queueatlas.db` | Chemin de fichier selon l'OS, absolu ou relatif |

`Defaults()` renvoie une valeur indépendante par appel. Le futur chargeur partira
de ces valeurs et remplacera seulement les champs présents ; un zéro explicite
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

Le futur chargeur devra résoudre un chemin relatif depuis le **répertoire du
fichier de configuration**, puis transmettre un chemin absolu au stockage.
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
Publication et CI129 à vérifier après commit.

Prochain lot130 : chargement YAML strict et borné, champs inconnus/doublons,
défauts pour les seuls champs absents, durées textuelles et résolution des chemins.
Les sources, CIDR/domaines, rétention et paramètres d'authentification demanderont
des contrats séparés selon les composants raccordés. `check-config`, `serve`,
doctor/db stats, auth/API/Web restent à développer. AD/OIDC après MVP, MIT conservée.
