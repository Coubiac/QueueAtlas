# Revue de la PR #11 — partie 3 : ouverture et observation du chemin

Revue du 4 octobre 2026, référence initiale
`25447608b60b1f0c6abdf96c9a7d3abdd215b3f4`, basée sur main après fusion de #10.
Coordinateur et auditeur agent indépendant en lecture seule. Périmètre : open*.go,
path.go, source_path.go et leurs tests. Données synthétiques uniquement.

## Constat et correction

**P2 — snapshot d'identité Windows chargée trop tard** : Go peut conserver un
chemin dans le FileInfo d'os.Stat puis le rouvrir pour charger les IDs lors de
os.SameFile. Après stat(A), fermeture/rename(A), création de B au même chemin et
ouverture de B, validateOpen pouvait donc accepter B contre le snapshot supposé
de A. La régression déterministe échouait avant le correctif.

OpenLog et ObservePath utilisent désormais statPath. Hors Windows, ce helper
conserve os.Stat. Sous Windows, il ouvre un handle de métadonnées sans accès aux
données, avec partage READ/WRITE/DELETE, suit les cibles des liens puis fait
File.Stat : attributs et IDs proviennent du même handle. Le handle temporaire est
fermé avant le retour, y compris si Stat échoue ; erreurs de fermeture conservées.
Les chemins courts gardent le namespace normal, les chemins longs drive/UNC
sont étendus si nécessaire, et les préfixes explicites sont conservés.

La régression refuse désormais le remplacement et vérifie la fermeture du
descripteur rejeté. Un test réel sur un chemin supérieur à 300 caractères, avec
et sans préfixe explicite, vérifie OpenLog et ObservePath. Un job CI Windows
ciblé exécute ces régressions et les tests portables d'ouverture/observation.
Aucune dépendance ou format persistant modifié. Relecture indépendante du patch :
aucun autre blocage concret identifié dans ce périmètre.

## Garanties relues

- Descripteur retourné en lecture seule, offset zéro, fichier régulier et identité
  comparée avant ouverture, sur le descripteur puis sur le chemin courant.
  Toute validation échouée ferme le descripteur possédé par OpenLog.
- Cibles régulières des liens acceptées ; remplacement ou disparition détectés
  pendant la validation refusés. Erreur de disparition conservée avec ErrPathChanged.
  FIFO connue refusée ; Linux O_NONBLOCK empêche l'attente d'un écrivain si une FIFO
  remplace le fichier entre preflight et open. Régression Linux déterministe existante.
- ObservePath distingue same/missing/replaced ; lien pendant : missing, boucle ou
  autre erreur : erreur sans observation utilisable, type non régulier refusé.
  Il ne lit aucune donnée, ne déplace pas et ne ferme pas le descripteur de l'appelant.
  Sous Windows, son snapshot du chemin utilise le handle temporaire décrit ci-dessus.
- Statut LastPathStatus protégé par RWMutex, mis à jour seulement après une
  observation réussie ; il reste un diagnostic pouvant être périmé. Les appels de
  reset sont mutex protégés ; leur ordre dans Run/FollowOpened reste à revoir avec
  l'orchestration. Aucun jugement global sur le scheduler dans cette partie.

## Vérifications

Régression Windows reproduite sur la référence initiale, puis réussie avec le
correctif. Coordinateur et auditeur : tests ciblés Windows `-count=1` réussis,
incluant substitution et chemin long. Coordinateur : go test ./..., go vet ./...,
git diff --check et compilation des tests FileSource Linux amd64 sans CGO réussis.
La [CI du correctif](https://github.com/Coubiac/QueueAtlas/actions/runs/37202322863)
réussit sur `cded0a87fd22f96220d3f519cb70b85f55f2cf3b` : nouveau job windows-path
(substitution et chemin long exécutés), Linux Go 1.26.x/stable, tests/vet,
race FileSource et builds amd64/arm64 sans CGO. Consulter la PR #11 pour la CI
de publication du point de reprise final.

## Limites et suite

Snapshots non atomiques : aucune immobilisation du chemin ni preuve de génération
complète. Annulation vérifiée entre appels ; pas d'interruption d'un syscall
bloquant. Le nonblocage lors d'une substitution FIFO est garanti ici pour Linux
seulement. UNC distant et liens Windows n'ont pas été testés localement ; les liens
et FIFO Linux sont couverts par les tests de la CI Linux. Identité persistante de
reprise toujours Linux seulement. Audit assisté par agents, pas certification humaine.

La PR #11 reste en brouillon : état paginé et lifecycle SQLite v2, rotation et
reprise orchestrée/Run restent à relire. Prochain lot : lecteurs d'état SQLite
par identité et par chemin, pages bornées, snapshots de page et isolation de source.
