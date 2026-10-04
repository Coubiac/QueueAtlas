# ADR-004 — Stockage SQLite

Statut : accepté le 3 octobre 2026 pour SQLite ; choix précis du pilote conditionné au benchmark et à l'inventaire des licences.

## Contexte
Recherche indexée et checkpoints doivent fonctionner sans serveur SQL ni CGO.
## Options
`modernc.org/sqlite`, `ncruces/go-sqlite3`, pilote CGO ou base externe.
## Décision
Évaluer `modernc.org/sqlite` en premier ; SQLite WAL sur disque local, un écrivain, `synchronous=FULL`, FK et `trusted_schema=OFF` par connexion.
## Raisons
Compatibilité annoncée avec Go pur et `database/sql`, recherche et transactions locales.
## Conséquences
Migrations versionnées, index mesurés, FK vérifiées, sauvegarde cohérente en WAL, rétention par parcours terminé.
## Limites
Débit, taille du binaire et dépendances à mesurer ; WAL sur réseau exclu ; FULL peut coûter du débit.
