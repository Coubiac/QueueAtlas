# Contrat de stockage des projections

## Lot106 : lecture bornée des faits persistés

`Store.CorrelationFacts` fournit les faits d'un périmètre explicite pour les
primitives de reconstruction. Scope non vide, au plus64 parties au total :
clés exactes `(instance configurée, queue_id)` et instances dont les faits sans
file/NOQUEUE sont demandés. Instances non vides <=1024 octets, IDs de file non
vides <=32, sans NUL/CR/LF/tabulation ; doublons dans chaque catégorie refusés.
Les valeurs sont des paramètres SQL littéraux ; elles ne deviennent pas du SQL
ou des motifs. Un même fait sélectionné par plusieurs conditions apparaît une fois.

Une seule requête joint événements et enregistrements bruts. Elle sélectionne
toutes les origines stockées pour les clés choisies, sans plage de date qui
supprimerait des faits non datés, puis trie source/origine/offset physique.
Les IDs d'insertion ne définissent pas les faits ou leur ordre. Instance vient de
l'événement persisté (source de confiance, ou SourceID de repli), jamais de Host.
Le lecteur conserve provenance, brut BLOB, message, valeurs/champs présents,
diagnostics du parser et contexte de date persistés. Il ne reparcourt pas le brut
avec un nouveau parser ou une nouvelle année. Les diagnostics de lecture physique
restent dans raw_records, hors du modèle Fact de corrélation.

Instant NULL reste NULL, même quand Raw/Year/Zone/Quality sont présents ; notamment
l'écrivain existant ne stocke pas de valeur UnixNano qui déborderait. Aucun instant
n'est reconstitué depuis ces métadonnées. Plusieurs origines restent distinctes.
Ces faits sont les valeurs persistées, avec les normalisations de l'écrivain
existant ; ils ne prétendent pas retrouver des valeurs antérieures au stockage.

La limite est positive <=4096, avec une ligne supplémentaire recherchée. Si elle
est dépassée, toute la lecture échoue avec ErrPartitionLimit : aucune page tronquée
ne devient un snapshot de corrélation. Scope invalide donne ErrCorrelationScope.
Les provenances sont ensuite validées par PartitionFacts ; aucune sortie partielle
sur refus ou annulation. Une limite de lignes ne borne pas indépendamment les
métadonnées ni le travail de tri SQL ; le caller conserve son contexte/deadline.

JSON des champs : exactement les membres fields et present, chacun une fois.
Maps entières nulles admises comme produites par l'écrivain ; entrées nulles,
types incorrects, doublons, membres manquants/inconnus ou casse différente refusés.
Les clés de maps restent des valeurs littérales ; leurs entrées sont strictement
string/bool. JSON UTF8 invalide refusé, sans rejeter les octets du brut BLOB.
ErrCorrelationStoredFields est fixe sans valeur stockée. Une erreur de conversion
Scan donne ErrCorrelationStoredFact fixe ; annulation et erreurs de requête restent
distinctes. Ce contrôle de forme n'est pas une preuve d'authenticité de la base.

L'absence d'une file signifie son absence dans le périmètre/base sélectionnés,
pas dans tous les journaux. Le caller choisit toutes les clés pertinentes et les
instances nécessaires aux relations ; les réserves de couverture restent exigées.
La lecture ne modifie ni faits, checkpoints, manifests ni schéma. Aucun stockage
de projection, recalcul transactionnel, rétention ou interface livré par106.

## Lot107 : schéma v4 des manifests de révision

Migration transactionnelle v3 vers v4 : faits, événements et checkpoints conservés,
cinq tables de manifests initialement vides. Aucun résultat dérivé n'est sérialisé.
Le périmètre, ses parties, les révisions, leurs mappings et tous les faits d'entrée
ont des tables distinctes. Instance/queue/relay sont des BLOB pour préserver leurs
octets, y compris UTF8 invalide ; les limites et caractères de contrôle refusés
du contrat106 restent appliqués. SHA256 stricts, format1, parties/mappings0..63,
nombre de faits0..4096 et fenêtre positive <=24h sont contraints.

