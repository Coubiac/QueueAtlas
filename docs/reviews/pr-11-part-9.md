# Revue de la PR #11 — partie 9 : ordonnancement et flux continu

Revue du 4 octobre 2026 sur la tête publiée
`337ceb56f0cfc28cef868bf52cb5ab19b28fb3c0`. Coordinateur et auditeur agent
indépendant en lecture seule. Périmètre : boucle de suivi de rotation.go,
continuous_poll/joint_follow, attente/erreurs de Follow et interaction avec
LineReader/CommitNext. Reprise orchestrée et Run exclus. Données synthétiques.

## Défaut corrigé

Un appel CommitNext pouvait consommer une quantité illimitée de fragments avant LF
ou EOF. Une ligne physique très longue ou alimentée sans LF pouvait monopoliser
le passage, empêchant la progression de l'autre génération et les polls. Le cadrage
phase 0 exige de ne pas bloquer indéfiniment sur une ligne trop longue (§ FileSource).
L'équité par ligne ne suffisait donc pas.

La nouvelle régression portable reproduit le défaut avant correction : le contrôle
d'une ancre modifiée attendait 196608 octets partiels, au lieu du budget 65536.
Elle passe après correction. L'auditeur a révisé son constat initial à la lumière
de ce risque et relu indépendamment le patch, sans blocage restant identifié.

- Le scheduler utilise un chemin interne limité à 16 fragments de 4096 octets,
  soit au plus 64 Kio consommés par génération et passage et au plus un record.
  Un yield partiel marque une progression de lecture, sans EOF ni commit : autre
  génération puis contrôle de l'échéance peuvent avancer sans attendre LF.
- Fragments, start/offset, préfixe surdimensionné et fenêtre d'ancre sont conservés
  entre yields. Ni normalisation ni checkpoint avant la ligne complète. LF et
  erreur réelle gardent leur traitement normal. Un batch pending est prioritaire ;
  une erreur du Sink ne peut pas être reclassée en yield car pending reste présent.
- Next et CommitNext publics conservent leur contrat de record complet. La limite
  de passage est propre à FileSource, pas une nouvelle erreur publique à gérer.
- Les commits restent séquentiels et les checkpoints distincts. Le scheduler
  attend seulement sans progression, pour le temps restant avant le prochain poll ;
  flux continu et fragments progressant n'empêchent plus ce contrôle entre passages.
- Erreur de lecture ou du Sink, dont EOF avec pending, arrête avant génération
  suivante, attente ou contrôle supplémentaire. Aucun réessai automatique.
  Annulation entre générations et nettoyage des descripteurs restent appliqués.

## Vérifications

Avant patch : TestContinuousPollBoundsPartialReadBeforeAnchorCheck échoue comme
attendu, avec 196608 octets consommés. Après patch : cette régression, le test de
ligne surdimensionnée répartie sur plusieurs yields (annulation, EOF du Sink,
batch identique au réessai, checkpoint et ancre), IdleRound et Follow réussissent
sous Windows avec -count=1. L'auditeur a également exécuté les régressions ciblées.
go test ./..., go vet ./..., git diff --check et compilation des tests FileSource
Linux amd64 sans CGO réussis localement. Pas d'exécution locale des tests Linux.

Tests Linux relus : ordre conjoint, ajout tardif/partiel, successeur vide, erreur du
retenu arrêtant avant le successeur, rotation avant EOF de l'ancien, expiration sous
flux continu et EOF du Sink avant poll dû. Nouveau test : ancien avec longue ligne
partielle cédant au record du successeur avant LF, checkpoint ancien inchangé et
aucune fuite. Ce nouveau test a été exécuté par la CI du correctif.

La [CI de la référence initiale](https://github.com/Coubiac/QueueAtlas/actions/runs/37204155236)
était verte ; elle ne valide pas ce correctif. Le correctif publié
`cd931e08886677511b8521b32553c3714c0d3019` a sa
[CI réussie](https://github.com/Coubiac/QueueAtlas/actions/runs/37204731135) :
tests/vet Linux Go 1.26.x/stable, nouveaux scénarios Linux, race FileSource, builds
Linux amd64/arm64 sans CGO et job Windows chemins. Lot clos sur cette référence.
Consulter la PR #11 pour la CI de publication de cette clôture documentaire.

## Limites et suite

Le budget borne la quantité consommée, pas la durée d'un syscall, du normaliseur ou
du Sink. L'échéance est contrôlée entre passages, sans garantie temps réel. Lectures
anticipées, preuves filesystem bornées/non atomiques et écritures sérialisées par
source restent soumises aux contrats déjà revus. Pas de benchmark de débit ni de
crash réel ajouté. PR #11 en brouillon.

Prochain petit lot : classification des états durables de reprise par LoadFollowOrigins,
budgets et refus sans ensemble partiel. Recherche/réouverture/transfert, préparation
orchestrée et Run seront relus séparément. Audit assisté par agents, sans
certification humaine externe.
