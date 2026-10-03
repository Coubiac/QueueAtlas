# Point de reprise QueueAtlas

Mis à jour le 4 octobre 2026. Ce fichier décrit le dernier état connu ; vérifier
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
- Sixième lot FileSource : sélection bornée d'un candidat unique dans
  `internal/source/file/selection.go`, commit
  `362817ad349a3bd0f438e8e7fd84cb7040ac2721`, toujours dans la PR #11.
- Septième lot FileSource : enregistrement initial et retour d'une reprise
  vérifiée dans `internal/source/file/generation.go`, commit
  `9902546d1a1b342b9f834a96afe5842863203fe2`, toujours dans la PR #11.
- Huitième lot FileSource : ingestion d'une ligne et acquittement de son
  checkpoint dans `internal/source/file/ingestor.go`, commit
  `2d3c7d7ab72f07b0bbfccef2932cceb7c2531e07`, toujours dans la PR #11.
- Neuvième lot FileSource : attente annulable des ajouts sur une génération
  ouverte, dans `internal/source/file/follow.go`, commit
  `719d5139c345e4eede483a4646eabc172e162833`, toujours dans la PR #11.
- Dixième lot FileSource : décision d'attente sans écriture pour une génération
  neuve vide, dans `generation.go`, commit
  `90e4efcb6a0a8bc63f1b02f911618c5e2a01ade5`, toujours dans la PR #11.
- Onzième lot FileSource : politique explicite de relecture à zéro dans les
  vérificateur/sélecteur/décision de génération, commit
  `a879283d0de1b632dc7d9960662eb1ef6b1ca591`, toujours dans la PR #11.
