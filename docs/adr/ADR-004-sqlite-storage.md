# ADR-004 — Stockage SQLite

Statut : accepté le 3 octobre 2026 pour SQLite ; pilote `modernc.org/sqlite` v1.60.1 retenu pour l'implémentation initiale, à réévaluer sur des mesures de charge avant le MVP.

## Contexte
Recherche indexée et checkpoints doivent fonctionner sans serveur SQL ni CGO.
## Options
`modernc.org/sqlite`, `ncruces/go-sqlite3`, pilote CGO ou base externe.
## Décision
Utiliser `modernc.org/sqlite` v1.60.1 pour la migration v1 et le Sink initial. SQLite en WAL sur disque local, un écrivain, `synchronous=FULL`, FK et `trusted_schema=OFF` par connexion. Le fichier neuf est créé avec des droits propriétaire seul ; un fichier existant accessible au groupe ou aux autres est refusé sur Unix. Les valeurs SQL sont paramétrées.

Le fichier principal et les sidecars existants (`-wal`, `-shm`, `-journal`) doivent
être réguliers ; leurs droits Unix doivent être privés avant connexion. SQLite
n'est pas supposé réduire les droits d'un WAL préexistant. Le parent doit rester
protégé contre les écritures non autorisées ; ces observations ne sont pas des
verrous contre remplacement concurrent. Les ACL Windows ne sont pas validées par
ce contrôle de bits Unix. Un contexte déjà annulé ne crée pas de fichier.
## Raisons
Compatibilité avec Go 1.26, Go pur et `database/sql`, recherche et transactions locales. Le [module publié](https://pkg.go.dev/modernc.org/sqlite@v1.60.1) est sous licence BSD-3-Clause ; cette licence est compatible avec la licence MIT du projet. Les dépendances directes et transitives restent figées dans `go.mod` et `go.sum`.
## Conséquences
Migration v1 atomique et refus des versions futures ou des bases étrangères non versionnées. La provenance `(source, génération de fichier, offset initial)` rend l'insertion idempotente ; le checkpoint n'avance qu'après l'insertion de l'événement dans la même transaction. Les projections de corrélation restent recalculables. Sauvegarde cohérente en WAL et rétention par parcours terminé.

Les versions négatives sont refusées avant WAL et dans la migration. Une base
non versionnée est vide uniquement si elle n'a aucun objet utilisateur : tables,
vues, index et triggers comptent, avec préfixe interne littéral `sqlite_` exclu.

La projection `events.time_utc_ns` utilise un entier signé de nanosecondes. Un
horodatage hors de cette plage (par exemple une année RFC5424 9999) conserve sa
date brute, année, zone et qualité, mais sa projection UTC est NULL. Un aller-retour
exact de l'entier est requis ; aucun instant débordé n'est indexé. L'observation,
sa provenance et son checkpoint restent acquittés ensemble. Les dates ReadAt et
FirstSeen sont fournies par les sources internes, qui doivent donner des instants
actuels valides ; elles ne proviennent pas de l'horodatage déclaré du journal.
## Limites
Débit d'ingestion, taille du binaire et inventaire complet des licences transitives à mesurer avant la première distribution ; WAL sur réseau exclu ; FULL peut coûter du débit. Le schéma v1 ne doit plus être modifié après une release publique : toute évolution ultérieure passera par une migration v2. Les tables de projection existent mais seront alimentées par le moteur de corrélation au jalon M3.

## Évolution du 4 octobre 2026

La migration v2 ajoute l'état durable du suivi des générations (ADR-009), en
conservant le schéma v1 intact et les anciennes générations à l'état inconnu.
Les migrations, leur historique et `user_version` sont acquittés dans une même
transaction ; l'ouverture contrôle désormais les entrées d'historique v1 et v2.
