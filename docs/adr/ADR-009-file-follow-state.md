# ADR-009 — État durable du suivi des générations

Statut : accepté pour le socle de stockage le 4 octobre 2026.

## Contexte

Les origines et checkpoints historiques ne disent pas quels descripteurs étaient
encore suivis avant un arrêt. FirstSeen, l'ID aléatoire et l'offset ne permettent
pas de distinguer un fichier conservé d'une génération retirée après la grâce.

## Décision

Ajouter à `OriginState` un état explicite `FollowUnknown` (0), `FollowFollowing`
(1) ou `FollowRetired` (2). La migration SQLite v2 ajoute une colonne contrainte,
avec 0 par défaut pour toutes les anciennes et nouvelles origines sans déclaration.
Aucun état actif ou retrait n'est déduit des données v1.

`Batch.FollowTransitions` porte origine, état attendu et état cible. Les transitions
autorisées sont inconnu → en suivi, en suivi → retiré et retiré → en suivi pour une
réacquisition explicite. Le retour à inconnu et le retrait direct d'un inconnu
sont refusés. Une seule transition par origine et batch est autorisée.

Le Sink applique ces transitions dans la même transaction que les origines,
observations et checkpoints. La source et l'origine doivent correspondre et l'état
stocké doit être l'état attendu ou déjà la cible pour un réessai idempotent.
Une erreur annule l'ensemble du batch. Un batch sans transition conserve l'état.

## Conséquences

FileSource déclare une acquisition après la décision de génération et la
vérification par `NewIngestor`, avant de consommer une ligne. Le batch change
uniquement l'état vers en suivi ; provenance et checkpoint restent inchangés.
Un état déjà en suivi réutilise son acquittement durable, sans transition vers
lui-même. Une génération retirée sélectionnée explicitement est revérifiée avant
réacquisition. Erreur du Sink (y compris EOF), conflit ou annulation arrête la
source sans consommer de ligne et ferme les descripteurs. Une annulation après
acquittement laisse l'état en suivi.

Le scheduler acquitte désormais en suivi → retiré après EOF stable/grâce et
revérification de taille, avant fermeture/libération de capacité. Le batch ne
change ni origine ni checkpoint. Un fichier vide encore non enregistré expire
sans transition. La génération courante, les lignes partielles et les batches
non acquittés restent protégés. Un ajout observé renouvelle la grâce.

Une erreur du Sink (dont EOF/conflit) ou une annulation avant acquittement arrête
le suivi sans retrait et sans nouvelle lecture/ouverture ; le nettoyage ferme
les descripteurs restants. Après acquittement, une annulation ou un échec de
fermeture conserve le retrait durable. Le descripteur est retiré de la collection
après l'appel à Close, même en erreur, pour éviter une seconde fermeture.
Un arrêt, une annulation ou une erreur de suivi hors expiration n'invente aucun
retrait.
La préparation de reprise distingue désormais les états connus de ceux restés
inconnus avec `LoadFollowOrigins`, après parcours complet borné par chemin.

Les lecteurs par identité et chemin exposent l'état dans la même page que le
checkpoint. Les empreintes et checkpoints restent conservés lors d'un retrait.
Le contrat, la migration, le Sink, l'acquisition et le retrait par FileSource sont
implémentés ; préparation/localisation et réouverture avec revérification des
candidats et observation du courant implémentées ; transfert au scheduler et
reprise de Run restent à développer.

## Préparation des candidats de reprise

`LoadFollowOrigins` réutilise `LoadPathOrigins` pour une source et un chemin
enregistré exacts, avec le même budget de 1 à 1000 états. Un parcours inachevé
renvoie `limit_reached` sans candidats. Après parcours complet, les états retirés
sont écartés ; toute valeur de suivi invalide ou inconnue interdit un plan
automatique. Plus de `MaxOpenGenerations` (2) états en suivi donne
`capacity_exceeded`, sans choisir deux générations par ID, date ou offset.

Priorité des décisions sur un parcours complet : invalide, inconnu, capacité,
absence, ensemble complet. Aucun candidat sur une décision bloquante ; seules
les valeurs en suivi sont rendues dans l'ensemble complet, avec copies détenues
par l'appelant. Un historique uniquement retiré donne absence de candidats en
suivi ; cela ne prouve pas l'absence de fichiers ou d'écritures futures.

