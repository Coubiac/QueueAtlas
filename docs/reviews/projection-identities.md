# Revue du chantier identités de projection

## Lot103 : clés candidates liées à une révision

Résultat attendu : numéroter les générations candidates sans alias après import
tardif, sous une révision d'entrée déterministe indépendante de l'ordre d'arrivée.
BuildQueueInstances réutilise les générations/bornes/refus ; origines séparées,
réserves inchangées et aucune identité pour Unresolved/Other. Clé complète
révision/instance/queue_id/ordinal ; le numéro seul n'est pas une identité durable.

Révision SHA256 à domaine versionné, toutes observations et provenances incluses,
chaînes cadrées en octets/maps triées/instants UTC. Les octets invalides restent
distincts ; l'empreinte ne devient pas une preuve de continuité ou de couverture.
Le domaine doit évoluer si le framing ou les règles d'identité/génération changent.
Options SMTP et projection complète ne sont pas versionnées par cette primitive.

Cinq tests QueueInstances, suite correlation, vet et diff Windows pass : deux
cycles recyclés,25permutations/copies ; trois sources/deux instances et réserves,
NOQUEUE/non datés gardés sans clé ; import ancien changeant ordinal/révision sans
alias ; changement de preuves/maps/date/hôte, instants équivalents, octets invalides,
frontières de champs et insertion des maps ; cinq refus sans sortie partielle et
snapshot vide versionné. Données synthétiques. Revue code indépendante sans blocage ;
cinq tests via overlay isolé Windows pass, root inchangé. Documentation alignée,
avis final favorable sans nouveau test.
Publié76055c0 dans [PR #23](https://github.com/Coubiac/mailtrace/pull/23) créée/attachée,
CI37329247549 entière success vérifiée sur la tête exacte ; fusionné validé102.

## Lot104 : composition cohérente et options versionnées

Résultat attendu : résumés, liens et NOQUEUE liés au même snapshot et à des clés
qui versionnent aussi la configuration explicite des liens. BuildProjection
compose les primitives validées, avec zéro output sur refus/invariant d'ancre.
InputRevision103 distincte de Revision complète à domaine versionné ; durée et
tous mappings inclus, copiés/triés, ordre de configuration sans effet.
Clés queues/endpoints dans la même révision ; preuves et réserves conservées.

Quatre tests Projection, suite correlation/vet/diff Windows pass : corpus mixte
8queues/3links/1session et25permutations, preuves/faits conservés et résultats des
destinataires identiques à BuildSummaries ; modifications de durée/mappings utilisés
ou non et permutation de configuration ; archive non datée tardive préservée et
relations devenant candidates ; quatre refus et vide sans résultat partiel.
Revue code indépendante favorable, quatre tests via overlay isolé Windows pass,
root inchangé. Documentation alignée, avis favorable sans nouveau test ;
publication/CI104 attendues.
Aucun stockage/source/API/Web ni parcours global ajouté.
