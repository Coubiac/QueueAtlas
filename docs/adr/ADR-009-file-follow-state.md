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
implémentés ; préparation des candidats implémentée, localisation/vérification
et raccordement à la reprise de Run restent à développer.

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
