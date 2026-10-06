# Revue du chantier stockage des projections

## Lot106 : lecture bornée des faits SQLite

Résultat attendu : snapshot choisi explicitement, valeurs persistées sans reparse,
toutes origines concernées conservées, aucun résultat partiel si limite dépassée.
CorrelationFacts développé en une requête paramétrée, scope1..64/limit1..4096,
provenances validées, context et erreurs sans output partiel. Aucun write/migration.

Six tests CorrelationFacts et suite SQLite/vet/diff Windows pass : corpus11/09/06,
insertion inverse et instance étrangère, projection avec mêmes états/preuves et
session NOQUEUE ; archive non datée indépendante et portée SQL littérale ; limites
exacte/dépassée, sept scopes invalides et annulation ; année2500 stockée sans instant
et instance de repli ; onze JSON invalides et erreur de conversion sans données.
Premier essai du test de copie échouait sur la map nil d'un connect ; test ajusté
pour muter une map présente, aucun défaut du lecteur associé.

Revue indépendante : cinq tests initiaux pass via overlay Windows isolé, puis deux
défauts reproduits avant publication. JSON null d'entrée devenait chaîne vide/bool
false et pouvait fabriquer un expéditeur explicitement vide ; erreur Scan brute
citait une valeur stockée. Les régressions root ont échoué avant correctif, avec
cas liés de doublons/casse/membre manquant. Lecture JSON par tokens à types exacts
et erreurs fixes corrigent ces défauts ; six tests et suite passent après correction.
Delta final sans blocage, deux régressions ciblées via overlay isolé pass, résultats
initiaux réutilisés/root inchangé. Documentation alignée, avis final favorable sans
nouveau test. Publié e2426b5 dans #24 ; CI37333408517 entière success vérifiée.

Les valeurs de test sont synthétiques ; aucune couverture, continuité ou persistance
de projection annoncée. Une base dont la forme est valide n'est pas authentifiée.

## Lot107 : migration et contraintes des manifests

Résultat attendu : schéma v4 conservant les faits existants, ownership des révisions
et intégrité de leurs entrées ; aucune sérialisation des résultats dérivés.
Cinq tables, configuration BLOB, hashes/formats/bornes, FK composite du current,
double référence raw/event et triggers contre UPDATE des faits référencés.

Cinq tests ciblés et suite SQLite/vet/diff Windows pass. Migration v3/reopen conserve
faits/checkpoint, échec provoqué revient entièrement à v3 ; current étranger refusé ;
suppression/mutation des parents et suppression de la révision courante refusées ;
invalidation explicite SQL permet ensuite la suppression ; événement manquant,
doublon, mauvais types/bornes refusés ; octets invalides UTF8 BLOB conservés.
Compatibilité v1/v2/v3 et refus d'une base plus récente vérifiés par les tests existants.

Auditeur indépendant : cinq tests via overlay Windows isolé pass, aucun blocage
concret sur le schéma/migration, root inchangé. La concordance de fact_count et
la fraîcheur après ajout de faits restent à vérifier par les API futures. Aucun
stockage de projection dérivée ni API de rétention annoncé. Documentation relue
indépendamment sans blocage, aucun test relancé. Publié8d49039 dans #24,
CI37336085923 entière success vérifiée sur la tête exacte.

## Lot108 : installation atomique et refus d'un snapshot obsolète

Résultat attendu : révision/options/périmètre/tous faits validés dans une transaction
après réservation d'écriture et relecture ; aucun remplacement si entrée changée.
InstallProjection compare nombre/refs/révision recalculée et installe le manifest,
remplace seulement le scope concerné, rollback intégral sur refus/erreur SQL.
Lecteur106 partagé avec transaction ; IDs internes ajoutés pour les seules FK.

