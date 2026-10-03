# ADR-004 — Stockage SQLite

Statut : accepté le 3 octobre 2026 pour SQLite ; pilote `modernc.org/sqlite` v1.60.1 retenu pour l'implémentation initiale, à réévaluer sur des mesures de charge avant le MVP.

## Contexte
Recherche indexée et checkpoints doivent fonctionner sans serveur SQL ni CGO.
## Options
`modernc.org/sqlite`, `ncruces/go-sqlite3`, pilote CGO ou base externe.
## Décision
Utiliser `modernc.org/sqlite` v1.60.1 pour la migration v1 et le Sink initial. SQLite en WAL sur disque local, un écrivain, `synchronous=FULL`, FK et `trusted_schema=OFF` par connexion. Le fichier neuf est créé avec des droits propriétaire seul ; un fichier existant accessible au groupe ou aux autres est refusé sur Unix. Les valeurs SQL sont paramétrées.
## Raisons
Compatibilité avec Go 1.26, Go pur et `database/sql`, recherche et transactions locales. Le [module publié](https://pkg.go.dev/modernc.org/sqlite@v1.60.1) est sous licence BSD-3-Clause ; cette licence est compatible avec la licence MIT du projet. Les dépendances directes et transitives restent figées dans `go.mod` et `go.sum`.
## Conséquences
Migration v1 atomique et refus des versions futures ou des bases étrangères non versionnées. La provenance `(source, génération de fichier, offset initial)` rend l'insertion idempotente ; le checkpoint n'avance qu'après l'insertion de l'événement dans la même transaction. Les projections de corrélation restent recalculables. Sauvegarde cohérente en WAL et rétention par parcours terminé.
## Limites
Débit d'ingestion, taille du binaire et inventaire complet des licences transitives à mesurer avant la première distribution ; WAL sur réseau exclu ; FULL peut coûter du débit. Le schéma v1 ne doit plus être modifié après une release publique : toute évolution ultérieure passera par une migration v2. Les tables de projection existent mais seront alimentées par le moteur de corrélation au jalon M3.
