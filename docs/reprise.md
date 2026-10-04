# Point de reprise QueueAtlas

Mis à jour le 4 octobre 2026. Ce fichier décrit le dernier état connu ; vérifier
Git et GitHub avant de modifier une branche ou de fusionner une PR.

## État validé

- Nom QueueAtlas, licence MIT et conception de phase 0 approuvés.
- M1 : enveloppes syslog, parseurs Postfix, corpus synthétique et tests/fuzz.
  [PR #9](https://github.com/Coubiac/mailtrace/pull/9), branche
  `m1-parser-foundation`, fusionnée dans main sur
  `b52e7fb78022049146d87eb69de002fc239e0f12` après revue et correctif.
- Socle M2 : migration SQLite v1, contrat `Source`/`Sink`, insertion idempotente
  des observations et commit atomique des checkpoints.
  [PR #10](https://github.com/Coubiac/mailtrace/pull/10), branche
  `codex/m2-sqlite-storage`, désormais basée sur main. Revue coordinateur et audit
  agent indépendant : bases étrangères, projection des dates hors plage, versions
  négatives et fichiers annexes corrigés. Rapport dans docs/reviews/pr-10.md.
  La CI de la tête publiée et le statut final de fusion sont consignés dans la PR.
- Validation locale du correctif : `go test ./...`, `go vet ./...`, `git diff --check`,
  builds Linux amd64/arm64 sans CGO et compilation des tests SQLite Linux réussis.
  Permissions Unix et FIFO confirmées par la
  [CI du correctif](https://github.com/Coubiac/mailtrace/actions/runs/37186524966)
  Go 1.26.x/stable sur `463d767418cb366b87aaf983530a42da6bba2f35`.
- Le chantier FileSource est dans la [PR #11](https://github.com/Coubiac/mailtrace/pull/11),
  avec un [point de reprise détaillé](https://github.com/Coubiac/mailtrace/blob/codex/m2-file-source/docs/reprise.md).
- AD et fournisseur OIDC externe, dont Keycloak :
  [issue #8](https://github.com/Coubiac/mailtrace/issues/8) et ADR-008.

FileSource est développé sur sa branche de chantier ; l'import, la corrélation,
l'authentification locale, l'interface et les paquets installables restent à développer.
Le pilote SQLite
est épinglé à v1.60.1 ; mesures de charge et inventaire complet des notices de
dépendances restent à faire avant distribution. La revue sécurité indépendante
de FileSource reste à mener. MIT et AD/OIDC après MVP conservés.

## Prochain petit lot : revue de FileSource

Après vérification de la fusion de #10, recadrer la revue de #11 en lots courts :
contrats/lecteur et décisions de reprise, puis rotation/lifecycle et intégration.
Réutiliser les tests et preuves du point de reprise détaillé du chantier. Auditer
les risques concrets et le schéma v2 sans attendre les fonctionnalités Web/M3.
Ne pas prétendre à une revue complète de FileSource à l'issue d'un premier lot.

La récupération explicite des lifecycle inconnus et l'import gzip sont des lots
distincts, après la revue du périmètre existant.

## Suite à découper au fil des reprises

1. Identité de génération de fichier et recherche des checkpoints.
2. Suivi d'un fichier actif et acquittement par le Sink.
3. Rotation par renommage/création et écritures tardives.
4. Reprise après arrêt, troncature et diagnostic des lacunes.
5. Import historique normal, puis gzip dans un lot distinct.

Chaque demande de continuation traite par défaut un seul petit lot et actualise
ce point de reprise avec le résultat et la prochaine action.
