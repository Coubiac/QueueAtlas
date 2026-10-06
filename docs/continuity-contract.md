# Continuité entre origines — lot 120

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
et `git diff --check` réussis. Les fondations SQLite/rétention ne sont pas relancées :
aucun de leurs comportements n'est changé. La CI de publication reste à vérifier.

## Prochaine étape

Intégrer séparément les attestations à la reconstruction pure et à ses clés
révisables, avec réserves explicites. Avant toute utilisation applicative ou
persistance d'un plan, définir le producteur fiable et sa revalidation. Tant que
ces propriétés ne sont pas établies, garder les origines distinctes. Le MVP
conservateur peut afficher cette incertitude ; le contrat ne promet pas une
certification automatique des rotations/imports.
