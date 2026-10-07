# SQLite pour les diagnostics — lots133–135

Résultat attendu : ouvrir une base QueueAtlas existante en lecture seule, sans
création ni migration, avant d'implémenter les statistiques et leur commande CLI.

## Contrat de bibliothèque

`sqlite.OpenDiagnostics(ctx, path)` renvoie un `*Diagnostics` distinct du `Store`
applicatif. Le handle expose `Close` et, depuis134, `Metadata` ; aucune méthode SQL
libre, ingestion, purge ou migration. `Open` reste l'ouverture applicative avec
création/migrations.

Le chemin doit désigner un fichier régulier existant ; une entrée vide, absente
ou non régulière est refusée. Les fichiers auxiliaires existants `-wal`, `-shm`
et `-journal` doivent aussi être réguliers. Sur Unix, base et auxiliaires doivent
être inaccessibles au groupe/aux autres, comme pour `Open`. Aucun chmod, création
de parent ou ACL Windows n'est effectué/attesté. L'appelant doit protéger le
répertoire parent et utiliser un disque local ; les observations de métadonnées
ne verrouillent pas les chemins contre leur remplacement. Les liens symboliques
sont suivis ; aucun confinement de chemin ou contrôle de montage n'est revendiqué.

La connexion utilise une URI construite/échappée avec `mode=ro`, `query_only=ON`,
`foreign_keys=ON`, `trusted_schema=OFF`, mode défensif et DQS désactivé. Une seule
connexion physique à la fois ; délai SQLite busy de 5 secondes. Les paramètres
sont réappliqués aux connexions de remplacement. Aucune affectation de
`journal_mode`, migration, checkpoint applicatif ni `immutable=1`/`nolock=1`.
Un contexte annulé est transmis ; le délai busy n'est pas une borne de temps pour
toutes les opérations sur le système de fichiers.

Version et historique sont lus dans une seule transaction : `user_version` doit
être la version courante (7), `schema_migrations` une table, et les entrées1–7
présentes. Une version antérieure, future, zéro ou un historique incomplet sont
refusés. Ce contrôle ne certifie ni authenticité, structure complète, intégrité
des lignes, fraîcheur des projections ou aptitude au déploiement. Un fichier
étranger falsifiant ces indicateurs n'est pas identifié par cette seule ouverture.
Les diagnostics détaillés et les compteurs de données restent à développer.

`ErrDiagnosticsOpen` et `ErrDiagnosticsSchema` ont des messages fixes sans chemin,
valeur stockée ou message du pilote ; annulation/expiration du contexte restent
identifiables. Les erreurs ne donnent pas encore un diagnostic CLI détaillé.

## WAL et effets sur les fichiers

`mode=ro` interdit les écritures dans la base, même si `query_only` est désactivé.
Les transactions validées présentes dans le WAL restent visibles ; une nouvelle
lecture peut voir les commits suivants. L'ouverture ne conserve pas un snapshot
de données pour toute la durée du handle.

SQLite peut utiliser/créer les auxiliaires WAL et sa mémoire partagée pour les
verrous/index, y compris avec une base ouverte en lecture seule. L'absence totale
d'écriture/création auxiliaire n'est donc pas garantie ; un répertoire non
inscriptible peut nécessiter des auxiliaires déjà lisibles. Une base avec journal
de récupération non résolu peut être refusée. Ne pas supprimer/copier le WAL
séparément pour contourner un refus. Cette API n'est pas une sauvegarde.

