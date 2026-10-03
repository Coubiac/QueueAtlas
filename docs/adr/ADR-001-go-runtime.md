# ADR-001 — Runtime Go autonome

Statut : accepté le 3 octobre 2026.

## Contexte
Le produit doit être installable comme un exécutable unique sous Linux amd64/arm64, sans service runtime externe.
## Options
Go avec bibliothèque standard dominante ; runtime applicatif nécessitant un serveur Web ou une VM séparée.
## Décision
Go, binaire servant API et Web, builds officiels `CGO_ENABLED=0`.
## Raisons
Déploiement simple, concurrence et outillage de tests adaptés à la lecture continue des logs.
## Conséquences
Toutes les dépendances et assets doivent être intégrables dans ces builds ; vérifier les deux architectures en CI.
## Limites
Taille et performance du binaire à mesurer ; Windows et macOS restent hors MVP.
