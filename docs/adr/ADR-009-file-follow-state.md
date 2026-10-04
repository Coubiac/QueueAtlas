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

Le futur scheduler devra déclarer une acquisition après vérification du fichier et
acquitter un retrait après EOF stable/grâce, avant de fermer le descripteur. Un
arrêt, une annulation ou une erreur de suivi ne devra pas inventer un retrait.
La reprise devra distinguer les états connus de ceux restés inconnus.

Les lecteurs par identité et chemin exposent l'état dans la même page que le
checkpoint. Les empreintes et checkpoints restent conservés lors d'un retrait.
Le présent lot implémente le contrat, la migration et le Sink ; publication par le
scheduler et reprise automatique restent des lots distincts.

## Limites

Le stockage n'observe ni descripteur, EOF, grâce ni empreinte : l'appelant justifie
la transition. Retiré ne prouve pas que le fichier ne recevra jamais d'ajout. La
réacquisition exige une vérification explicite sans reset de checkpoint. Un seul
écrivain doit sérialiser acquisition, retrait et réessais pour une source ; ce
protocole ne comporte pas d'époque de propriétaire ou de protection contre des
réessais obsolètes après un cycle complet de réacquisition.
