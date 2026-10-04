# QueueAtlas

QueueAtlas est un projet de suivi de messages pour les infrastructures Postfix. Il vise à reconstituer, à partir des journaux, les étapes observées d'un message et les résultats par destinataire. L'installation cible est un exécutable Go autonome avec une base SQLite locale et une interface Web intégrée.

**État : parseurs Postfix, stockage SQLite, FileSource et récupération explicite du courant unique fusionnés dans `main`.** Le jalon M2 reste en cours : départ initial optionnel en fin de fichier et import historique à compléter. Aucun service, interface Web ou paquet installable n'est encore livré. Voir [l'avancement et les lots restants](docs/avancement.md) et le [point de reprise](docs/reprise.md). Le nom QueueAtlas et la licence MIT ont été confirmés par le propriétaire le 3 octobre 2026. Le dépôt conserve actuellement son URL historique `Coubiac/mailtrace`.

La [proposition de phase 0](docs/phase-0-proposal.md) décrit l'architecture, les limites de corrélation, le suivi des fichiers, la sécurité, le packaging et les premières étapes de développement. La feuille de route prévoit plus tard l'authentification Active Directory et OpenID Connect, notamment avec Keycloak.
