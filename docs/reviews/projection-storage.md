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
Windows isolé pass, fondations réutilisées sans rerun/root inchangé. Publication/CI108
non vérifiées. Estimation mise à jour séparément dans avancement : 10–20 M3 et35–63
MVP, incluant validation108 et sans réduire les critères.
Pas de cache dérivé, d'historique public, de lecture de révision ni de fraîcheur après
ingestion future annoncé : ces contrôles restent au lot suivant.
