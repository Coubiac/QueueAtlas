# Revue du chantier expiration et réserves de synthèse

## Lot94 : faits explicites d'expiration

Résultat attendu : conserver les rapports qmgr status=expired avec leur preuve,
sans inférence depuis removed, bounce, reply distante ou journal incomplet.
BuildRecipients ajoute les preuves à la génération candidate existante ; aucun
changement de résultat des tentatives ou de DB/source/Web. Premier statut natif
borné exigé, chevrons/valeurs anciennes et suffixes trompeurs refusés. Toutes les
références et dates copiées, multiplicité/origines et unresolved conservés.

Quatre tests Expirations et suite correlation, vet/diff Windows réussis : corpus10,
25 permutations/copie de date, trois corpus sans preuve, quatorze refus natifs ou
de classification, rapports répétés de même date, sources indépendantes et
undated/limite sans résultat partiel. Revue indépendante sans blocage, quatre
TestExpirations exécutés par auditeur sur Windows : pass. Documentation relue,
publié0762759 dans #20, CI37294712566 success vérifiée sur tête exacte.
Pas de test des fondations relancé sans delta. Données synthétiques ; revue assistée.

## Lot95 : comptes et réserves de synthèse

Résultat attendu : compter les résultats des adresses observées, sans confondre
retries/rapports d'expiration avec destinataires, et rendre les réserves visibles.
BuildSummaries reprend bornes/refus/projections. CoverageUnproven toujours présent
(aucun contrat de couverture en entrée), réserves de réception/removal/date/origines/
absence de destinataires/empty/tie/unknown/unprojectable fixes et ordonnées. Aucun
statut global, nrcpt inféré, certificat de complétude ou DB/source/API/Web.

Quatre tests Summaries et suite correlation/vet/diff Windows réussis : mixed04,
trois retries08, expiry10 séparé, permutations, réception+removal+dates explicites
ne certifient pas la couverture, nrcpt999 ne crée pas d'adresses, partial17,
aucune tentative, tie07/empty/unprojectable, origines distinctes/undated/NOQUEUE et
trois limites refusées sans résultat partiel. Une attente initiale incorrecte pour
fixture05 corrigée après lecture : remise local/maildir = delivered, pas sent.
Revue indépendante en cours ; lot95 non publié/non validé par CI.
