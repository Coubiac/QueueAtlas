# Revue de la PR #11 — partie 8 : courant et successeurs aux polls

Revue du 4 octobre 2026 sur la tête publiée
`82635d7e27fa3da5447a6e23fd27925a82f417aa`, basée sur main après fusion de #10.
Coordinateur et auditeur agent indépendant en lecture seule. Périmètre : poll dans
rotation.go, contrôles taille/ancre associés, observation du chemin, choix du courant,
ouverture des successeurs, capacité et propriété sur erreur. Tests synthétiques.
Ordonnancement global des lectures/polls et reprise orchestrée/Run exclus de ce lot.

## Résultat et garanties relues

Aucun blocage concret identifié par les deux revues. Code inchangé et aucun nouveau
test sans défaut concret à reproduire.

- Chaque poll contrôle les tailles des générations ouvertes, puis leurs ancres,
  avant observation du chemin, retrait ou bascule. La taille est comparée à l'offset
  consommé, y compris les partiels, sans prendre la lecture anticipée pour une perte.
  L'ancre porte sur le dernier checkpoint acquitté ; un zéro ordinaire n'a pas de
  preuve, tandis qu'un replay zéro explicite conserve son contrôle de préfixe.
- same et missing conservent le courant et son ingestor. Une disparition ne crée
  pas un successeur. Si le chemin désigne une génération déjà conservée, celle-ci
  redevient courante sans nouvelle ouverture, registration ou perte de ligne partielle.
  Sa grâce est réinitialisée avant les retraits.
- Le retrait des anciens fichiers admissibles précède le contrôle de capacité.
  Le plafond de deux générations ouvertes est vérifié avant OpenLog : une troisième
  génération ne peut pas être ouverte tant que les deux places restent occupées.
- OpenLog vérifie l'ouverture ; le poll compare ensuite l'identité retournée à
  l'observation précédente. Un changement détecté ferme le nouveau descripteur et
  retourne ErrPathChanged, avec l'erreur de fermeture éventuelle jointe.
- Un successeur accepté rejoint la collection possédée avant sa préparation.
  Décision insuffisante, erreur de registration/acquisition/record, annulation ou
  erreur de contrôle arrêtent le suivi ; le nettoyage ferme les descripteurs restants.
  La préparation/acquisition et le retrait ont leurs revues distinctes, parties 6–7.
- LastPathStatus conserve la dernière observation réussie et peut être périmé.
  replaced reste une observation relative au précédent courant même lorsqu'un
  descripteur conservé est réutilisé ; ce statut ne certifie pas l'état du suivi.

## Vérifications

Coordinateur sous Windows :
`go test ./internal/source/file -run '^(TestSourcePathFollow|TestPollStopsOnCurrent|TestSizeCheck|TestLiveAnchor)' -count=1`
réussi. Auditeur : tests portables ObservePath, SourcePathFollow, tailles/troncature
et ancres réussis. git diff --check réussi ; checkout isolé propre et tête confirmée.

Tests Linux relus : bascule avec checkpoints distincts et ancien descripteur retenu,
successeur vide sans registration, capacité avant troisième fichier, échecs du Sink
sans réessai/fuite, retour du retenu avec sa ligne partielle, décision du successeur
insuffisante, chemin disparu/restauré, FIFO/boucle de lien, troncature ou réécriture
d'ancre du retenu avant retrait/ouverture de la troisième génération.
Ces tests ne s'exécutent pas localement sous Windows.

La [CI de la référence revue](https://github.com/Coubiac/QueueAtlas/actions/runs/37203882023)
a réussi : tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64
sans CGO et job Windows ciblé chemins. Les intégrations Linux y sont exécutées.
Ce lot modifie uniquement la documentation ; consulter la PR #11 pour la CI de
publication du rapport.

## Limites et suite

Les snapshots, contrôles et ouvertures ne sont pas atomiques ; aucun verrou ne
garantit la stabilité future du chemin. Préfixes/ancres bornés, mutations hors fenêtre
ou entre contrôles et troncature suivie de croissance peuvent échapper au diagnostic.
Le plafond concerne les générations suivies, pas tous les handles du processus,
notamment les handles temporaires de métadonnées Windows. Sink durable et écritures
sérialisées par source requis. Pas d'injection locale du changement entre observation
et ouverture ; branche de refus relue et tests OpenLog déjà revus dans la partie 3.

PR #11 en brouillon. Prochain petit lot : ordonnancement des lectures conjointes,
polls sous flux continu, attente et priorité des erreurs ; reprise orchestrée et Run
seront traités ensuite. Audit assisté par agents, sans certification humaine externe.
