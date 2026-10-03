# ADR-007 — Distribution Linux

Statut : accepté le 3 octobre 2026.

## Contexte
Le MVP doit s'installer sur Debian/Ubuntu et RHEL/Fedora avec peu d'étapes.
## Options
Archive seule ; archive + DEB/RPM et service systemd ; conteneur seulement.
## Décision
Archives statiques amd64/arm64, DEB/RPM avec utilisateur dédié et unité systemd ; image Docker complémentaire. Évaluer GoReleaser/nFPM.
## Raisons
Installation et mise à jour idiomatiques sur les deux familles Linux sans imposer Docker.
## Conséquences
Préserver la configuration, tester lecture des logs après rotation, DB locale, checksums, SBOM et chaîne CI/release cloisonnée.
## Limites
Permissions et emplacement des journaux varient selon distribution ; tester les paquets réels avant release.
