# Revue du chantier NOQUEUE et sessions candidates

## Lot97 : rapports smtpd NOQUEUE distincts

Résultat attendu : projeter reject et reject_warning séparément avec leurs faits,
sans session/file inventée depuis PID, adresse ou proximité de date. BuildPrequeue
réutilise bornes/refus de PartitionFacts, conserve Queued/Other ; filtre natif et
service, métadonnées présentes/vide, dates copiées, Message/PID simples données.
Warning ne prouve ni rejet ni acceptation ultérieure. Pas de DB/source/API/Web.

Quatre tests Prequeue et suite correlation/vet/diff Windows réussis : RCPT05 avec
file distincte, 25permutations/copiedate, warning/nullsender, absence et ambiguïté
de métadonnées, champs hostiles, date inconnue, rapports répétés mêmePID/origines,
hôte déclaré sans effet, neuf faits non projectables conservés et quatre refus
de snapshot sans résultat partiel. Revue code/docs indépendante sans blocage,
quatre tests Prequeue exécutés par auditeur Windows via overlay checkout isolé
propre : pass. Aucun fichier root modifié, aucun rerun à la relecture documentaire.
Publication/CI97 à vérifier ; données synthétiques, aucune certification de rendu Web.
