# Revue de la PR #11 — partie 13 : réouverture de l'ensemble localisé

Revue du 4 octobre 2026 sur la tête publiée
`a04afbb42fa93a678a4316017ba8b019f0a70894`. Coordinateur et auditeur agent
indépendant en lecture seule. Périmètre : OpenFollowLocations/OpenedFollowSet et
tests ; OpenLog, VerifyCandidate et NewIngestor réutilisent leurs revues précédentes.
Observation, transfert et Run exclus. Données synthétiques uniquement.

## Résultat

Aucun blocage concret identifié par les deux revues. Code inchangé et aucun nouveau
test sans défaut concret à reproduire. Comportement conforme à ADR-009.

- Validation et copie de tout l'ensemble avant accès disque : source file et
  normaliseur, unique de 1–2 following, IDs non vides/distincts, même chemin d'origine
  non vide, chemins sélectionnés absolus et distincts après nettoyage lexical.
- Chaque descripteur rejoint immédiatement son propriétaire après ouverture,
  avant contrôle d'annulation, collision ou preuves. Une collision physique est
  ambiguous même avec des chemins différents. Aucun premier fichier ignoré lors
  d'un échec sur le second.
- OpenLog, VerifyCandidate strict et NewIngestor vérifient identité/préfixe/ancre/LF
  puis préparent la lecture au checkpoint positif. Nil/zéro ne permettent pas de
  replay implicite. ReadAt de preuve et Seek autorisés, aucune consommation de
  ligne, normalisation ou écriture d'état pendant la préparation.
- Échec ou annulation ferme toutes les ouvertures acquises et rend nil, avec cause
  initiale et erreurs de nettoyage conservées. Pas de réessai automatique.
- Succès : ensemble opaque propriétaire jusqu'à Close ou transfert ultérieur.
  Close vide d'abord la collection, ferme tous les fichiers malgré une erreur et
  reste idempotent ; valeur zéro et récepteur nil acceptés, état durable inchangé.

## Vérifications

Coordinateur et auditeur : les deux tests portables OpenFollowLocations et
OpenedFollowSet -count=1 réussis sous Windows. Validation entière avant ouverture,
source/normaliseur/annulation, Close de tous les fichiers malgré une erreur et
idempotence/nil/zéro couverts. git diff --check réussi ; checkout isolé propre et
référence confirmée.

Tests Linux relus : réouverture réelle de 1–2 fichiers après append, offset et
checkpoint conservés sans normalisation, lecture seule ; copie du second checkpoint
avant première ouverture ; second fichier absent/remplacé/réécrit/tronqué/nil/zéro/
ancre invalide, collision physique, erreur/annulation et cause de cleanup sans fuite
ni ensemble partiel. Pas d'exécution locale Linux sous Windows.
La [CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37220607670)
a réussi : tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64
sans CGO et job Windows chemins. Les intégrations Linux y sont exécutées.
Ce lot modifie uniquement la documentation ; consulter la PR #11 pour sa CI de publication.

## Limites et suite

Namespace de source fourni par l'appelant et écritures sérialisées jusqu'à application.
Preuves bornées et vérifications non atomiques : aucune garantie contre une écriture
future. Propriétaire à ne pas copier ni utiliser concurremment. Reprise persistante
actuellement Linux seulement ; aucun crash réel ajouté dans ce lot.

Chantier FileSource du jalon M2 : réouverture relue ; restent observation du courant,
transfert, préparation/reprise orchestrée et Run, puis validation finale et fusion.
Critère de fin : chemins restants revus, défauts corrigés, CI verte et PR #11 fusionnée.
Prochain petit lot : ObserveCurrent, décision known/missing/new/capacity sans
consommation ni transfert de propriété. PR en brouillon. Audit assisté par agents,
sans certification humaine externe.
