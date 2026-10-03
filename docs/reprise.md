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
- Deuxième lot FileSource : identité physique et empreinte de début bornée dans
  `internal/source/file/identity*.go`, sur la même branche et dans la PR #11.
- Troisième lot FileSource : interface de lecture `source.StateReader` et
  implémentation SQLite de la recherche paginée des origines et checkpoints.
  Toujours dans la PR #11, sans migration ni dépendance ajoutée.
- Quatrième lot FileSource : ancres bornées de checkpoint et lecture stricte
  des formats persistés, dans `internal/source/file/anchor.go`.
- Cinquième lot FileSource : vérification d'un candidat de reprise dans
  `internal/source/file/resume.go`, commit
  `489c2dc5a2f067d7d7a1ca8271b32fb88f8a0ff4`, toujours dans la PR #11.
- Les trois PR sont en brouillon. La PR #10 cible la branche de la PR #9.
  Aucune fusion n'a été effectuée.
- Validation du lecteur : `go test ./...` et `go vet ./...` réussis localement.
  [CI du commit de code](https://github.com/Coubiac/mailtrace/actions/runs/37142141635)
  réussie, incluant les builds Linux amd64/arm64 sans CGO.
- Validation locale du lot identité : `go test ./...`, `go vet ./...` et builds
  Linux amd64/arm64 avec `CGO_ENABLED=0` réussis.
  [CI identité](https://github.com/Coubiac/mailtrace/actions/runs/37154817870)
  réussie, notamment les tests d'identité Linux.
- Validation locale du lot lecture d'état : `go test ./...` et `go vet ./...`
  réussis. [CI lecture d'état](https://github.com/Coubiac/mailtrace/actions/runs/37155184130)
  réussie, incluant les builds Linux sans CGO.
- Validation locale du lot ancres : `go test ./...` et `go vet ./...` réussis.
  [CI ancres](https://github.com/Coubiac/mailtrace/actions/runs/37155582363)
  réussie.
- Validation locale du lot candidat : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource pour Linux amd64 sans CGO réussis.
  [CI candidat](https://github.com/Coubiac/mailtrace/actions/runs/37156132714)
  réussie, incluant les cas device/inode sur Linux et les builds sans CGO.
- AD et fournisseur OIDC externe, dont Keycloak :
  [issue #8](https://github.com/Coubiac/mailtrace/issues/8) et ADR-008.

Le suivi des fichiers, l'import, la corrélation, l'authentification locale,
l'interface et les paquets installables restent à développer. Le pilote SQLite
est épinglé à v1.60.1 ; mesures de charge et inventaire complet des notices de
dépendances restent à faire avant distribution. La revue sécurité indépendante
des PR reste à mener.

## Composants FileSource disponibles

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

## Identité physique des fichiers

`Inspect` valide un fichier régulier à partir du descripteur et expose sa taille
ainsi que device/inode sur Linux. `Identity.SameFile` compare les identités via
`os.SameFile`, y compris après renommage. Les autres plateformes n'exposent pas
de device/inode persistables dans cette version.

`CapturePrefix` lit au maximum 4096 octets avec `ReadAt`, sans déplacer la
position de lecture. `PrefixFingerprint` conserve la longueur et le SHA-256 ;
`Matches` compare exactement cette même fenêtre, même si le fichier a grandi.
Une fenêtre vide ne correspond jamais. `String` encode la longueur avec le
digest (`sha256:<longueur>:<hex>`) pour une future persistance.

Tests : renommage/remplacement, ajout de données, position de lecture inchangée,
préfixe modifié, troncature, fenêtre bornée/vide et refus d'un répertoire. Sur
Linux, le test garde l'ancien descripteur ouvert pendant le renommage ; sur
Windows, il le ferme avant renommage pour respecter le partage des handles Go.

Limites : une identité physique et un préfixe identique ne prouvent pas une
génération ; la réutilisation d'inode et la troncature exigent encore les ancres
de checkpoint et un diagnostic. La capture n'est pas une vue atomique du fichier
pendant une écriture concurrente. Aucune dépendance ou migration ajoutée.

## Lecture des origines et checkpoints persistés

`source.StateReader` définit `FileOrigins` et le lecteur de checkpoint existant.
`OriginQuery` filtre exactement une source, un device et un inode. Les résultats
sont triés par ID et paginés avec `AfterID`/`NextID`, avec 1 à 100 états par page.
L'identité physique vide est refusée, sans recherche large implicite.

Le stockage SQLite renvoie métadonnées et checkpoint optionnel dans une seule
requête pour chaque page. `OriginState.Checkpoint == nil` signifie absent ; un
checkpoint à zéro reste présent. Les empreintes sont renvoyées telles quelles,
sans tentative de sélectionner ou fusionner les générations.

Tests : état absent, réouverture, métadonnées conservées, pagination sans perte,
checkpoints absents/zéro/non-zéro, séparation source/device/inode et annulation.
Limites : les pages successives ne constituent pas un instantané global et
l'ordre des IDs n'est pas chronologique ; le prochain sélecteur devra en tenir
compte. La boucle de suivi et le choix d'une génération restent à développer.

## Ancres et formats persistés

`CaptureAnchor` hache les derniers `min(offset, 4096)` octets avant l'offset,
sans déplacer la position de lecture. Un offset au-delà de EOF ou une fenêtre
partielle produit une erreur. `CheckpointAnchor.Matches` vérifie la même fenêtre
après append et renvoie une non-correspondance si le fichier a été tronqué.

Format canonique de l'ancre : `sha256:<offset>:<longueur>:<hex>`, à enregistrer
dans `Position.AnchorHash`. `ParseCheckpointAnchor` et `ParsePrefixFingerprint`
rejettent encodages malformés, tailles hors bornes, digests invalides et formes
non canoniques. La chaîne est limitée à 128 octets avant décodage.

Offset zéro : ancre valide, longueur zéro, SHA-256 du contenu vide ; `Matches`
renvoie toujours false car elle ne fournit aucune preuve. Les fenêtres bornées
ne détectent pas une modification hors fenêtre et ne prouvent pas à elles seules
la génération du fichier. Ces helpers ne vérifient pas la frontière de ligne.

Tests : fenêtre courte/longue, round-trip, append, position de lecture inchangée,
modification dans/hors fenêtre, troncature, zéro et nombreux formats invalides.
La sélection de génération et la boucle de suivi restent à développer.

## Dernier lot terminé : vérifier un candidat de reprise

`VerifyCandidate` compare un `source.OriginState` au descripteur ouvert :
identité device/inode, préfixe, ancre, égalité des offsets et séparateur LF
avant le checkpoint. Le chemin enregistré ne sert pas de preuve d'identité.
Le résultat contient un statut et un code de diagnostic fixe, sans contenu
des journaux. La position de lecture n'est pas déplacée.

Une différence d'identité, de contenu ou de taille produit `different`.
Un état malformé, un checkpoint absent/zéro, une empreinte vide, une identité
indisponible ou une frontière de ligne invalide produit `insufficient`.
Les erreurs d'accès au descripteur sont retournées séparément.

Tests : append, chemin enregistré obsolète, position inchangée, changements
d'identité/préfixe/ancre, troncature, états incohérents, frontière de ligne,
absence de preuve, descripteur invalide et répertoire.

Limites : `match` signifie que ces preuves bornées concordent ; les changements
hors fenêtres et les écritures concurrentes ne sont pas couverts. Le contrôle
n'est pas un instantané atomique. L'appelant conserve le namespace de source
fourni par `StateReader`. Aucun choix global ni suivi de fichier n'est ajouté.

## Prochain petit lot : choisir un candidat unique

Reprendre sur `codex/m2-file-source`, conserver la PR #11 et consulter l'issue #4
et ADR-003. Parcourir les pages de `StateReader` pour l'identité du fichier
ouvert et sélectionner une origine seulement lorsque les preuves permettent
un choix unique :

- Réutiliser `VerifyCandidate`, sans supposer les IDs chronologiques.
- Distinguer absence, différence, preuve insuffisante et ambiguïté.
- Borner le parcours, gérer l'annulation et les erreurs du lecteur d'état.
- Tester plusieurs pages et plusieurs candidats concordants ou incomplets.

La création/persistance d'une génération et la boucle de suivi viendront dans
des lots ultérieurs. Les pages ne forment pas un instantané global ; le contrat
du sélecteur devra expliciter cette limite.

## Suite à découper au fil des reprises

1. Sélection d'une génération avec empreintes et ancres de checkpoint.
2. Suivi d'un fichier actif et acquittement par le Sink.
3. Rotation par renommage/création et écritures tardives.
4. Reprise après arrêt, troncature et diagnostic des lacunes.
5. Import historique normal, puis gzip dans un lot distinct.

Chaque demande de continuation traite par défaut un seul petit lot et actualise
ce point de reprise avec le résultat et la prochaine action.
