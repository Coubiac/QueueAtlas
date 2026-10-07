# Ouverture SQLite pour les diagnostics — lot133

Résultat attendu : ouvrir une base QueueAtlas existante en lecture seule, sans
création ni migration, avant d'implémenter les statistiques et leur commande CLI.

## Contrat de bibliothèque

`sqlite.OpenDiagnostics(ctx, path)` renvoie un `*Diagnostics` distinct du `Store`
applicatif. Le handle expose seulement `Close` à ce stade ; aucune méthode SQL
libre, ingestion, purge ou migration. Les futures lectures seront ajoutées dans
les lots suivants. `Open` reste l'ouverture applicative avec création/migrations.

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
Les diagnostics détaillés et les statistiques restent à développer.

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