- Douzième lot FileSource : ouverture en lecture seule et vérification du
  chemin dans `internal/source/file/open*.go`, toujours dans la PR #11.
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
- Validation locale du lot sélection : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI sélection](https://github.com/Coubiac/mailtrace/actions/runs/37156470072)
  réussie, incluant les preuves réelles sur Linux et les builds sans CGO.
- Validation locale du lot génération : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI génération](https://github.com/Coubiac/mailtrace/actions/runs/37156904624)
  réussie, incluant la réouverture SQLite sur Linux et les builds sans CGO.
  La CI du commit documentaire `02c4744` est aussi confirmée réussie.
- Validation locale du lot ingestion : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI ingestion](https://github.com/Coubiac/mailtrace/actions/runs/37157404479)
  réussie, incluant le réessai idempotent SQLite et les builds sans CGO.
- Validation locale du lot suivi ouvert : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI suivi ouvert](https://github.com/Coubiac/mailtrace/actions/runs/37157772435)
  réussie, incluant attente/append/annulation et builds Linux sans CGO.
- Validation locale du lot fichier vide : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI fichier vide](https://github.com/Coubiac/mailtrace/actions/runs/37158062815)
  réussie, incluant attente sans écriture, append et builds Linux sans CGO.
- Validation locale du lot politique zéro : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI politique zéro](https://github.com/Coubiac/mailtrace/actions/runs/37158472028)
  réussie, incluant relecture réelle après réouverture SQLite et builds sans CGO.
- Validation locale du lot ouverture : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis. Consulter
  les checks de la PR #11 pour la CI du dernier commit.
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

Limites : le lecteur seul ne remplit pas OriginID, ReadAt et Observation ;
le composant d'ingestion ci-dessous les attribue. Le lecteur ne suit pas les
chemins, ne détecte pas les rotations et ne persiste aucun checkpoint seul.
Une lecture bloquante reste à interrompre par son propriétaire.

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
l'ordre des IDs n'est pas chronologique. Le sélecteur ci-dessous utilise cet
ordre uniquement comme curseur. La boucle de suivi reste à développer.

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
La boucle de suivi reste à développer.

## Vérifier un candidat de reprise

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

## Choisir un candidat unique

`SelectResume(ctx, fichier, sourceID, StateReader)` interroge exactement la
source et l'identité du descripteur, puis réutilise `VerifyCandidate`.
`unique` exige un parcours terminé, une seule correspondance et aucun candidat
aux preuves insuffisantes. Seul ce statut contient une copie de l'origine et
du checkpoint sélectionnés ; aucun choix par date, ID ou offset maximal.

Les autres statuts sont `absent`, `different`, `insufficient`, `ambiguous` et
`limit_reached`. Deux correspondances suffisent pour conclure à l'ambiguïté.
Le parcours examine au maximum 1 000 candidats, dans des pages de 100 au plus,
sans accumuler toutes les origines. Une page incohérente (ordre, curseur,
identité ou taille), une erreur d'accès ou une annulation ne renvoie aucun
résultat partiel sélectionnable. Les plateformes sans device/inode persistable
renvoient `insufficient`, sans requête large de remplacement.

Tests : plusieurs pages, candidat unique avant/après les différences, preuves
insuffisantes bloquant le choix, deux correspondances, copie du checkpoint,
limite exacte/dépassée, pages invalides, erreurs et annulation après une
correspondance. Les tests Linux vérifient aussi la sélection face aux preuves
réelles d'un fichier et la position de lecture inchangée.

Limites : l'appelant doit sérialiser les écritures d'origines/checkpoints de
cette source pendant la sélection et l'application du résultat. Les pages ne
forment pas un instantané global ; le sélecteur n'ajoute ni verrou ni transaction
globale. Les limites des fenêtres et écritures concurrentes du fichier restent
celles de `VerifyCandidate`. Aucun état n'est écrit par ce lot.

## Enregistrer une nouvelle génération

`EnsureGeneration` sélectionne dans le namespace d'une identité de source
`file`. Pour `absent` ou `different`, il capture identité et préfixe du
descripteur. Si le préfixe est non vide, il crée un ID opaque aléatoire de
128 bits puis transmet origine et checkpoint zéro avec ancre canonique dans
un seul `Sink.Commit`, sans record. Une capture vide diffère cet enregistrement.

Le résultat contient l'état et `Created == true` seulement après acquittement.
Pour `unique`, il retourne l'origine et le checkpoint vérifiés sans écrire.
Pour `insufficient`, `ambiguous` ou `limit_reached`, il retourne le diagnostic
avec état absent. Une erreur de lecture, de Sink ou une annulation avant commit
ne produit pas d'état utilisable. Le helper ne déplace pas la lecture.

Tests : validation des entrées, annulation, absence d'identité persistable ;
sur Linux, état initial d'un fichier non vide retrouvé après réouverture
SQLite, origine et checkpoint transmis ensemble, chemin/identité/date conservés,
reprise existante, nouvelle génération après différence, refus des décisions
incomplètes/ambiguës/limitées et erreurs de Sink/lecteur.

Limites : sérialisation de la source à assurer par l'appelant ; captures du
fichier non atomiques. Un arrêt après création mais avant la première ligne
validée laisse un checkpoint zéro qui exige la politique explicite décrite
ci-dessous pour une relecture automatique.
Une ancienne origine capturée vide conserve son empreinte vide immuable et ne
fournit pas de preuve de contenu lors d'une reprise future. Les nouvelles
origines vides sont désormais différées. Leur reprise sans preuve reste à
définir avant de déclarer FileSource terminé.

## Acquitter une ligne complète

`NewIngestor` reçoit l'identité de source, l'état de génération décidé et une
fonction de normalisation injectée. Il vérifie le checkpoint initial, son
contenu/frontière LF, le préfixe et l'identité physique disponible, puis positionne
le descripteur à cet offset. `Position()` expose seulement l'état acquitté.

`CommitNext(ctx, Sink)` envoie une ligne complète et son checkpoint dans un même
batch. Il attribue OriginID, ReadAt et SourceID configuré. Les lignes trop longues
contournent le parser et deviennent des observations `unknown` avec erreur,
tout en validant la consommation physique jusqu'au séparateur.

Le lecteur transmet les fragments consommés à une fenêtre roulante de 4 Kio,
initialisée depuis l'ancre de reprise. Le digest de fin est calculé sur ces
octets, y compris après lecture anticipée ou pour les suffixes de lignes trop
longues ; il n'est pas recalculé depuis un fichier éventuellement réécrit.

EOF partiel n'écrit rien et conserve la ligne en cours. En cas d'erreur du Sink
ou d'annulation avant commit, le batch complet reste en mémoire et sera réémis
à l'identique avant toute lecture suivante. L'offset acquitté avance seulement
après succès ; normalisation et heure de lecture restent stables au réessai.

Tests : normalisation Postfix réelle, CRLF, source configurée, reprise à offset
non nul, fenêtres courtes/longues, EOF répété puis append, ligne trop longue,
réessai après erreur/annulation, fichier réécrit après lecture anticipée,
checkpoint initial invalide et commit SQLite réussi suivi d'un accusé perdu
(un seul record et événement après réessai).

Limites : un seul appelant et ownership exclusif des lectures/seeks du
descripteur ; sérialisation des écritures de source à assurer par l'appelant.
Le Sink ne doit pas modifier le batch ; le normaliseur cède son résultat à
l'ingestion. Aucun polling, suivi de chemin ou diagnostic de rotation/troncature
pendant lecture dans ce lot. Le constructeur n'arbitre pas une reprise jugée
insuffisante : l'état doit venir de la décision de génération.

## Attendre les ajouts à EOF

`Ingestor.Follow(ctx, Sink, intervalle)` poursuit les commits sur la génération
déjà ouverte. Un EOF du lecteur provoque une attente avec timer annulable, puis
une nouvelle lecture conservant la ligne partielle. L'intervalle zéro choisit
une seconde ; une valeur explicite doit être entre 10 ms et une minute.

Les autres erreurs sont retournées immédiatement. Un EOF du Sink, même enveloppé,
reste une erreur de commit et ne déclenche aucune attente ou tentative automatique.
Le batch reste disponible dans l'ingesteur pour un réessai explicite. Aucun timer
n'est actif entre les lignes déjà disponibles ; un seul timer est utilisé par
attente, arrêté au retour. L'annulation à EOF ne dépend pas de l'intervalle choisi.

Tests : append après EOF partiel, parcours de plusieurs lignes, attente réelle
avec timer puis append, annulation pendant une attente longue, erreurs du Sink
(dont EOF direct/enveloppé) sans boucle de réessais, erreur de lecture et intervalles
invalides sans progression. Les tests couvrent aussi le batch réessayable après arrêt.

Limites : suivi du seul descripteur, pas du chemin ; aucun choix de génération,
fermeture de fichier ou diagnostic de rotation/troncature. Un seul appelant ;
l'annulation pendant une lecture bloquante ou un commit reste conditionnée aux
contrats du lecteur et du Sink. FileSource n'est pas encore complet.

## Différer une génération initialement vide

`GenerationStart.WaitingForContent` est vrai uniquement lorsque la sélection
autorisait une nouvelle origine (`absent` ou `different`) mais que le préfixe
capturé contient zéro octet. Aucun ID, origine ou checkpoint n'est enregistré ;
`State` reste absent et `Created` faux. L'appel ne déplace pas la lecture.

L'appelant peut relancer `EnsureGeneration` après ajout de contenu. La décision
est alors refaite, et un préfixe non vide permet l'enregistrement initial normal.
Il n'y a pas de boucle d'attente ajoutée à ce helper. Les origines persistées
avec empreinte vide restent diagnostiquées `insufficient`, même après append,
sans réécriture de leur identité ou remplacement implicite.

Tests Linux : décisions répétées sur fichier vide sans aucun commit, état SQLite
inchangé pour source sans origine ou après troncature d'une génération connue,
position inchangée, enregistrement après append avec préfixe non vide, et
conservation d'une ancienne origine vide avant/après append. Les tests de
registration existants couvrent maintenant aussi un fichier d'un seul octet.

Limites : une ancienne origine vide exige encore une décision explicite ;
checkpoint zéro et course de réécriture du fichier restent des comportements
distincts. Le scheduler de chemin devra gérer `WaitingForContent` avant de créer
un ingesteur. Aucune migration ou dépendance ajoutée.

## Politique explicite de reprise à zéro

Les entrées `VerifyCandidateWithPolicy`, `SelectResumeWithPolicy` et
`EnsureGenerationWithPolicy` acceptent `ResumePolicy{AllowZeroCheckpoint: true}`.
Les entrées existantes et la valeur zéro de la politique restent strictes.

La relecture exige un checkpoint présent à zéro avec OriginID cohérent, une
ancre zéro canonique, une identité physique concordante et un préfixe non vide
concordant. Le résultat est `restart_zero`, distinct de `match`/`unique` fondés
sur une ancre positive. L'ancre vide ne devient pas une preuve de continuité.

Le sélecteur applique les mêmes limites, pagination et règle de choix unique.
Deux candidats à zéro, ou un candidat à zéro et un positif, sont ambigus.
Un candidat incomplet ou à empreinte vide bloque toujours la décision. Aucune
priorité par offset, date ou ID ; aucune nouvelle origine pour une relecture.

La décision de génération renvoie l'origine et son checkpoint zéro sans écrire
au Sink, sans marquer `Created` ou `WaitingForContent`. L'ingesteur peut alors
relire depuis zéro ; son premier commit avancera le checkpoint normalement.
Un préfixe changé, avec politique activée, est une différence et peut conduire
à une nouvelle génération suivant les règles existantes.

Tests : choix paginé à zéro avant/après des différences, ambiguïté zéro/zéro
et zéro/positif dans les deux ordres, preuves insuffisantes. Tests Linux : append,
préfixe modifié/tronqué, ancre incohérente/non canonique, identité/OriginID,
checkpoint absent/négatif, préfixe vide ; réouverture SQLite, mode strict par
défaut, décision sans écriture puis relecture réelle de la première ligne et
retour à une reprise positive.

Limites : l'activation autorise une relecture ; elle ne prouve pas la génération
avec une ancre positive. Les garanties bornées/non atomiques du préfixe et la
sérialisation de la source restent requises. Les anciennes empreintes vides
restent insuffisantes. La politique n'est pas encore exposée par une CLI/config.

## Dernier lot terminé : ouvrir et vérifier un chemin de journal

`OpenLog(ctx, chemin)` retourne un descripteur en lecture seule, à l'offset zéro,
et son identité physique, après vérification d'un fichier régulier. L'identité
doit être identique avant l'ouverture, sur le descripteur et sur le chemin après
l'ouverture. Les liens symboliques vers des fichiers réguliers sont acceptés.

Un remplacement ou une disparition observée pendant cette séquence produit
`ErrPathChanged` ; la disparition conserve aussi `fs.ErrNotExist`. Tous les
échecs après ouverture ferment le descripteur et ne retournent ni fichier ni
identité utilisables. Après succès, la fermeture appartient à l'appelant.

Sur Linux, `O_NONBLOCK` empêche une course de remplacement par FIFO d'attendre
un écrivain ; le descripteur non régulier est ensuite refusé. Les autres
plateformes font le contrôle préalable et après ouverture mais ne garantissent
pas une ouverture non bloquante pendant une course de remplacement.

Tests : lecture dès zéro, écriture refusée/contenu inchangé, chemin vide/absent,
répertoire, annulation, identités remplacées et fermeture sur échec. Tests Linux :
lien symbolique régulier puis cible changée, rename/create avec ancien descripteur
ouvert, FIFO connu et remplacement par FIFO après un contrôle régulier.

Limites : les vérifications ne verrouillent pas le chemin et ne prouvent pas la
génération ; les changements ultérieurs restent à surveiller. L'annulation est
vérifiée entre appels système et n'interrompt pas un appel de filesystem bloqué.
Aucun Sink, suivi de rotation, migration ou dépendance ajouté.

## Prochain petit lot : orchestrer le démarrage FileSource

Reprendre sur `codex/m2-file-source`, conserver la PR #11 et consulter l'issue #4
et ADR-003. Ajouter une implémentation initiale du contrat `source.Source` qui
relie les composants disponibles sur un chemin configuré :

- Valider configuration, normaliseur, StateReader et intervalle avant écriture.
- Ouvrir avec `OpenLog`, décider avec `EnsureGenerationWithPolicy` et démarrer
  l'ingesteur seulement si un état est utilisable.
- Attendre de manière annulable pour `WaitingForContent`, en refaisant ouverture
  et décision ; retourner les décisions insuffisantes/ambiguës/limitées clairement.
- Appliquer `Ingestor.Follow`, fermer le descripteur à la fin et interdire deux
  exécutions simultanées du même objet Source.
- Tester démarrage, append après fichier vide, reprise, annulation et erreur Sink.

La rotation et la troncature pendant suivi restent les lots suivants ; documenter
le suivi du seul descripteur dans cette implémentation initiale. Pas de CLI ici.

## Suite à découper au fil des reprises

1. Démarrage Source.Run avec décision de génération et attente initiale.
2. Rotation par renommage/création et écritures tardives, en lots distincts.
3. Reprise après arrêt, troncature et diagnostic des lacunes.
4. Décisions insuffisantes restantes, dont anciennes empreintes vides.
5. Import historique normal, puis gzip dans un lot distinct.

Chaque demande de continuation traite par défaut un seul petit lot et actualise
ce point de reprise avec le résultat et la prochaine action.
