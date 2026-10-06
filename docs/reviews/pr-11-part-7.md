# Revue de la PR #11 — partie 7 : retrait durable et grâce

Revue du 4 octobre 2026 sur la tête publiée
`f468a7ad6e2327be58ce0c074b29b1491a03fed3`, basée sur main après fusion de #10.
Coordinateur et auditeur agent indépendant en lecture seule. Périmètre : grace.go,
tests grace*.go/retirement_linux_test.go et appels du scheduler nécessaires au
retrait et à la propriété des descripteurs. Données synthétiques uniquement.

## Résultat et garanties relues

Aucun blocage concret identifié par les deux revues. Code inchangé et aucun nouveau
test sans défaut concret à reproduire.

- La grâce démarre à EOF pour une génération conservée à une frontière complète.
  Les EOF répétés conservent son début. Le courant, les lignes partielles, même
  surdimensionnées, et un batch non acquitté restent protégés.
- Un progrès acquitté réinitialise l'attente. À échéance, frontière et taille sont
  revérifiées : un ajout intermédiaire empêche la fermeture et renouvelle la grâce
  après consommation et nouvel EOF. Les contrôles de taille et d'ancre du poll
  précèdent le retrait ; une troncature observée arrête le suivi.
- Le retrait d'une origine enregistrée acquitte seulement following → retired
  avant fermeture et libération de capacité. Origine, provenance, checkpoint et
  observations restent inchangés. Un ancien fichier vide non enregistré est fermé
  sans transition ; le courant reste protégé même quand le chemin est absent.
- Erreur du Sink, y compris EOF ou conflit, et annulation avant acquittement arrêtent
  le retrait. Aucun successeur supplémentaire n'est ouvert après cet arrêt.
  Une réponse perdue ne prouve pas l'absence de commit durable.
- Après acquittement, annulation ou erreur de fermeture conservent retired durable.
  La fermeture retire le descripteur de la collection avant de retourner son erreur,
  évitant une seconde fermeture par le nettoyage. Si l'annulation précède Close,
  le scheduler possède encore le descripteur et le ferme au retour.

## Vérifications

Coordinateur et auditeur : les trois tests portables TestGrace* -count=1 réussis
sous Windows, couvrant pending/réessai/annulation, fichier vide sans transition et
erreur de fermeture après acquittement. git diff --check réussi ; checkout isolé
propre et référence confirmée par l'auditeur.

Tests Linux d'intégration relus : échéance exacte et réutilisation de capacité pour
la troisième génération ; ajout tardif et renouvellement ; partiels ordinaires et
surdimensionnés ; courant absent ; erreurs/EOF/conflit/annulation avant et après
acquittement et Close défaillant sans fuite ni modification de checkpoint.
Ils ne s'exécutent pas localement sous Windows.

La [CI de la référence revue](https://github.com/Coubiac/QueueAtlas/actions/runs/37203460955)
a réussi : tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64
sans CGO et job Windows ciblé chemins. Les intégrations Linux y sont exécutées.
Ce lot modifie uniquement la documentation ; consulter la PR #11 pour la CI de
publication du rapport.

## Limites et suite

Sink durable et écritures/réessais sérialisés par source requis. Le dernier contrôle,
la transaction de retrait et la fermeture ne sont pas atomiques : un ajout après
le dernier contrôle, notamment pendant Commit, peut être manqué. La grâce borne
l'attente, elle ne garantit pas l'absence de toute écriture future sur l'ancien inode.
Pas de crash réel ou nouveau scénario concurrent injecté dans ce lot.

Rotation globale, ordonnancement et reprise orchestrée/Run restent à relire ; PR #11
en brouillon. Prochain petit lot : sélection du courant et ouverture des successeurs
lors des polls, capacité et propriété sur erreur. L'ordonnancement et Run seront
traités séparément. Audit assisté par agents, sans certification humaine externe.
