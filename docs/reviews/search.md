# Revue du chantier recherche

## Lot111 : recherche exacte et pagination

Résultat attendu : rechercher sender/recipient persistés dans une instance/période
explicite, parcours indexé, curseur lié aux critères, aucun résultat partiel sur erreur.
SearchEvents développé : enum fermé, paramètres littéraux, période UTC <=31 jours,
limite1..200, comparaison de tuple temps/ID après curseur, hits physiques avec réserves
de date/NOQUEUE. Aucun nouveau schéma, parser, verdict, DTO de projection ou Web.

Six tests SearchEvents Windows pass : corpus11/09/06, champ vide vs absent, casse,
instance étrangère/date inconnue/NOQUEUE ; timestamps égaux, From/Until, six hits
sans doublon, curseur incompatible/limit changé/location UTC équivalente ; SQL,
wildcards et octets invalides littéraux ; treize refus, annulation, page maximale200 ;
deux corruptions après un hit valide rendent page zéro et erreur sans donnée ;
EXPLAIN vérifie sender/recipient index avec curseur et sans tri temporaire.
Suite SQLite/vet/diff pass avant remplacement OR par tuple ; six tests/vet/diff
pass sur tuple final. Premier build corrigeait un nom de type model.EventKind
inexistant, remplacé par model.Kind avant tests ; aucun défaut runtime associé.

Revue indépendante code/docs favorable sans blocage ; six tests via overlay Windows
isolé pass, checkout propre/root inchangé. Deux mentions obsolètes de reprise
corrigées (CI108 et branche courante). Publié28eddb9 dans #25 créée/attachée.
CI37372754946 : Windows pass, deux jobs Linux annulés sans runner acquis,
annotations GitHub vérifiées ; aucune étape de test dans ces deux jobs.
Le commit112 déclenchera la validation entière111–112, sans rerun local des fondations.
Pas de mesure de charge ni couverture, pas de snapshot conservé entre pages.
Les adresses sont exactes et présentes, jamais normalisées depuis l'absence.

## Lot112 : identifiants exacts et index v5

Résultat attendu : Queue ID/Message-ID littéraux, résultats non fusionnés, index
temps et pagination, migration sans changement des faits existants. Enum fermée,
non vides32/1024, Queue ID <>'' pour index existant ; nouvel index Message-ID/temps
non unique uniquement, historique/user_version dans la même transaction.

Cinq nouveaux tests et suite SQLite/vet/diff Windows pass : corpus13 huit faits
de deux cycles conservés/instance étrangère exclue ; corpus11 deux files avec même
Message-ID paginées distinctes ; SQL/wildcards/octet invalide littéraux, bornes et
curseur incompatible ; EXPLAIN avec/sans curseur pour deux index sans tri ; v4→v5
préserve faits/CP/projection et reopen ; échec provoqué rollback index/historique/version.
Premiers tests corrigés : nom du corpus13 et attente Message-ID avec angles alors
que le parser existant les retire. Aucun parser changé. Assertions courantes à5,
version6 refusée. Revue code indépendante favorable, cinq tests via overlay isolé
Windows pass, root/Git inchangés, fondations non relancées. Revue documentaire finale
favorable après précision du rollback vers la version initiale (cas v4 testé).
Aucun test relancé ; implémentation112 publiéec44746d dans #25,
CI initiale37415832690 en cours à publication ; bilan final13bce0d publié,
CI37415877709 entière success vérifiée REST sur cette tête. Lot112 validé.

## Lot113 : domaines et migration dérivée v6

Résultat attendu : critères domaine exacts normalisés ASCII, pas de suffixe/LIKE,
colonnes dédiées indexées sans modification de facts référencés. Table dérivée,
indexes instance/domaine/date/ID ; même helper extraction migration/ingestion,
backfill par256 dansTX, writes/CP atomiques. NULL pour formes non extractibles,
adresse native conservée ; aucune validation complète RFC/IDNA ou couverture.

