# QueueAtlas

QueueAtlas est un projet de suivi de messages pour les infrastructures Postfix. Il vise à reconstituer, à partir des journaux, les étapes observées d'un message et les résultats par destinataire. L'installation cible est un exécutable Go autonome avec une base SQLite locale et une interface Web intégrée.

**État : M3 clôturé en bibliothèque ; CLI `version` et `check-config` fusionnée, M4 en cours.** Parseurs Postfix, SQLite, FileSource et import normal/gzip sont implémentés en bibliothèque Go. L'import valide le contenu entier dans une copie privée, applique observations/checkpoints/manifest atomiquement et reprend une liste ordonnée sous limites de taille, ratio, fichiers et durée. Les diagnostics et les limites de chevauchement sont documentés et testés.

La reconstruction conserve les tentatives par destinataire, expirations explicites, réserves et rapports NOQUEUE dans des projections candidates. Indices et relations de file (#22), révisions des clés/résumés/liens (#23), puis stockage des révisions [#24](https://github.com/Coubiac/QueueAtlas/pull/24) sont fusionnés avec CI finale réussie. Le stockage lit les faits, installe les manifests atomiquement et reconstruit les résultats avec fraîcheur vérifiée au snapshot, sans cache sérialisé. La CI push main de #24 est réussie. Recherche exacte par adresse, Queue ID/Message-ID et domaines ASCII fusionnée dans #25 (lots111–115, CI finale/main vertes), avec enchaînement des API de bibliothèque vers une reconstruction de périmètre complet testé. Le lot115 mesure recherche/migration localement et resserre la borne de pagination ; [protocole et limites](docs/search-measurements.md). Rétention116–119 fusionnée dans [#27](https://github.com/Coubiac/QueueAtlas/pull/27), CI finale/main vertes : aperçu readonly, garde persisté de rejeu, purge transactionnelle et intégration WAL/recherche/reconstruction/import. Le lot120 ajoute le [contrat de cohérence des attestations de continuité](docs/continuity-contract.md), publié dans [#28](https://github.com/Coubiac/QueueAtlas/pull/28) avec CI entière réussie. Le lot121 lie les clés candidates au contexte d'attestation, publié dans la même PR avec cinq nouveaux tests et CI entière réussie. Le [bilan120–122](docs/reviews/continuity.md) est fusionné dans #28, CI finale/main vertes. La [matrice de sortie M3](docs/m3-exit-checklist.md) consigne le contrôle124 : conflit à date égale conservé après persistance/reopen et insertion inverse, publié dans #29 avec CI entière réussie. Les [mesures locales des projections125–126](docs/projection-measurements.md) sont consignées sur profils synthétiques bornés ; la [clôture M3](docs/reviews/m3-exit.md) est vérifiée, #29 fusionnée et CI finale/main réussies. Production fiable des attestations et raccordement applicatif restent à définir ; aucune génération n'est fusionnée par ces deux lots.

Aucun service, interface Web ou paquet installable n'est encore livré. Voir [l'avancement et les lots restants](docs/avancement.md) et le [point de reprise](docs/reprise.md) pour les PR, CI et limites vérifiées. Le nom QueueAtlas et la licence MIT ont été confirmés par le propriétaire le 3 octobre 2026. Le dépôt GitHub est [Coubiac/QueueAtlas](https://github.com/Coubiac/QueueAtlas).

La [proposition de phase 0](docs/phase-0-proposal.md) décrit l'architecture, les limites de corrélation, le suivi des fichiers, la sécurité, le packaging et les premières étapes de développement. La feuille de route prévoit plus tard l'authentification Active Directory et OpenID Connect, notamment avec Keycloak.

Le [contrat d'exploitation de l'ingestion](docs/ingestion-contract.md) récapitule les conditions de reprise, les limites des diagnostics et l'incertitude des chevauchements entre sources.

Le [contrat de reconstruction](docs/correlation-contract.md) décrit les résultats observés et les conclusions encore interdites.

Le [contrat de stockage des révisions](docs/projection-storage-contract.md) décrit les transactions, références complètes et limites de fraîcheur des manifests.

Le [contrat de recherche](docs/search-contract.md) décrit les critères exacts, dates inconnues et limites de pagination.

## Premier point d'entrée CLI

Depuis les sources du chantier M4 :

```powershell
go run ./cmd/queueatlas version
go run ./cmd/queueatlas check-config --config examples/queueatlas.yaml
```

Affiche `QueueAtlas dev` pour un build ordinaire. Voir [commandes, compilation et
codes de sortie](docs/m4-cli.md). Le [contrat initial de configuration](docs/configuration.md)
fixe les défauts et leur validation (lot129), avec un chargeur YAML strict et borné
(lot130) et un [exemple](examples/queueatlas.yaml). `check-config` valide ce fichier
et affiche `Configuration valid` sans ouvrir de base (lot131). La consultation
authentifiée reste à développer. Les lots133–134 préparent les [diagnostics SQLite](docs/sqlite-diagnostics.md)
en lecture seule et leurs métadonnées ; aucune commande `db stats`/`doctor` n'est encore disponible.
