# Continuité entre origines — lots 120–121

## Résultat livré

`CheckContinuityClaims` contrôle la cohérence d'attestations explicites pour un
snapshot de faits. Il rend un plan lié à une révision ou une erreur sans résultat
partiel. Il ne produit pas la preuve physique d'une rotation et ne fusionne pas
les générations. Les projections et le schéma SQLite existants restent inchangés.

Le suivi FileSource actuel peut observer un remplacement et lire conjointement
l'ancien et le nouveau fichier. Cela ne garantit ni la fin définitive de l'ancien,
ni l'absence de fichier intermédiaire ou de trou. `FollowRetired`, les noms, dates,
PID, Queue ID, Message-ID ou textes identiques ne constituent pas une attestation.
Un import complet ne prouve pas davantage sa succession avec un fichier suivi.

## Frontière de confiance

`ContinuityClaims.InputRevision` est la révision complète des faits du lot 103.
Chaque `OriginBoundary` contient deux références physiques exactes `From` et `To`.
Le producteur affirme que `From.End` est la fin définitive de son origine et que
`To.Start == 0` est le début de son successeur direct, sans données ou origine
intermédiaires omises. Ce producteur doit établir ces propriétés indépendamment,
avec une autorité et des preuves de collecte connues de l'application.

La fonction ne reçoit ni fichiers ni certificat signé : elle ne peut vérifier
cette affirmation dans le monde réel. Un appelant peut soumettre une affirmation
fausse mais cohérente. Le plan n'est donc ni un jeton de preuve, ni une permission
de transformer une donnée distante en certitude. Aucun producteur automatique
d'attestations ni raccordement applicatif n'est livré dans ce lot. L'absence de
producteur fiable laisse la continuité incertaine dans le fonctionnement courant.

## Contrôles et limites

- Validation habituelle de tout le snapshot par `PartitionFacts`, limite explicite
  de 1 à 4096 faits et aucun chevauchement de positions dans une origine.
- Au plus 256 frontières ; révision exacte obligatoire, même pour un plan vide.
  Une observation modifiée ou un import tardif invalide les anciennes attestations.
- Origines distinctes, même source et même instance configurée. Les associations
  entre une source import et une source live sont exclues de ce contrat initial.
- Références exactes présentes : dernière référence observée de l'origine gauche,
  première référence observée de la droite, dont l'offset initial vaut zéro.
  Tous les faits comptent, y compris inconnus et NOQUEUE. Un périmètre de recherche
  filtré ne devient pas une attestation de collecte complète.
- Au plus un successeur et un prédécesseur par origine ; doublons, embranchements,
  jonctions et cycles refusés. Plusieurs chaînes séparées restent séparées.

Les extrémités du snapshot ne prouvent ni sa couverture, ni l'absence de bytes
omises dans une origine, ni sa fermeture. La fonction n'infère rien des horloges,
des contenus ou de l'ordre d'arrivée. Une limite de nombre de faits/frontières
n'est pas une limite indépendante de bytes de métadonnées.

## Révision et propriété

Le plan copie et trie les frontières par source/origine gauche, sans donner un
sens temporel à cet ordre d'affichage. Sa révision SHA256 utilise le domaine
`continuity-claims-v1`, la révision d'entrée, le nombre de frontières et leurs
références complètes, avec cadrage par longueur du lot 103. L'ordre des faits ou
attestations ne change pas la révision ; un changement de frontière la change.
Ce digest versionne les données, sans authentifier le producteur. Faire évoluer
son domaine si les règles de validation ou le cadrage changent.

L'appelant conserve les observations immuables pendant l'appel. La sortie possède
sa liste ; sa modification ne change pas les entrées ni une autre reconstruction.
Les erreurs fixes ne contiennent ni noms d'origine ni données du journal :
`ErrContinuityClaims`, `ErrContinuityStale`, ou les erreurs de provenance/limite
existantes, sans plan partiel.

## Vérifications du lot