Cinq nouveaux tests Windows et suite SQLite/vet/diff pass : casse/curseur/ties,
suffixes/malformés exclus, NOQUEUE/instances/dates, adresse native inchangée ;
subset253/labels63/valeurs hostiles/UTF8/cancel ; EXPLAIN deux index avec/sanscurseur
sans tri ; migration301 faits/manifest/CP/immutabilité/reopen ; rollback migration
et ingestion/retry/cascade. Assertions version courante6/newer7, fixture v1 crée
temporairement puis retire table dérivée pour laisser un vrai ancien schéma avantOpen.
Revue indépendante code favorable, cinq tests Windows via overlay isolé pass,
root/Git inchangés, aucune fondation relancée. Pas de charge ni Linux local mesuré.
Revue documentaire finale favorable après précision des bornes d'adresse111 ;
aucun test relancé. Publiéeca2e7cb627879b40b442daa80e414e940cf22bb dans #25,
CI37418164349 entière success vérifiée REST sur tête exacte. Lot113 validé.

## Lot114 : intégration recherche → scope complet → reconstruction

Résultat attendu : usage explicite des API existantes, sans confondre page/critère
de recherche et snapshot complet du scope. Runtime111–113 inchangé ; trois tests
nouveaux Windows pass, vet/diff pass. Pas de relance locale des fondations scellées.

- Six critères limit1/périodeOct3 : scope QueueKey choisi, 16 faits toutes origines,
  8 sans date/4Oct4 conservés, 2 cycles séparés+1stream non résolu ; counts SMTPsent
  et localdelivered distincts, revision/coverage reserve et ALL16 memberships.
- Hit NOQUEUEdomain : unqueuedinstance choisi explicitement, 6faits/2rejets, session
  candidate1/rapportundatednonassigné1, pasqueuedforeign ni lienfile, Current identique.
- Message-ID répété/page1 : une file choisie4facts sans suivre l'autre file ; import
  tardif stale/readlimitnil/installation refusée/ancien manifest intact ; refresh8facts
  sépare2origins et Current identique à la nouvelle révision.

Revue indépendante favorable, trois tests via overlay Windows isolé pass, root/Git
inchangés et aucune fondation relancée. Revue documentaire finale favorable,
aucun test relancé. Publié7d029b896c039da3c4af427e74e5a01149195ff0 dans #25,
CI37420603702 entière success vérifiée REST sur tête exacte. Lot114 validé.
Pas de mesure de performance ou exécution Linux locale, aucun pilote/API/Web ajouté.

## Lot115 : mesures ciblées, correction seek et bilan

Résultat attendu : protocole reproductible et limites honnêtes, pas seuil CI de
performance. Deux benchmarks1k/10k : six critères/page100/première et seek,50%foreign
avanttrusted, cache chaud/préparationhorsmesure ; migration réelleDDL/backfill/commit
avec reset/vérificationhorsmesure. Smoke1x pass, mesures3x100 pages avant/après et
3x1 migrations. Rapport et sorties brutes liés depuis search-measurements.md.

Coût du seek initial croissant confirmé ; args[2]=cursor.TimeNS après validation
resserre plage scalaire, tuple/dateégale/hashoriginal conservés. Suite SQLite/vet/
diff Windows après correctif pass. Douze tests recherche via overlay isolé pass,
revue code/méthode favorable, mesures lues sans remesure indépendante, root/Git
inchangés. Seek10k~0,214–0,220ms après vs0,470–1,118ms avant ; premières pages
adresse/MessageID~2,1–2,2ms filtrentforeignsansindexinstance. Migration~8,484/87,684ms,
allocations cumulées paspicRSS, aucun seuil/SLA/extrapolation.
Revue documentaire finale indépendante favorable, médianes/méthode/estimations
vérifiées sans rerun. Publication115/CI finale/fusion/main à vérifier au moment du
commit ; #25 passe ready puis fusion seulement après CI entière de tête finale verte.
Publié694fb6de0836b5e7773e081f2082be8a39830589, CI finale37423566878 entière success
vérifiée REST exacte. Commentaire assisté5424580355/ready, #25 fusionnée sur
f68ae85970e7e5878774ed948330eb7059def21a ; CI pushmain37423754491 entière success
vérifiée REST exacte. Chantier111–115 terminé, checkout main actualisé/propre avant116.
M3 encore incomplet.
