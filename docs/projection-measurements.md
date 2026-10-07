# Protocole de mesure des projections — lot125

État au 7 octobre 2026 : benchmarks préparés et exécution courte locale réussie.
Les mesures répétées et leur analyse restent au lot126. Aucune conclusion de
capacité, comparaison avant/après ou optimisation n'est livrée par125.

## Entrées et bornes

Les trois benchmarks de [projection_benchmark_test.go](../internal/storage/sqlite/projection_benchmark_test.go)
utilisent exactement le même générateur de logs Postfix synthétiques natifs.
Une source, une origine physique, une instance configurée ; dates RFC3164 avec
année2026/UTC explicites dans la configuration du parseur. Chaque génération
comprend quatre faits : réception qmgr, deferred SMTP, sent SMTP une seconde
plus tard, puis retrait qmgr. Les cycles réutilisent le Queue ID après un retrait
strictement antérieur à la réception suivante. Un destinataire, deux tentatives.

| Sous-cas | Faits | Parts du scope (Queue IDs) | Cycles par ID | Générations candidates |
| --- | ---: | ---: | ---: | ---: |
| facts16_scope4 | 16 | 4 | 1 | 4 |
| facts1024_scope16 | 1 024 | 16 | 16 | 256 |
| facts4096_scope1 | 4 096 | 1 | 1 024 | 1 024 |
| facts4096_scope64 | 4 096 | 64 | 16 | 1 024 |

Les deux derniers cas isolent la répartition des mêmes quantités entre un ID
dense et64IDs ; le scope ne compte pas les générations. Aucun appel ne dépasse
MaxPartitionFacts=4096 ni MaxCorrelationScopeParts=64. Le limit passé est4096
dans les quatre cas. Le profil de retry est régulier, sans Message-ID, NOQUEUE,
réinjection, origines multiples, faits non datés ou conflit de frontière.
Les tests fonctionnels couvrent ces comportements ; leurs coûts ne sont pas
mesurés par ce protocole. Le pilote représentatif et la concurrence restent M5.

## Ce qui est chronométré

| Benchmark | Opération mesurée | Préparation exclue |
| --- | --- | --- |
| BenchmarkProjectionBuild | BuildProjection, composition complète depuis les faits en mémoire, hashes/clés/réserves inclus | Génération des raw, parsing, construction du batch et des faits ; aucune base ouverte |
| BenchmarkProjectionInstall | InstallProjection : reconstruction initiale, transaction/réservation, relecture et seconde reconstruction de revalidation, remplacement du manifest/FK/scope, commit | Open/migrations, Commit des raw, CorrelationFacts initial, première installation et warmup |
| BenchmarkProjectionCurrent | CurrentProjection : transaction readonly, scope/options/membership/faits, reconstruction, fraîcheur et commit | Open/migrations, Commit, lecture initiale des faits, installation et warmup |

Install mesure le remplacement répété d'un manifest existant pour le même scope,
sans ingestion concurrente ni changement des faits. Current reconstruit réellement,
il ne lit pas un cache de résultats sérialisés. Base fichier temporaire par
sous-cas/exécution, WAL/synchronous FULL selon Open, une connexion. Parsing,
préparation et fermeture/nettoyage hors chronométrage. Série, connexion et pages
chaudes, aucune purge de cache OS ni réouverture par opération ; premier Open,
première installation et démarrage à froid ne sont pas mesurés.

Avant chronométrage : contrôle des dates/parsing, tailles/bornes et génération de
la projection pure ; vérification de toutes les générations, quatre références,
réception/retrait, deux tentatives, résultat sent et réserves coverage_unproven /
non_explicit_time. Pour SQL : faits persistés identiques aux faits parsés,
installation/warmup identiques à la projection pure (comparaison complète).
Pendant chaque opération : erreur/found, révision et nombre de générations
contrôlés ; ces contrôles légers et l'appel de fonction sont inclus dans ns/op.
Aucun verdict de couverture complète ou livraison finale n'est supposé.

## Commandes reproductibles

Depuis le dépôt, smoke fonctionnel125 (une opération mesurée par cas) :

```powershell
go test ./internal/storage/sqlite -run '^$' -bench '^BenchmarkProjection(Build|Install|Current)$' -benchtime=1x -count=1 -benchmem
```

Cette commande est aussi un gate Linux Go1.26 dans la CI, sans seuil de temps.
Les tests habituels compilent les benchmarks ; le smoke les exécute réellement.
Un succès établit la validité du jeu et du parcours, pas un budget de latence.
[Sortie brute locale125](benchmarks/projection-smoke-windows-2026-10-07.txt).

Lot126 prévu : enregistrer SHA des fichiers mesurés, date, Go/GOOS/GOARCH/CPU,
driver et répertoire temporaire ; conserver toutes les sorties, y compris celles
d'un essai qui échoue. Première campagne proposée :

```powershell
go version
go env GOOS GOARCH
go list -m modernc.org/sqlite
go test ./internal/storage/sqlite -run '^$' -bench '^BenchmarkProjection(Build|Install|Current)$' -benchtime=5x -count=3 -benchmem
```

Ne pas exécuter cette campagne pendant une autre tâche de mesure. Conserver les
trois valeurs par cas ; médiane des moyennes ns/op, sans inventer des percentiles
par requête ni intervalle de confiance. Ajouter des répétitions seulement pour une
variabilité précise à résoudre. Le protocole peut évoluer avant126 si un défaut
de méthode est identifié, avec changements et nouveau SHA consignés.
Consigner aussi une éventuelle variable GOMAXPROCS ; son effectif est visible
dans le suffixe du nom de benchmark (20 pour le smoke125). Les boucles restent
sérielles : ce suffixe ne signifie pas20requêtes concurrentes.

## Limites d'interprétation

Build donne un coût en mémoire/CPU avec GC ; Install et Current incluent CPU,
allocations Go, driver et IO SQLite/OS. La différence entre deux chiffres ne mesure
pas isolément le disque, le fsync ou le parseur. B/op et allocs/op sont cumulés
par opération, pas heap retenu, pic RSS, taille DB/WAL ou pic mémoire natif du
driver. TempDir suit la configuration du système ; stockage et charge du poste
doivent être consignés, sans leur supposer une performance constante.

Les sorties smoke125 sont une seule observation par cas. Elles ne sont pas une
mesure répétée ni un résultat pilote. Les profils bornés ne donnent pas de coût
constant, débit d'ingestion, capacité maximale, SLA ou comportement au-delà4096.
La transposition Windows→Linux et la charge représentative restent à vérifier M5.
M3 demeure ouvert pour les mesures répétées, leur analyse et la revue finale.
