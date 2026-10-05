# Revue du chantier indices et liens de file

## Lot100 : indices natifs, sans arc

Résultat attendu : conserver des indices typés SMTP/local/bounce avec preuve native,
jamais créer une file depuis un ID distant ni fusionner sur Message-ID ou relay.
BuildQueueHints réutilise bornes/refus/index de PartitionFacts. IsQueueID expose
uniquement la grammaire existante du parser. Premier statut/réponse exacts, formats
limités et identifiant borné exigés ; champs historiques/HTML/chevrons/suffixes
trompeurs ne deviennent pas preuves. SMTP n'a aucune instance cible de confiance,
même loopback ou file du même nom présente. Local/bounce restent des indices.

Quatre tests QueueHints et suites correlation/parser Postfix, vet/diff Windows
réussis : corpus11/12/21/09, preuves/permutations/copies, quinze formats non admis
et quatre valeurs historiques refusés, deux origines, dates inconnues/hôte déclaré,
REMOTE02 non projeté et quatre refus de snapshot sans résultat partiel. Toutes
les observations restent indexées, aucun résultat de remise ou arc changé.
Revue code/docs indépendante sans blocage : quatre tests QueueHints via overlay
isolé Windows pass, root inchangé ; documentation alignée, aucun rerun.
Publiébfdb9e0 dans #22 créée/attachée en brouillon, CI37324927168 entière success
vérifiée. Données synthétiques ; aucune configuration du serveur déduite
des manuels ou du relay.

## Lot101 : corroboration des deux côtés

Résultat attendu : indice conservé, relation corroborée seulement avec génération
cible unique et observations concordantes sous dates/mapping explicites. BuildQueueLinks
garde tous indices/générations, bornes/options et refus sans output partiel. Mapping
SMTP copié et littéral, source/target reçues strictement ordonnées et cible dans
fenêtre <=24h. Métadonnées qmgr/cleanup/delivery des deux côtés référencées ; aucun
choix parmi cibles ambiguës par leurs attributs faibles. Bounce distinct, sources
multi-origines et faits non datés restent candidats ; aucun résultat recipient changé.

Six tests QueueLinks et suite correlation/vet/diff Windows réussis : corpus11/21/09
corroborés et25permutations/copies, états des destinataires inchangés ; douze cas
sans mapping/cible/preuve/date ou avec contradiction conservés candidats ; origines
sources/cibles séparées, deux générations recyclées plausibles restent ambiguës ;
date égale/référence à soi refusées et qualité explicite conservée ; huit options
invalides et trois snapshots refusés sans sortie partielle. Revue initiale code
sans blocage, cinq tests via overlay isolé pass. Précision intégrée : unicité parmi
les générations temporellement admissibles, ID recyclé possible si une seule.
Contrôle supplémentaire de taille : 700 répétitions qmgr produisaient 708 preuves
positives pour un lien ; test échoué avant correctif. Garder seulement la première
référence par champ, en contrôlant toutes les contradictions et conservant les704
faits source, réduit les preuves à <=8. Test et suite passent après correction.
Delta final et docs validés sans blocage par auditeur ; régression700 répétitions
exécutée via overlay isolé Windows pass. La dernière répétition contradictoire est
également testée localement : reste candidate/evidence_insufficient. Aucun test
des fondations relancé, root inchangé par auditeur. Publication/CI101 à vérifier.
Pas de DB/source/API/Web.
