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
- Premier lot FileSource : lecteur de lignes borné dans
  `internal/source/file/reader.go`, commit
  `dfacfa20575529b421d7a6f0dedef8e7f476ddcc`.
  [PR #11](https://github.com/Coubiac/mailtrace/pull/11), branche
  `codex/m2-file-source`, empilée sur la PR #10.
- Les trois PR sont en brouillon. La PR #10 cible la branche de la PR #9.
  Aucune fusion n'a été effectuée.
- Validation du lecteur : `go test ./...` et `go vet ./...` réussis localement.
  [CI du commit de code](https://github.com/Coubiac/mailtrace/actions/runs/37142141635)
  réussie, incluant les builds Linux amd64/arm64 sans CGO.
- AD et fournisseur OIDC externe, dont Keycloak :
  [issue #8](https://github.com/Coubiac/mailtrace/issues/8) et ADR-008.

Le suivi des fichiers, l'import, la corrélation, l'authentification locale,
l'interface et les paquets installables restent à développer. Le pilote SQLite
est épinglé à v1.60.1 ; mesures de charge et inventaire complet des notices de
dépendances restent à faire avant distribution. La revue sécurité indépendante
des PR reste à mener.

## Dernier lot terminé : lecteur de lignes borné

`LineReader.Next(ctx)` renvoie une ligne complète avec son séparateur LF/CRLF et
ses offsets physiques. Une ligne partielle à EOF reste en mémoire jusqu'à une
lecture suivante ; une ligne trop longue conserve au maximum 64 Kio, consomme
le suffixe et produit une erreur dans `source.Record` après son séparateur.

Les tests couvrent les offsets non nuls, le seuil exact de taille, EOF répété
puis ajout de données, la ligne trop longue partielle, l'annulation, une erreur
de lecture et le débordement d'offset. Les records retournés restent stables
après les lectures suivantes. Aucun changement au schéma ou aux dépendances.

Limites : le lecteur ne remplit pas encore OriginID, ReadAt et Observation ;
il ne suit pas les chemins, ne détecte pas les rotations et ne persiste aucun
checkpoint. Une lecture bloquante reste à interrompre par son propriétaire.

## Prochain petit lot : identité physique des fichiers

Reprendre sur `codex/m2-file-source` et réutiliser la PR #11. Consulter l'issue #4
et ADR-003. Ajouter une petite abstraction d'identité du fichier ouvert :

- Identifier les fichiers réguliers et exposer device/inode sur Linux sans CGO.
- Distinguer un renommage d'un remplacement à l'aide de l'identité physique.
- Fournir une empreinte d'une fenêtre de début bornée, avec sa longueur, afin
  de pouvoir comparer la même fenêtre après ajout de données.
- Tester renommage, remplacement et stabilité de la fenêtre après append.

La sélection d'une génération persistée et la recherche des checkpoints restent
un lot distinct ; ne pas ajouter encore la boucle de suivi ou les rotations.

## Suite à découper au fil des reprises

1. Sélection d'une génération de fichier et recherche des checkpoints.
2. Suivi d'un fichier actif et acquittement par le Sink.
3. Rotation par renommage/création et écritures tardives.
4. Reprise après arrêt, troncature et diagnostic des lacunes.
5. Import historique normal, puis gzip dans un lot distinct.

Chaque demande de continuation traite par défaut un seul petit lot et actualise
ce point de reprise avec le résultat et la prochaine action.
