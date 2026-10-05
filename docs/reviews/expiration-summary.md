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
Revue indépendante code/docs sans blocage : quatre tests Summaries exécutés par
auditeur via overlay dans le checkout isolé propre, pass. Aucun fichier root
modifié, aucune suite fondatrice relancée. Publiéff8e2a1 dans #20,
CI37295460030 entière success vérifiée sur la tête exacte.

## Lot96 : clôture expiration et réserves de synthèse

Résultat attendu : clôturer le périmètre #20 après revue/CI exacte, fusionner sa
tête attendue et vérifier la CI push main. Runtime inchangé depuis94–95 relus ;
aucun contrôle des fondations relancé sans delta. Bilan/documentation actualisés,
M3 toujours incomplet : synthèse observée, pas couverture certifiée ni état final.
Avis documentaire final favorable : HEADff8e2a1, aucun delta runtime/tests depuis
revue95, expiration94 inchangée ; aucun test relancé. Résultats94–95 réutilisés,
CI95 entière success désormais vérifiée. Publication/CI finale96, fusion et CI
push main restent à vérifier. Revue assistée.