La révision courante appartient obligatoirement au même périmètre grâce à une
clé étrangère composite. Un membership référence à la fois raw_records et events ;
supprimer un parent ou modifier un fait référencé est refusé. La rétention devra,
dans une transaction, retirer le pointeur courant et les manifests concernés avant
de supprimer leurs faits. Le test SQL vérifie ce protocole de contraintes ; aucune
API de rétention n'est encore livrée. Voir les [clés étrangères SQLite](https://www.sqlite.org/foreignkeys.html).

Les IDs SQLite servent aux références internes ; FactRef reste l'identité publique.
Les memberships devront inclure tous les faits, y compris Other et Unresolved,
et pas seulement les preuves positives. Le schéma borne fact_count mais ne vérifie
pas sa concordance avec les lignes, ni le hash ou l'ordre canonique des parties.
Ces contrôles appartiennent à la future API. Ajouter un fait ne déclenche pas les
contraintes de suppression/mutation : la fraîcheur doit être contrôlée séparément.

## Suite

## Lot108 : installation transactionnelle du manifest

`Store.InstallProjection` reçoit le périmètre, tous ses faits, la limite et les
options. Le caller garde ces entrées immuables pendant l'appel. BuildProjection
calcule d'abord hors transaction ; la transaction acquiert ensuite une réservation
d'écriture avant de relire les faits sélectionnés, avec le même lecteur que106.
Un autre écrivain ne peut donc valider de nouveaux faits entre contrôle et commit.
Nombre, ensemble des FactRef et révision complète recalculée doivent correspondre.
Sinon ErrProjectionStale, sans sortie ni remplacement. Un dépassement de limite
ou des métadonnées invalides restent des refus complets du lecteur existant.

Le scope est un ensemble canonique trié par kind/instance/queue, avec domaine
`projection-scope-v1`, compte et chaînes encadrées par leur longueur uint64 BE.
Les parties absentes de la base participent aussi au hash. Les octets sont exacts,
sans conversion JSON/UTF8. La révision et toutes ses options, avec tous les IDs
internes des faits d'entrée, sont installées dans la même transaction ; fact_count
correspond aux memberships écrits. Pointeur courant et memberships sont validés
ensemble. Une erreur après suppression de l'ancien manifest revient entièrement
à l'état précédent, y compris lors de la création d'un nouveau scope.

Chaque scope conserve uniquement son manifest courant : remplacement supprime
ses anciennes révisions, sans modifier les autres scopes ni leurs références.
Réinstaller la même révision peut renouveler ID interne/date ; aucune API d'historique
n'est promise. Le résultat retourné reste la projection pure avec les mêmes réserves,
aucun résultat dérivé n'est sérialisé. ErrProjectionInstall est fixe pour les erreurs
d'installation SQL, context annulé distinct. Cette API ne certifie pas une base
modifiée par un tiers hors du contrat ; elle n'authentifie ni faits ni hashes.

La réservation est obtenue par un UPDATE sans ligne correspondante avant SELECT ;
un test avec deux connexions vérifie le refus de commit concurrent puis la libération
au rollback. La sélection/refonte du lecteur conserve sa requête unique et retourne
les IDs internes uniquement pour les FK, jamais comme identité publique.

## Prochaine étape

Lire/reconstruire le manifest et contrôler sa fraîcheur après ingestion ultérieure.
Un manifest installé peut devenir obsolète dès qu'un fait supplémentaire est ingéré ;
il ne doit pas être affiché comme actuel sans cette revalidation. Aucun lecteur public
de manifest ni cache dérivé livré par108. Couverture, continuité et parcours global
restent non prouvés ; rétention applicative reste à développer.
