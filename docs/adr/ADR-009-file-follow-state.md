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
candidats, observation du courant, transfert au scheduler et reprise de Run
implémentés. Relecture zéro explicite limitée à une génération en suivi courante
implémentée ; récupération des lifecycle inconnus encore à développer.

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
choix du courant, transfert au scheduler et raccordement à Run sont des étapes
distinctes décrites ci-dessous.
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
Les décisions missing/capacity restent refusées ; diagnostics missing et traitement
new sont décrits ci-dessous.

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
Run utilise désormais le pipeline de reprise des ensembles persistés.

## Nouveau courant avec une génération conservée

`FollowOpened` accepte aussi `new_generation`, sans OriginID, avec un seul fichier
dans l'ensemble. Deux fichiers déjà détenus provoquent ErrRotationCapacity avant
toute ouverture. Nouvelle observation du chemin et concordance du snapshot,
identités de source, tailles/ancres de l'ancien sont vérifiées avant ouverture.
OpenLog vérifie le chemin/descripteur et l'identité doit encore correspondre au
snapshot observé. Refus, erreur d'ouverture, remplacement observé ou annulation
avant transfert : ancien propriétaire conservé, éventuel nouveau descripteur fermé.

Après ouverture vérifiée, collection du propriétaire vidée et statut réinitialisé.
La préparation de la nouvelle génération réutilise décision/verification/acquisition
existantes, avant toute ligne des deux fichiers. Échec, EOF du Sink ou annulation
ferme les deux fichiers, sans retrait inventé ni avancement de checkpoint de l'ancien.
Un nouvel enregistrement peut déjà être durable à offset zéro ; acquisition et
enregistrement restent deux transactions, et une annulation après acquittement
conserve l'état en suivi. La politique stricte de reprise zéro demeure applicable.

Un fichier neuf vide reste détenu comme courant sans origine/checkpoint/acquisition.
Le scheduler attend ses ajouts tout en lisant l'ancien, puis prépare/acquiert le
nouveau avant sa première ligne. Les ingesteurs anciens ne sont pas réenregistrés.
Suivi conjoint, contrôles périodiques, grâce et capacité restent ceux du scheduler.
Les contrôles ne sont pas atomiques ; un remplacement supplémentaire après transfert
peut arrêter à capacité pleine. Run applique désormais ce même cœur après préparation.

## Absence du courant au démarrage

Une décision `missing` canonique (sans origine/snapshot) est recontrôlée par
FollowOpened : ensemble valide, descripteurs inspectables, absence encore observée,
identités complètes de source et contexte valides. `ErrCurrentMissing` est un
diagnostic fixe exploitable via errors.Is, sans chemin ni contenu. Aucune génération
retenue n'est choisie comme courant ; le propriétaire conserve les 1–2 fichiers,
positions/ingesteurs/grâce, LastPathStatus et état durable. Ni ouverture ni lecture
de contenu, attente, écriture ou transfert. La garde d'exécution est libérée.

Une décision obsolète lorsque le chemin réapparaît donne ErrPathChanged : l'appelant
doit observer à nouveau, puis appliquer known/new ou traiter capacity. Une erreur
de stat, un ensemble/source invalide ou une annulation reste cette cause d'erreur,
sans être masquée par le diagnostic missing. Un appelant abandonnant la reprise
doit fermer le propriétaire. Pas de réessai automatique. Au cours d'un suivi déjà
transféré, le scheduler garde le dernier courant observé sur disparition ; ce
comportement distinct ne justifie pas un choix arbitraire au démarrage. Run ferme
l'ensemble préparé lorsque la décision bloque, sans propriétaire à rendre au client.

## Orchestrateur de préparation de reprise

`PrepareFollowResume` relie LoadFollowOrigins → LocateFollowOrigins →
OpenFollowLocations → ObserveCurrent. Avant tout accès à l'état : identité de
source file complète, chemin configuré absolu, PathStateReader/normaliseur et
budgets explicites FollowResumeLimits (Origins de 1 à 1000, Entries de 1 à 2000).
Les limites des composants restent applicables : pages de 100, scans de 1000
entrées chacun au plus, budget partagé comptant répétitions/exclusions.

