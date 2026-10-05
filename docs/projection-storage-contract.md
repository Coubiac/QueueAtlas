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

## Suite

Définir les tables et contraintes des révisions/projections, puis remplacement
atomique sous contrôle des faits courants et lecture cohérente. La rétention doit
invalider une projection dont des preuves disparaissent. Garder les réserves et
les références physiques, sans identité globale créée depuis du texte identique.
