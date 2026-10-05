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
TestExpirations exécutés par auditeur sur Windows : pass. Publication/CI en cours.
Pas de test des fondations relancé sans delta. Données synthétiques ; revue assistée.
