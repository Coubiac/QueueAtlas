# QueueAtlas

QueueAtlas est un projet de suivi de messages pour les infrastructures Postfix. Il vise à reconstituer, à partir des journaux, les étapes observées d'un message et les résultats par destinataire. L'installation cible est un exécutable Go autonome avec une base SQLite locale et une interface Web intégrée.

**État : socle d'ingestion M2 et reconstruction M3 fusionnés (#18–24) ; M3 en cours.** Parseurs Postfix, SQLite, FileSource et import normal/gzip sont implémentés en bibliothèque Go. L'import valide le contenu entier dans une copie privée, applique observations/checkpoints/manifest atomiquement et reprend une liste ordonnée sous limites de taille, ratio, fichiers et durée. Les diagnostics et les limites de chevauchement sont documentés et testés.

La reconstruction conserve les tentatives par destinataire, expirations explicites, réserves et rapports NOQUEUE dans des projections candidates. Indices et relations de file (#22), révisions des clés/résumés/liens (#23), puis stockage des révisions [#24](https://github.com/Coubiac/mailtrace/pull/24) sont fusionnés avec CI finale réussie. Le stockage lit les faits, installe les manifests atomiquement et reconstruit les résultats avec fraîcheur vérifiée au snapshot, sans cache sérialisé. La CI push main de #24 est réussie. Recherche exacte par adresse publiée au lot111 ; Queue ID/Message-ID étendus au lot112 sur la branche de travail. Domaines, intégration, rétention et continuité restent à réaliser.

Aucun service, interface Web ou paquet installable n'est encore livré. Voir [l'avancement et les lots restants](docs/avancement.md) et le [point de reprise](docs/reprise.md) pour les PR, CI et limites vérifiées. Le nom QueueAtlas et la licence MIT ont été confirmés par le propriétaire le 3 octobre 2026. Le dépôt conserve actuellement son URL historique `Coubiac/mailtrace`.

La [proposition de phase 0](docs/phase-0-proposal.md) décrit l'architecture, les limites de corrélation, le suivi des fichiers, la sécurité, le packaging et les premières étapes de développement. La feuille de route prévoit plus tard l'authentification Active Directory et OpenID Connect, notamment avec Keycloak.

Le [contrat d'exploitation de l'ingestion](docs/ingestion-contract.md) récapitule les conditions de reprise, les limites des diagnostics et l'incertitude des chevauchements entre sources.

Le [contrat de reconstruction](docs/correlation-contract.md) décrit les résultats observés et les conclusions encore interdites.

Le [contrat de stockage des révisions](docs/projection-storage-contract.md) décrit les transactions, références complètes et limites de fraîcheur des manifests.

Le [contrat de recherche](docs/search-contract.md) décrit les critères exacts, dates inconnues et limites de pagination.
