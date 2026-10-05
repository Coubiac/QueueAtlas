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
nouveau test. Publication/CI106 à vérifier.

Les valeurs de test sont synthétiques ; aucune couverture, continuité ou persistance
de projection annoncée. Une base dont la forme est valide n'est pas authentifiée.