Ce choix porte uniquement sur `FollowState`. Checkpoints nil/zéro/positifs,
identité et empreintes restent des métadonnées brutes pour vérification ultérieure.
L'ordre lexical de parcours est conservé sans lui donner de sens temporel. Ce
composant ne choisit pas le fichier courant, n'ouvre aucun journal, ne recherche
aucune rotation et n'écrit aucun état. La capacité vérifiée porte sur les candidats
persistés en suivi ; l'ensemble final des fichiers à ouvrir devra aussi respecter
la capacité (notamment si le chemin courant est une nouvelle génération).

## Localisation des candidats

`LocateFollowOrigins` accepte un ensemble `complete` de 1 ou 2 origines en suivi
du chemin configuré exact, IDs non vides distincts. Il valide tout l'ensemble
avant accès disque et copie les checkpoints. Le répertoire est celui du chemin
configuré résolu en absolu, incluant son fichier courant ; les origines restent
inchangées dans le résultat.

La recherche `SelectRotation` est séquentielle pour chaque candidat. Budget
global explicite de 1 à `MaxFollowLocationEntries` (2000) entrées examinées, avec
au plus `MaxRotationEntries` (1000) et le reste du budget pour chaque recherche.
Une entrée examinée lors de deux recherches compte deux fois. Budget épuisé avant
le candidat suivant : limite sans seconde recherche. Les exclusions, les pages
bornées et la lecture d'une entrée supplémentaire pour établir la fin/limite
restent celles de `SelectRotation`.

Seul un chemin unique pour chaque candidat, distinct des autres chemins, rend
l'ensemble `unique`. Deux origines localisées sur le même chemin donnent
ambiguïté. La première décision non unique est conservée dans l'ordre des
candidats (pas une priorité globale des causes), avec compte examiné mais aucun
chemin partiel utilisable. Erreur ou annulation efface tout le résultat.
La vérification est stricte, sans relecture zéro implicite ; les checkpoints nil
ou zéro restent soumis à une décision de preuves insuffisantes.

Tous les descripteurs temporaires sont fermés avant retour. Les chemins devront
être rouverts et revérifiés avant utilisation ; aucun état, ingestion ou
descripteur durable n'est modifié. L'ordre des recherches ne choisit pas le
fichier courant. La capacité finale, incluant un éventuel nouveau courant, et
le raccordement au démarrage restent des lots distincts.

## Réouverture de l'ensemble localisé

`OpenFollowLocations` accepte seulement un ensemble entièrement `unique` de
1 ou 2 états en suivi. Avant accès disque : source file/normaliseur valides,
IDs non vides distincts, même chemin d'origine non vide, chemins sélectionnés
absolus et distincts après nettoyage lexical ; copies de tous les checkpoints.
Le namespace source reste garanti par l'appelant et ses lecteurs.

Chaque fichier est rouvert en lecture seule par `OpenLog`, vérifié strictement
par `VerifyCandidate`, puis revérifié et positionné par `NewIngestor` au checkpoint
positif. L'identité physique, les fenêtres préfixe/ancre et la frontière LF sont
contrôlées sans consommation de ligne, normalisation ni écriture d'état. Un
checkpoint absent/zéro ou une preuve insuffisante n'autorise aucune relecture
implicite. Deux ouvertures du même fichier physique donnent ambiguïté, même
avec deux chemins distincts.

Le succès rend un `OpenedFollowSet` opaque propriétaire de tous les descripteurs.
`Len` expose sa capacité ; `Close` ferme l'ensemble sans modifier les états durables.
La collection est vidée avant fermeture, même en erreur, pour ne pas réessayer
un Close et fermer les autres fichiers. Close est idempotent ; valeur zéro et
récepteur nil admis. L'objet ne doit pas être copié ni utilisé concurremment.

Échec/annulation, notamment sur le second fichier : toutes les ouvertures acquises
sont fermées et aucun ensemble partiel n'est rendu. Cause initiale et erreurs de
fermeture sont conservées. Descripteurs restent au propriétaire jusqu'à Close ;
choix du courant, transfert au scheduler et raccordement à Run seront séparés.
Les vérifications bornées successives ne forment pas un snapshot atomique.

## Observation du courant parmi les descripteurs