Un parcours complet vide ou entièrement retiré donne `absent`, sans accès journal,
et ne démarre pas lui-même une génération fraîche. Unknown bloque via
ErrUnknownFollowState, invalide via ErrInvalidFollowState, capacité via
ErrRotationCapacity, limite via ResumeDecisionError/limit_reached. Aucune décision
bloquante ne devient absence ou fallback. Localisation non unique : diagnostic de
sélection conservé (absent/different/insufficient/ambiguous/limit_reached), résultat
vide ; strict nil/zéro maintenu dans cette fonction. L'exception explicite pour
une génération en suivi à zéro est décrite ci-dessous.

Après réouverture vérifiée de tout l'ensemble, courant missing/capacity bloque et
ferme tous les descripteurs sans état partiel. Erreur d'observation ou annulation
conserve sa cause et joint les erreurs de fermeture. `ready` seul fournit le
propriétaire complet et une observation known/new ; l'appelant doit fermer ou
appliquer via FollowOpened. Pas de consommation/normalisation/Sink/transition ni
transfert au scheduler ; nouvelle génération courante seulement observée, jamais
ouverte ici. Recontrôle lors de l'application toujours nécessaire.

Les accès à l'état doivent être sérialisés par source jusqu'à application ; aucune
garde d'exécution de FileSource ni snapshot atomique ajouté. Cette fonction de
préparation reste utilisable isolément. Son intégration à Run est décrite ci-dessous.

## Démarrage de Run avec reprise d'ensemble

Run garde son verrou pendant préparation et application. Le StateReader injecté
doit aussi implémenter PathStateReader ; sinon ErrPathStateReaderRequired arrête
avant tout accès journal, sans fallback. New reste utilisable pour les helpers
avec StateReader seul, mais Run impose ce contrat supplémentaire.

Config.ResumeLimits est copié/validé par New : chaque champ zéro prend son maximum
(1000 états, 2000 entrées partagées), valeurs négatives ou excessives refusées avant
lecture/ouverture. Run appelle PrepareFollowResumeWithPolicy avec ces budgets et
Config.ResumePolicy. Toute erreur
de préparation bloque. Un résultat ready utilise le cœur d'application commun avec
FollowOpened sans réacquérir le verrou. Fermeture du propriétaire sur toute sortie
avant transfert ; après transfert, propriétaire vide et nettoyage par scheduler.
LastPathStatus est réinitialisé à chaque Run accepté, même si la préparation bloque.

Seul absent après parcours complet sans générations en suivi permet le démarrage
du fichier courant existant, avec décision/acquisition et attente du fichier neuf
vide. Aucune sélection bloquante n'est convertie en démarrage neuf. Unknown bloque
la reprise, même avec AllowZeroCheckpoint ; la localisation stricte d'un ensemble
en suivi refuse aussi nil/zéro. La politique explicite de replay zéro reste applicable
au démarrage courant sans ensemble en suivi, par exemple un courant explicitement
retiré. Une unique génération en suivi à zéro peut aussi être reprise dans les
conditions ci-dessous. La récupération d'un état inconnu reste à développer ;
aucun reclassement implicite de lifecycle.

La reprise known/new suit aussi les ajouts tardifs des fichiers renommés. Ancres,
polling/grâce/retrait durable et erreurs du Sink restent ceux du scheduler. Un
courant missing au redémarrage bloque, tandis qu'une disparition pendant un suivi
déjà établi conserve le descripteur. Contrôles, pages et écritures non atomiques ;
écritures d'état à sérialiser par source entre tous les objets.

## Relecture zéro explicite d'une génération en suivi unique

PrepareFollowResume reste strict. PrepareFollowResumeWithPolicy autorise, seulement
avec AllowZeroCheckpoint, un parcours lifecycle complet contenant exactement une
génération en suivi, avec checkpoint présent à zéro. Les retirés restent écartés ;
inconnus, états invalides et limites gardent leurs diagnostics bloquants.

