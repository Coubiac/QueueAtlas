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
Publié3ef8b1e dans #21 créée/attachée en brouillon,
CI37296483561 entière success vérifiée sur tête exacte. Données synthétiques,
aucune certification de rendu Web.

## Lot98 : fenêtres candidates connect/disconnect

Résultat attendu : citer les frontières et clients natifs concordants pour une
fenêtre fermée dans une origine, sans identité ou lien par PID seul. BuildPrequeueSessions
conserve toutes les projections97 et les références originales. Clients littéraux
connect/rapports/disconnect, dates utilisables dans leur intervalle et fermeture
strictement plus tardive, HasNonExplicitTime/CoverageUnproven. Un rapport douteux
rend toute la fenêtre non attribuée ; fenêtre ouverte, rapport orphelin, double
connect, client absent et date incompatible gardent des motifs fixes. La nouvelle
fenêtre démarre sur une nouvelle ancre connect.

Quatre tests PrequeueSessions et suite correlation/vet/diff Windows pass : deux
rapports rejet/warning, preuves/25permutations/copies, PID recyclé/origines distinctes,
file acceptée non rattachée, huit fenêtres douteuses refusées sans attribution
partielle, trois bornes absentes, start chevauchant et trois refus snapshot.
Un test échoué avant correction : absence de client natif connect donnait motif
client_mismatch, écrasant boundary_unproven déjà établi. Garde préserve le premier
motif, test passe ; aucune fausse attribution observée dans ce défaut diagnostic.
Revue code/tests indépendante sans blocage : quatre tests via overlay isolé Windows
pass, root inchangé. Relecture documentaire favorable avec précision appliquée :
priorité du premier motif seulement dans une fenêtre fermée ; frontière absente ou
interrompue impose boundary_unproven. Aucun test relancé pour cette précision.
Publication/CI98 en cours, aucun test des fondations relancé sans delta. Toujours
aucune identité globale, stockage ni Web.
