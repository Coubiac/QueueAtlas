# Mesures locales de recherche — lot115

## Environnement et protocole

Mesuré le 6 octobre 2026 : Windows/amd64, Go1.26.2, Intel Core i7-13650HX,
modernc.org/sqlite v1.60.1. Base fichier temporaire, WAL/synchronous FULL,
une connexion ; données exclusivement synthétiques. Versions initiales sur
7d029b896c039da3c4af427e74e5a01149195ff0, puis borne scalaire du curseur corrigée
dans ce lot. [Sortie brute avant/après](benchmarks/search-windows-2026-10-06.txt).

BenchmarkSearchEventsIndexed : 1 000 et10 000 événements, une seconde entre dates,
deux instances de même taille avec mêmes critères. Toutes les lignes étrangères
précèdent chronologiquement celles de l'instance recherchée ; critères denses pour
les six champs. Page de100 hits, première page ou curseur au milieu de la moitié
recherchée (position75% de la base). Le benchmark vérifie100hits et une suite.
Préparation, warmup et construction du curseur hors mesure. Même page répétée en
série/cache chaud ; pas parcours intégral, test concurrent ou corpus Postfix parsé.

BenchmarkSearchDomainMigration : même jeu persisté, puis tablev6 retirée et version
ramenée à5 pour la fixture. Reset/vérification hors mesure ; véritable migrate
v5→v6 chronométré, table/index/backfill/history/version/commit inclus. Connexion et
pages natives déjà chaudes ; aucun manifest de projection dans ce jeu de mesure
(préservation vérifiée par les tests113). Temps d'Open/pragmas et pic mémoire exclus.

Commandes reproductibles depuis le dépôt :

```powershell
go test ./internal/storage/sqlite -run '^$' -bench '^BenchmarkSearchEventsIndexed$' -benchtime=100x -count=3 -benchmem
go test ./internal/storage/sqlite -run '^$' -bench '^BenchmarkSearchDomainMigration$' -benchtime=1x -count=3 -benchmem
```

## Résultats après correction

Médiane des trois valeurs ns/op, en millisecondes. Chaque valeur de recherche est
la moyenne de100 opérations, chaque valeur de migration une seule opération.
Ce ne sont pas des percentiles de latence par requête ni un intervalle de confiance.

| Critère | Première1k | Curseur1k | Première10k | Curseur10k |
| --- | ---: | ---: | ---: | ---: |
| sender | 0,284 | 0,221 | 2,193 | 0,218 |
| recipient | 0,283 | 0,216 | 2,191 | 0,214 |
| Queue ID | 0,205 | 0,216 | 0,209 | 0,215 |
| Message-ID | 0,286 | 0,217 | 2,079 | 0,219 |
| Domaine sender | 0,204 | 0,216 | 0,215 | 0,219 |
| Domaine recipient | 0,207 | 0,214 | 0,211 | 0,220 |

Avant correction, pages après curseur10k : 0,470–1,118ms selon champ ; après :
0,214–0,220ms. Le filtre scalaire conservait From, permettant de parcourir un
préfixe avant le test de tuple. Il utilise maintenant cursor.TimeNS après validation,
avec tuple(time,id)>cursor inchangé pour les égalités. Hash des critères conserve
le From original ; aucune nouvelle sémantique ou invalidation des anciens curseurs.
Suite SQLite/vet/diff après correctif réussis, incluant bornes/date égale/critères.

Premières pages sender/recipient/Message-ID10k restent plus coûteuses : leurs index
commencent par valeur/date, sans préfixe instance. La recherche filtre donc des
lignes étrangères partageant la valeur. Page et fenêtre ne constituent pas un
budget SQL indépendant ; ce résultat ne justifie aucune promesse de coût constant.

| Migration v5→v6 | Médiane ms | Allocations Go cumulées B/op, ordre de grandeur |
| --- | ---: | ---: |
| 1 000 événements | 8,484 | 0,71–0,72 Mo |
| 10 000 événements | 87,684 | 7,17–7,18 Mo |

Backfill parcourt toute la base sous une transaction, batches256 bornant les lignes
en mémoire de travail, pas la durée globale ni les allocations cumulées. B/op de
benchmem est une quantité allouée par opération, pas mémoire retenue, heap/RSS de
pointe ou taille SQLite/disque. Pages de recherche après curseur : environ82ko
alloués/1892–1893 allocations dans ce jeu, sans mesure de pic.

## Portée du bilan

Deux benchmarks, smoke1x réussi, mesures avant/après3x100 pages et3x1 migrations,
sorties conservées. Revue code/méthode indépendante favorable ; douze tests ciblés
de recherche via overlay Windows isolé réussis, résultats lus sans remesure
indépendante. Ce bilan ne mesure pas débit d'ingestion/parser, reconstruction,
recherche sans hit, dates identiques en masse, stockage froid, Linux, concurrence,
gros volumes ou disponibilité en production. Charge de l'ordinateur non contrôlée.

Les essais couvrent un profil dense précis, pas le corpus représentatif du critère
MVP. Les mesures/pilote Linux et plusieurs distributions restent àM5, les limites
API et délais applicatifs àM4. Aucun seuil CI fragile ajouté aux mesures ; tests
fonctionnels et plan EXPLAIN restent les contrôles reproductibles du chantier.