Six tests Windows ciblés réussis : ordre/révision/propriété, graphes contradictoires,
frontières et namespaces exacts incluant NOQUEUE, fraîcheur/limites/provenance,
absence d'inférence/fusion sur le corpus synthétique d'ID recyclés et chaîne de
256 frontières avec séparation en deux chaînes. La suite complète
`go test ./internal/correlation -count=1`, `go vet ./internal/correlation`
et `git diff --check` réussis. Les fondations SQLite/rétention ne sont pas relancées
localement : aucun de leurs comportements n'est changé. Publication dans
[PR #28](https://github.com/Coubiac/QueueAtlas/pull/28) sur
16784eb297dfc0aead2212e63c13bb828899f936 ;
[CI37537740519](https://github.com/Coubiac/QueueAtlas/actions/runs/37537740519)
entièrement réussie, trois jobs Windows/stable/Go1.26 vérifiés sur cette tête exacte.
La CI exécute la suite complète requise. La PR reste ouverte pour la suite du chantier.

L'enregistrement documentaire120 ec56b35e50d14dc2ac167a5a676f93bcdb68b28a a aussi
passé la [CI37537911013](https://github.com/Coubiac/QueueAtlas/actions/runs/37537911013)
entière, trois jobs réussis sur cette tête exacte.

## Lot121 : clés liées au contexte d'attestation

`BuildQueueInstancesWithContinuity(facts, limit, claims)` rend un
`ContinuityInstances` contenant le plan contrôlé et une partition de candidats.
La fonction recontrôle les attestations sur le snapshot ; elle n'accepte pas un
plan fabriqué par l'appelant comme jeton de validation.

La révision des clés utilise le domaine `queue-instances-with-continuity-v1` et
la révision du plan120, avec le cadrage par longueur existant. Elle versionne donc
tous les faits et toutes les frontières. Modifier ou retirer une attestation
invalide toutes les anciennes clés, même si les faits et ordinaux restent
identiques. Réordonner les mêmes entrées conserve les clés. Faire évoluer le
domaine si les règles d'identité sous ce contexte changent.

Une nouvelle observation exige une réattestation explicite par l'appelant :
l'ancienne révision est refusée, sans sortie partielle. Un contexte explicite vide
reçoit lui aussi une révision distincte de la voie ordinaire et de la révision du
plan ; ne pas mélanger ces namespaces de clés. Garder le plan avec la partition.

Le plan ne fusionne aucun candidat et ne réordonne pas les ordinaux selon la
succession déclarée. Générations, ancres, retrait observé, réserves de date et
`CrossStreamUncertain`, flux non résolus et autres faits sont conservés. Même un
cycle coupé entre deux origines demeure deux candidats sous ce contexte, en
l'absence de producteur fiable et de règles de fusion justifiées. Les observations
et attestations doivent rester immuables pendant l'appel ; les références, listes
et pointeurs de sortie appartiennent au résultat.

Cette API pure n'est raccordée ni à `BuildProjection` ni aux manifests SQLite.
La voie existante `BuildQueueInstances` conserve ses révisions et comportements.
Aucun jeton de preuve, conclusion de livraison, déduplication ou réparation de
couverture n'est ajouté. Les erreurs120/provenance/limites sont conservées, avec
résultat entièrement vide sur refus.

Cinq tests Windows121 et suite complète de corrélation réussis, ainsi que vet,
format et diff : clés contextuelles sans fusion sur un cycle synthétique coupé,
changement d'attestation, fait tardif/réattestation, permutations/propriété avec
NOQUEUE/non résolu, refus/limites et snapshot vide. Fondations SQLite/rétention
non relancées localement, puisque leur code et leurs entrées publiques ne changent
pas. Lot121 publié720a0544e7674f18ab3a159cb1e7cbea8b83f24f dans #28 ;
[CI37540913337](https://github.com/Coubiac/QueueAtlas/actions/runs/37540913337)
entièrement réussie sur cette tête, trois jobs vérifiés REST.

## Prochaine étape

Lot122 : bilan/relecture et clôture de la PR du contrat et des clés120–121, après
CI finale entière, puis validation d'ensemble M3. Avant toute utilisation applicative ou
persistance d'un plan, définir le producteur fiable et sa revalidation. Tant que
ces propriétés ne sont pas établies, garder les origines distinctes. Le MVP
conservateur peut afficher cette incertitude ; le contrat ne promet pas une
certification automatique des rotations/imports.
