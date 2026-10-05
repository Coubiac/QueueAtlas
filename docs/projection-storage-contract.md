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

Installer atomiquement un manifest après relecture des faits courants sous verrou
d'écriture ; refuser une entrée devenue obsolète. Puis lire/reconstruire une révision
en vérifiant sa fraîcheur. Aucun write public, cache dérivé ou recalcul transactionnel
n'est livré par107. Couverture, continuité et parcours global restent non prouvés.