Références primaires consultées : [URI SQLite](https://www.sqlite.org/uri.html)
pour les modes et l'immuabilité, [WAL SQLite](https://www.sqlite.org/wal.html#read_only_databases)
pour les auxiliaires et la concurrence, [pilote modernc v1.60.1](https://pkg.go.dev/modernc.org/sqlite@v1.60.1)
pour les options de connexion. Le choix sans `immutable` conserve détection des
changements et verrous pendant une ingestion active.

## Vérifications et suite

Cinq tests ciblés passés sous Windows : refus sans création/annulation et
auxiliaires non réguliers ; base rollback inchangée octet pour octet et sans
auxiliaire ajouté ; refus des versions/historiques/fichiers étrangers sans
migration ; WAL validé visible, transaction non validée invisible, commits suivants
visibles, protections réappliquées après reconnexion, écritures refusées même sans
query_only ; chemin Unicode/espaces/caractères URI traité littéralement.
Vet ciblé, format et diff vérifiés. Test Linux supplémentaire pour FIFO et droits
Unix ajouté ; son exécution relève de la CI. Une étape Windows dédiée couvre ces
ouvertures ; les jobs Linux existants exécutent la suite complète.

Au moment du commit133 : publication/PR/CI encore à terminer. Prochain lot134 :
lecture bornée des métadonnées utiles au diagnostic, avec résultat sans journaux,
adresses ni identifiants. Raccordement CLI dans un lot séparé.

Validation publiée133 : `41fbf0f99a4b127e260dea0a5ce900c9d06855c8` dans
[PR #31](https://github.com/Coubiac/QueueAtlas/pull/31),
[CI37571495723](https://github.com/Coubiac/QueueAtlas/actions/runs/37571495723)
entière réussie, trois jobs/SHA exact vérifiés ; tests Linux FIFO/droits et étape
Windows d'ouverture passés. La mention d'attente précédente est le snapshot133.

## Lot134 : métadonnées au même snapshot

`Diagnostics.Metadata(ctx)` renvoie un seul `DiagnosticMetadata`, sans chemin,
nom de table, identifiant, adresse ou journal. Champs fixes :

| Champ | Sens |
| --- | --- |
| SchemaVersion | Version QueueAtlas courante, revérifiée avec l'historique |
| SQLiteVersion | Version du moteur SQLite exécuté, chaîne bornée à64octets |
| JournalMode | Mode de journal observé par la connexion, valeur SQLite connue |
| PageSize | Taille d'une page en octets, puissance de deux512–65536 |
| PageCount | Nombre de pages de l'image logique visible au snapshot |
| FreePageCount | Pages de la freelist SQLite, entre0 et PageCount |

Version/historique et métadonnées sont lus dans une seule transaction readonly.
La compatibilité est revérifiée à chaque appel : l'ouverture ne certifie pas la
version pour toutes les lectures suivantes. Les pages validées dans le WAL font
partie de l'image logique. Le produit PageCount × PageSize n'est donc pas une
mesure du seul fichier principal, de l'espace libre du disque ou des fichiers
auxiliaires. FreePageCount ne compte ni journaux purgés ni lignes supprimées.
Le mode observé n'atteste pas tous les réglages d'une autre connexion d'écriture.

Requêtes fixes, une ligne scalaire chacune, sans parcours/count des tables de
journaux, liste d'objets, lecture de chemin SQLite, scan d'intégrité, checkpoint,
vacuum ou migration. L'historique reste une lecture des sept entrées attendues.
Le résultat est borné, sans garantie de temps absolu d'IO ou de coût d'un fichier
SQLite hostile. Les [PRAGMA SQLite](https://www.sqlite.org/pragma.html) définissent
les pages et freelist ; [sqlite_version](https://www.sqlite.org/lang_corefunc.html#sqlite_version)
décrit la version du moteur, distincte du schéma applicatif.

Toute erreur renvoie le résultat zéro, aucune donnée partielle. Une incompatibilité
donne ErrDiagnosticsSchema ; une autre erreur de lecture donne ErrDiagnosticsRead
avec message fixe sans pilote/contenu. Annulation et expiration du contexte restent
identifiables. Une lecture échouée libère sa transaction/connexion ; les lectures
suivantes peuvent réussir si la cause a disparu. Ce diagnostic n'atteste pas
intégrité, fraîcheur des projections, permission de sauvegarder ni état de service.

Quatre tests134 passés sous Windows : image rollback comparée à son fichier/header,
pages libres et absence de contenu sensible/mutation ; ancien snapshot conservé
pendant croissance WAL/changement atomique de version, refus de la nouvelle version
puis lecture de croissance après rétablissement, sans checkpoint/mutation ; historique
supprimé après ouverture détecté ; annulation/délai en attente de connexion puis
relecture et handle fermé, résultat zéro/messages fixes. Les neuf tests portables
133–134 et vet/format/diff passent ; l'étape Windows CI couvre aussi Metadata.
Au moment du commit134 : publication/CI encore à terminer dans la même PR #31.
Prochain lot135 : CLI `db stats --config <chemin>` pour ce résultat limité ; aucun
compteur de lignes ou diagnostic d'intégrité implicite.

Validation publiée134 : `8e9008b90d48eaff0f90b53cb4718e978521b223`,
[CI37572091851](https://github.com/Coubiac/QueueAtlas/actions/runs/37572091851)
entière réussie, trois jobs/SHA exact et étape Windows diagnostics vérifiés.
Les attentes précédentes sont le snapshot avant publication134.

## Raccordement CLI135

[db stats --config](m4-cli.md#lot135--db-stats) imprime seulement les six champs
en JSON, avec des noms stables distincts du struct de bibliothèque ; l'ajout d'un
champ de bibliothèque n'étend donc pas automatiquement la sortie. Chargeur existant,
OpenDiagnostics puis Metadata, fermeture avant sérialisation/sortie. Config invalide
code2, diagnostic DB échoué code1/fixe/stdout vide, succès code0. Contexte DB
coopératif10secondes ; pas de deadline dure IO. Aucun changement de stockage134,
compteur de lignes, migration/checkpoint ni composant réseau. Les auxiliaires
SQLite restent possibles. Tests réels du binaire et vérifications CLI Windows
passés ; publication/CI135 encore à terminer dans #31 au moment du commit.

Validation effective135 : cae194c publié,
[CI37574528679](https://github.com/Coubiac/QueueAtlas/actions/runs/37574528679)
entière réussie, trois jobs/SHA exact revérifiés à la reprise136.
[Relecture136](reviews/m4-diagnostics.md) favorable ; #31 fusionnée sur918ef0c,
CI finale37576814504/main37576942301 entières réussies, trois jobs/SHA exact
vérifiés. La clôture de ce chantier ne clôture
pas M4 et ne rend pas l'application installable/authentifiée.

## Raccordement doctor137

[doctor --config](m4-cli.md#lot137--doctor) utilise uniquement OpenDiagnostics,
puis ferme la connexion avant de publier un statut de compatibilité. Il n'appelle
pas Metadata, ne crée/migre aucune base et ne démarre pas de composant.
Les limites de version/historique, droits, chemins et auxiliaires restent celles
de ce lecteur ; aucune attestation d'intégrité/déploiement. Stockage inchangé.
