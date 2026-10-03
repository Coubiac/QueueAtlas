# Point de reprise QueueAtlas

Mis à jour le 3 octobre 2026. Ce fichier décrit le dernier état connu ; vérifier
Git et GitHub avant de modifier une branche ou de fusionner une PR.

## État validé

- Nom QueueAtlas, licence MIT et conception de phase 0 approuvés.
- M1 : enveloppes syslog, parseurs Postfix, corpus synthétique et tests/fuzz.
  [PR #9](https://github.com/Coubiac/mailtrace/pull/9), branche
  `m1-parser-foundation`, commit `932bef1e20c3573b243d1ddbdfd6ec42e4fdb4d6`.
- Socle M2 : migration SQLite v1, contrat `Source`/`Sink`, insertion idempotente
  des observations et commit atomique des checkpoints.
  [PR #10](https://github.com/Coubiac/mailtrace/pull/10), branche
  `codex/m2-sqlite-storage`, dernier commit de code
  `34478635265c8001b4f0d2e8f42485b380f5060f`.
- Les deux PR sont en brouillon. La PR #10 cible la branche de la PR #9.
  Aucune fusion n'a été effectuée.
- Dernière validation du code : `go test ./...`, `go vet ./...` et CI GitHub
  réussis ; builds Linux amd64/arm64 avec `CGO_ENABLED=0` réussis.
- AD et fournisseur OIDC externe, dont Keycloak :
  [issue #8](https://github.com/Coubiac/mailtrace/issues/8) et ADR-008.

Le suivi des fichiers, l'import, la corrélation, l'authentification locale,
l'interface et les paquets installables restent à développer. Le pilote SQLite
est épinglé à v1.60.1 ; mesures de charge et inventaire complet des notices de
dépendances restent à faire avant distribution. La revue sécurité indépendante
des PR reste à mener.

## Prochain petit lot : lecteur de lignes borné

Objectif : un lecteur réutilisable pour FileSource, sans dépendance SQLite,
capable de produire une ligne complète avec ses offsets physiques.

Acceptation :

- Une ligne complète avance jusqu'à l'octet suivant son séparateur.
- Une ligne partielle en fin de fichier attend la suite sans être validée.
- Une ligne dépassant `model.MaxLineBytes` est consommée avec mémoire bornée,
  puis signalée explicitement ; la ligne suivante reste lisible.
- Tests ciblés : plusieurs lignes, CRLF, fin partielle puis ajout de données,
  ligne trop longue et offsets après celle-ci.

Partir du code de la PR #10 dans une branche `codex/` consacrée à FileSource.
Réutiliser `internal/source/source.go` ; consulter l'issue #4 et ADR-003 avant de
fixer l'interface du lecteur. Les rotations et la reprise durable feront l'objet
de lots suivants.

## Suite à découper au fil des reprises

1. Identité de génération de fichier et recherche des checkpoints.
2. Suivi d'un fichier actif et acquittement par le Sink.
3. Rotation par renommage/création et écritures tardives.
4. Reprise après arrêt, troncature et diagnostic des lacunes.
5. Import historique normal, puis gzip dans un lot distinct.

Chaque demande de continuation traite par défaut un seul petit lot et actualise
ce point de reprise avec le résultat et la prochaine action.
