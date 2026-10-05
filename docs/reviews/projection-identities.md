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
Publication/CI103 à vérifier ; dernier état fusionné validé102.