`OpenedFollowSet.ObserveCurrent` exige un chemin configuré absolu et un propriétaire
valide non vide de 1 ou 2 fichiers. Il inspecte tous les descripteurs, puis réutilise
`ObservePath` et compare l'identité actuelle via `SameFile`. IDs d'origine et
identités physiques de la collection doivent être distincts. Un second fichier
inutilisable ne peut pas être ignoré parce que le premier semble courant.

Résultats : `known` avec origine et snapshot courant ; `missing` sans courant
inventé ; `new_generation` avec snapshot seulement s'il reste une place ;
`capacity_exceeded` sans identité utilisable si les deux places sont prises.
Ordre d'ingestion, ID, date et offset ne choisissent pas le courant. Les liens
vers un fichier régulier suivent la même observation physique que `ObservePath`.

Erreur, propriétaire fermé/invalide, chemin non régulier ou annulation : résultat
vide, cause conservée et propriété des fichiers conservée pour nettoyage par
l'appelant. Ni ouverture/lecture/seek/fermeture, consommation, changement de grâce,
transition ni transfert. Les snapshots ne sont pas atomiques et `known` ne
revalide ni checkpoint ni intégrité : contrôles de suivi restent nécessaires.
Une nouvelle génération exigera ouverture vérifiée, acquisition et contrôle de
capacité ; raccordement au démarrage reste un lot distinct.

## Transfert d'un ensemble avec courant connu

`FileSource.FollowOpened` accepte un `OpenedFollowSet` vérifié et une décision
`known`. Avant transfert : contexte, Sink, garde d'exécution partagée avec Run,
ensemble valide, nouvelle observation du chemin et concordance OriginID/identité
physique avec la décision fournie ; identité de source complète de chaque
ingesteur égale à la configuration. Toute erreur conserve le propriétaire et son
statut de chemin. Une décision devenue obsolète retourne `ErrPathChanged`.
Les décisions missing/new/capacity exigent encore un traitement distinct.

La collection du propriétaire est vidée avant remise au scheduler commun. Son
Close devient inoffensif. Le scheduler possède seul les descripteurs : contrôle
des tailles/ancres avant consommation, suivi conjoint, polling/grâce et retrait
durable, fermeture une seule fois au retrait ou à la sortie. Les ingesteurs déjà
préparés reprennent leur checkpoint sans nouvel enregistrement/acquisition.
Le courant est désigné par OriginID, sans dépendre de l'ordre de collection.
LastPathStatus est réinitialisé au transfert puis actualisé par le polling.

Une réécriture/troncature de la même identité physique peut être découverte
après transfert : arrêt et fermeture par le scheduler, checkpoint préservé.
Les vérifications ne constituent pas un verrou du système de fichiers. Après
transfert, une disparition du chemin conserve le dernier courant observé selon
le suivi existant ; elle ne justifie pas une décision missing au démarrage.
Les accès au propriétaire et écritures de namespace restent à sérialiser.
Run ne lance pas encore le pipeline de reprise des ensembles persistés.

## Limites

Le stockage n'observe ni descripteur, EOF, grâce ni empreinte : l'appelant justifie
la transition. Retiré ne prouve pas que le fichier ne recevra jamais d'ajout. La
réacquisition exige une vérification explicite sans reset de checkpoint. Un seul
écrivain doit sérialiser acquisition, retrait et réessais pour une source ; ce
protocole ne comporte pas d'époque de propriétaire ou de protection contre des
réessais obsolètes après un cycle complet de réacquisition.

Observation de taille, transaction du Sink et fermeture ne sont pas atomiques
avec les écritures du journal : un ajout après le dernier contrôle, notamment
pendant l'acquittement du retrait, peut être manqué. La grâce et l'état retiré
ne prouvent pas l'absence d'écritures ultérieures. Le Sink doit respecter le
contrat d'acquittement durable et l'annulation ; aucun réessai automatique.

L'enregistrement initial (origine/checkpoint zéro) précède l'acquisition dans une
transaction distincte. Si l'acquisition échoue ou si aucune première ligne n'est
acquittée, le checkpoint reste zéro ; la reprise exige toujours la politique
explicite `AllowZeroCheckpoint`. Les lectures de métadonnées nécessaires à la
vérification précèdent l'acquisition ; aucune ligne n'est consommée avant elle.
