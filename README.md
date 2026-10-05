# QueueAtlas

QueueAtlas est un projet de suivi de messages pour les infrastructures Postfix. Il vise à reconstituer, à partir des journaux, les étapes observées d'un message et les résultats par destinataire. L'installation cible est un exécutable Go autonome avec une base SQLite locale et une interface Web intégrée.

**État : parseurs Postfix, stockage SQLite, FileSource et import historique normal/gzip implémentés en bibliothèque Go.** L'import valide le contenu entier dans une copie privée, applique observations/checkpoints/manifest atomiquement et reprend une liste ordonnée sous limites de taille, ratio, fichiers et durée. Le jalon M2 reste en cours pour les compléments et diagnostics. Aucun service, interface Web ou paquet installable n'est encore livré. Voir [l'avancement et les lots restants](docs/avancement.md) et le [point de reprise](docs/reprise.md) pour les PR, CI et limites vérifiées. Le nom QueueAtlas et la licence MIT ont été confirmés par le propriétaire le 3 octobre 2026. Le dépôt conserve actuellement son URL historique `Coubiac/mailtrace`.

La [proposition de phase 0](docs/phase-0-proposal.md) décrit l'architecture, les limites de corrélation, le suivi des fichiers, la sécurité, le packaging et les premières étapes de développement. La feuille de route prévoit plus tard l'authentification Active Directory et OpenID Connect, notamment avec Keycloak.

Le [contrat d'exploitation de l'ingestion](docs/ingestion-contract.md) récapitule les conditions de reprise, les limites des diagnostics et l'incertitude des chevauchements entre sources.