Six tests nouveaux, six lecteurs106 après refonte, suite SQLite/vet/diff Windows pass.
Manifest complet live+archive non datée+NOQUEUE, réserves conservées, réordonnancement
stable, changement d'options/version, autre scope préservé et reopen ; entrées
incomplètes/attribut changé/import tardif refusés sans remplacement ; erreur provoquée
après un membership revient à l'ancien état ou ne laisse aucun nouveau scope ;
scope vide déclaré et options BLOB UTF8 invalide, framing sans ambiguïté ; limites,
scope/options invalides/annulation ; deux connexions vérifient le verrou avant lecture.
Premier test rollback utilisait un compte global=1 et ne provoquait pas l'erreur pour
le nouveau scope ; fixture ajustée au compte par NEW.revision_id, aucun défaut runtime.

Revue indépendante code/docs favorable sans blocage, six nouveaux tests via overlay
Windows isolé pass, fondations réutilisées sans rerun/root inchangé. Publié
1edbed25b4bfbb914a7d5fa83f6d6fe58b1eba51 dans #24 ; CI37367582155 en file d'attente,
trois jobs queued vérifiés, aucun succès ni échec annoncé. Estimation mise à jour
séparément dans avancement : 10–20 M3 et35–63
MVP, incluant validation108 et sans réduire les critères.
Pas de cache dérivé, d'historique public, de lecture de révision ni de fraîcheur après
ingestion future annoncé : ces contrôles restent au lot suivant.

## Lot109 : lecture/reconstruction et contrôle de fraîcheur

Résultat attendu : manifest et faits dans le même snapshot, aucun résultat obsolète
ou partiel, aucun recalcul persistant implicite. CurrentProjection vérifie parties,
scope propriétaire/format, mappings/ordinals, compte et ensemble complet des entrées,
puis InputRevision et révision complète de BuildProjection ; absent/currentNULL
distincts d'un manifest valide vide. Lecture seule, réserves/provenances conservées.

Six tests nouveaux, suite SQLite/vet/diff Windows pass. Corpus11/09/06 égale
installation, instance étrangère exclue, sortie modifiée sans effet et reopen ;
import tardif refuse stale puis installation explicite conserve unresolved ; neuf
manifests malformés (compte, entrée supprimée, hashes, bytes/ordinals/mappings,
conversion du window avec contrainte volontairement désactivée) refusent sans PII
ni résultat partiel ; absent/NULL/vide distincts ; limites, annulation/options binaires ;
deux connexions WAL : ancien manifest et faits restent ensemble dans la transaction
pendant nouvelle ingestion/remplacement, lecture suivante voit la nouvelle révision.
Souscas de conversion ajouté après suite : test malformed ciblé repassé ; hash forcé
à zéro dans la fixture évite une dépendance à un caractère particulier du digest.

Revue indépendante code/docs sans blocage ; six nouveaux tests via overlay Windows
isolé pass, delta malformed neuf souscas également pass/root inchangé. Documentation
validée sans nouveau test. Publié8f1ed84518f15a0005dccbcda242cdcc0662affc dans #24,
CI37368041134 queued au dernier contrôle.
Pas de garantie de fraîcheur future ou couverture ; pas d'historique/cache dérivé,
de rétention applicative ou de parcours global. Aucun rerun des fondations demandé.

## Lot110 : bilan et clôture documentaire

Runtime109 relu inchangé ; résultats23 tests ciblés106–109 (6+5+6+6), suites/vet/diff
et avis indépendants code/docs réutilisés. README aligné sur chantier publié mais
non fusionné ; contrats conservés : entrées exactes/BLOB, transaction manifeste,
reconstruction au snapshot, aucun résultat partiel ou réparation implicite.
Finale11037368191438 entière success sur f813a7d : job stable initial annulé sans
runner acquis (annotation GitHub), relance ciblée demandée puis trois jobs success.
Commentaire assisté5420339349/ready, fusion #24 suraaa95f8 ; CI main37372128796
success vérifiée REST sur tête exacte le6octobre après relance du job Go1.26 annulé
sans runner acquis. Runtime/local/fondations non relancés.
Revue documentaire finale indépendante favorable après correction d'une phrase
obsolète d'avancement (lecteur109 publié, validation distante encore attendue).
Aucun test local relancé ; publication/CI finale/fusion110 et CI main validées.
M3 inachevé, aucun Web/service/paquet, rétention ou continuité prouvée annoncé.
