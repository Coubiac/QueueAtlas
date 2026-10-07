# Mesures locales des projections — lots125–126

État au 7 octobre 2026 : benchmarks125 préparés, smoke Windows et CI Linux réussis ;
campagne répétée126 exécutée et analysée. Les résultats restent locaux et
synthétiques, sans conclusion de capacité, comparaison avant/après ou optimisation.

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

Campagne126 réalisée : SHA, date, Go/GOOS/GOARCH/CPU, driver et répertoire temporaire
consignés dans la sortie brute. Commande de la campagne répétée :

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
M3 demeure ouvert pour la revue finale et la décision de clôture.

## Campagne126 et résultats

Exécutée le7octobre2026 de00:55:44 à00:56:11UTC sur
`aa5f3c9f727253ad3e2811050f6207e58adda2fe`, Git initial propre. Blob du benchmark
`90385a32e324e650b262c556d0a95c2bdf630b91`, inchangé pendant la campagne et dans126.
Windows/amd64, Go1.26.2, Intel Core i7-13650HX, modernc.org/sqlite v1.60.1 ;
GOMAXPROCS non défini dans l'environnement, effectif20 indiqué par Go. TempDir sur
`C:\Users\benoi\AppData\Local\Temp`, volumeC:NTFS. Matériel disque et charge du
poste non qualifiés ; aucune autre campagne de mesure lancée en parallèle par
l'agent. Boucles sérielles, conditions chaudes décrites ci-dessus.

[Sortie brute complète126](benchmarks/projection-windows-2026-10-07.txt), stdout/stderr
capturés sans réécriture des lignes de benchmark ; heures de début/fin et code de
sortie0 conservés. 36échantillons parsés : 12cas × 3répétitions, tous avec5opérations
mesurées ; contrôles de formes/révisions passés. Les warmups et calibrations Go
ne sont pas comptés dans ces5opérations. Aucune tentative échouée ou valeur retirée.

### Temps observés

Médiane des trois moyennes ns/op, convertie en ms. Entre parenthèses : minimum–maximum
des trois moyennes. Ce sont des moyennes de5opérations, pas des latences individuelles,
percentiles, intervalle de confiance ou délai maximal garanti. Arrondi à0,001ms.

| Faits / scope | Build ms | Install ms | Current ms |
| --- | ---: | ---: | ---: |
| 16 / 4 | 0,120 (0,115–0,147) | 1,340 (1,334–1,362) | 0,448 (0,447–0,462) |
| 1 024 / 16 | 9,386 (7,268–13,597) | 34,245 (33,907–35,004) | 17,959 (17,885–18,085) |
| 4 096 / 1 | 26,266 (25,555–26,717) | 133,933 (132,496–134,257) | 74,452 (73,374–77,244) |
| 4 096 / 64 | 25,998 (25,660–28,270) | 136,706 (134,482–136,797) | 75,689 (74,946–75,798) |

Build1024 varie de7,268 à13,597ms, rapport max/min≈1,87 ; la charge du poste n'est
pas contrôlée et ces trois moyennes ne permettent pas d'en attribuer la cause.
Le bilan est descriptif : aucune décision d'optimisation ou de classement ne dépend
de cet écart, donc pas de répétition supplémentaire pour rechercher une valeur
préférée. Une future comparaison nécessitera un environnement et un protocole
adaptés à sa question précise.

À4096faits, les deux distributions ont des temps du même ordre de grandeur dans
cette campagne. On ne peut pas en déduire un coût indépendant du nombre de parts,
ni extrapoler au-delà des limites. Install inclut deux reconstructions et un
remplacement transactionnel ; Current relit les faits et reconstruit. Les écarts
entre opérations ne sont pas une mesure isolée de leur coût IO.

### Allocations Go observées

Chaque cellule : médiane B/op en Mo décimaux (1Mo=1 000 000octets), puis médiane
allocs/op. Les médianes sont calculées séparément par métrique ; ce ne sont pas
nécessairement les valeurs de la répétition dont le temps est médian.

| Faits / scope | Build Mo / allocs | Install Mo / allocs | Current Mo / allocs |
| --- | ---: | ---: | ---: |
| 16 / 4 | 0,160 / 2 115 | 0,430 / 7 149 | 0,265 / 4 962 |
| 1 024 / 16 | 9,623 / 107 444 | 25,871 / 380 026 | 15,978 / 267 414 |
| 4 096 / 1 | 42,132 / 420 455 | 111,929 / 1 500 372 | 68,712 / 1 058 983 |
| 4 096 / 64 | 40,675 / 429 101 | 108,732 / 1 520 456 | 66,954 / 1 070 275 |

La reconstruction alloue sensiblement à la borne4096, surtout lorsque Install
compose deux projections. Ces quantités sont cumulées, pas une mémoire résidente
ou un pic mémoire ; aucun profil d'allocation/pic/RSS natif n'a été réalisé.
Le coût des scénarios exclus (liens, origines multiples, NOQUEUE, ambiguïtés),
des pages froides, de la concurrence et du pilote Linux demeure non mesuré.

## Bilan de vérification126

Campagne5x/count3 entière réussie, cohérence des36échantillons et calcul des
12groupes/minimums/médianes/maximums contrôlés. Sortie brute/environnement/SHA
conservés ; aucun changement de code, benchmark ou configuration CI dans126.
Validations fonctionnelles scellées réutilisées, pas de rerun local des fondations.
Le lot125 est publié dans #29 suraa5f3c9 ;
[CI37552260931](https://github.com/Coubiac/QueueAtlas/actions/runs/37552260931)
entièrement réussie, étape smoke Linux Go1.26 passée. Publication/CI126 à vérifier
après commit. Prochaine action127 : relecture de sortie M3, limites/critères et
PR #29, puis décision de clôture après CI finale. Les mesures ne ferment pas M3
à elles seules ; le pilote Linux représentatif demeure M5.