Cette branche ouvre uniquement le chemin courant configuré, sans recherche dans
les archives. VerifyCandidateWithPolicy doit rendre restart_zero : identité physique
concordante, préfixe non vide concordant et checkpoint/ancre zéro canoniques de la
même origine. NewIngestor revérifie puis se positionne à zéro sans lire de ligne.
Le courant observé doit toujours être connu et concordant. Courant absent :
ErrCurrentMissing ; remplacement observé : ErrPathChanged ; preuve insuffisante ou
divergente : ResumeDecisionError/insufficient ou different. Aucun diagnostic de
lacune de recherche n'est inventé ici. Échec/annulation ferme l'ouverture, résultat
vide ; ready fournit un seul propriétaire complet, sans écriture d'état.

Le propriétaire conserve une copie du préfixe validé. Avant transfert et dans les
contrôles du scheduler tant que le checkpoint acquitté reste zéro, le préfixe est
revérifié par ReadAt (4096 octets au plus, sans seek). Divergence avant transfert
conserve le propriétaire et les offsets ; après transfert, arrêt et fermeture par
le scheduler. Dès le premier acquittement positif, les contrôles ordinaires de taille
et d'ancre prennent le relais. Aucune registration/acquisition répétée, reset de
checkpoint/provenance ni transition inventée. Un arrêt après acquisition avant
première ligne peut ainsi relire explicitement depuis zéro, puis reprendre strictement
ses futurs ajouts depuis le checkpoint positif.

Les ensembles multiples avec checkpoint zéro, nil, anciennes empreintes vides,
preuves invalides, fichiers divergents et lifecycle inconnus restent bloqués. La
relecture explicite ne prouve pas une continuité par ancre positive. Préfixes bornés,
lectures et écritures non atomiques : une modification hors fenêtre ou entre deux
contrôles peut échapper à la vérification. Les fonctions de localisation/réouverture
standalone gardent leur politique stricte.

## Lacune de reprise d'une génération en suivi

Dans PrepareFollowResume/Run, un parcours de localisation terminé sans génération
vérifiée (SelectionAbsent ou SelectionDifferent) devient FollowResumeGapError.
ErrFollowResumeGap identifie le diagnostic via errors.Is ; ResumeDecisionError
reste accessible via errors.As avec son status absent/different. Message fixe
sans ID d'origine, chemin, offsets ni contenu. Les sélections standalone gardent
leurs résultats existants ; classification seulement pour un ensemble persisté
en suivi, jamais pour un parcours lifecycle vide ou entièrement retiré.

Ce diagnostic signifie continuité vérifiée indisponible dans le répertoire/budget
observé. Il ne prouve ni suppression du fichier ni perte ou nombre de messages :
un fichier présent mais réécrit peut aussi donner different. Gzip/liens/exclusions
de recherche et fenêtres bornées restent applicables. Insufficient/ambiguous/limit
ou erreur filesystem ne sont pas transformés en lacune confirmée ; leur cause
reste distincte. Courant missing après localisation réussie reste ErrCurrentMissing.

Arrêt sans lire le nouveau courant ou seulement les fichiers encore disponibles,
sans reset, nouveau checkpoint/origine ni retrait artificiel. État durable et
provenance conservés ; descripteurs temporaires de recherche fermés. Une restauration
de la même archive, revérifiée à la reprise suivante, permet de reprendre ses ajouts
depuis le checkpoint conservé avec le scheduler normal. Pas d'enregistrement durable
de la lacune, aucune récupération automatique ni preuve atomique de continuité.

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
acquittée, le checkpoint reste zéro ; la reprise exige une décision explicite de
lifecycle ainsi que la politique `AllowZeroCheckpoint` lorsqu'elle est applicable.
Run ne contourne pas un état inconnu/en suivi insuffisant. Les métadonnées de
vérification précèdent l'acquisition ; aucune ligne n'est consommée avant elle.
