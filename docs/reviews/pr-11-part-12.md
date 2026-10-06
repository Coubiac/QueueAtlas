# Revue de la PR #11 — partie 12 : localisation de l'ensemble en suivi

Revue du 4 octobre 2026 sur la tête publiée
`e95f84ecc53096bd8af11d67356bf78075f9a103`. Coordinateur et auditeur agent
indépendant en lecture seule. Périmètre : LocateFollowOrigins et tests ; SelectRotation
réutilise sa revue partie 11. Réouverture, transfert et Run exclus. Données synthétiques.

## Résultat

Aucun blocage concret identifié par les deux revues. Code inchangé et aucun nouveau
test sans défaut concret à reproduire. Comportement conforme à ADR-009.

- Ensemble complete de 1–2 following, IDs non vides/distincts et chemin enregistré
  exact validés avant recherche. Checkpoints copiés avant les scans, nil/zéro et
  autres métadonnées conservés sans reprise implicite.
- Budget partagé de 1 à 2000 entrées examinées ; chaque SelectRotation reçoit
  au plus 1000 et le budget restant. Entrées répétées et exclues comptées ; budget
  épuisé avant le suivant donne limit sans recherche supplémentaire.
- Recherches séquentielles dans le répertoire du chemin configuré résolu en absolu.
  La première décision non unique est conservée dans l'ordre fourni, sans priorité
  globale des causes et sans chemins partiels. Collision de chemin : ambiguous.
- Unique et locations seulement si toutes les origines sont localisées à des chemins
  distincts. L'ordre ne choisit pas le courant ni ne prouve la capacité finale avec
  un nouveau courant. Erreur/annulation efface tout le résultat, cause conservée.
- Descripteurs temporaires possédés et fermés par SelectRotation ; aucun descripteur
  durable, record, checkpoint ou état de suivi modifié par la localisation.

## Vérifications

Coordinateur et auditeur : quatre tests portables TestLocateFollowOrigins*
-count=1 réussis sous Windows. git diff --check réussi ; checkout isolé propre
et référence confirmée. Budgets exact/insuffisant/plafond par scan, absence de chemins
partiels après première réussite, entrées invalidées avant recherche, erreur et
annulation, copies indépendantes et collision de chemin couverts.

Tests Linux relus : 1–2 fichiers courant/renommé réellement localisés, métadonnées
inchangées, absence/réécriture/nil/zéro/hard links/génération partagée/limite et
fermeture de tous les fichiers/répertoire. Pas d'exécution locale Linux sous Windows.
La [CI de la référence revue](https://github.com/Coubiac/QueueAtlas/actions/runs/37220323146)
a réussi : tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64
sans CGO et job Windows chemins. Les intégrations Linux y sont exécutées.
Ce lot modifie uniquement la documentation ; consulter la PR #11 pour sa CI de publication.

## Limites et suite

Namespace de source assuré par l'appelant, écritures sérialisées jusqu'à application.
Scans et preuves bornées ne forment pas un snapshot atomique : chaque chemin doit
être rouvert et revérifié. Le budget compte les entrées examinées, pas une garantie
de temps réel ; SelectRotation peut lire une entrée supplémentaire pour établir la fin.
Reprise persistante actuellement Linux seulement.

PR #11 en brouillon. Prochain petit lot : OpenFollowLocations/OpenedFollowSet,
validation avant ouverture, réouverture et vérification de tout l'ensemble, propriété
et fermeture sur échec. Observation/transfert, préparation orchestrée et Run seront
relus séparément. Audit assisté par agents, sans certification humaine externe.
