# Revue des diagnostics de source

## Lot86 : rapport de vocabulaire fixe

Deux fichiers file/diagnostic.go/tests. Fonction pure Diagnose, aucune mutation de
Run/Sink/stockage. Path enum whitelist, Failure vocabulaire fixe, bool rapports
missing/gap/degraded, pas de contenu/identifiants/strings erreur. LastPathStatus
stale, rapport non atomique, tags de dépendances non preuve de perte/recovery.
Context pur neutre, joins avec réel échec préservés, scan insuffisant/ambigu/limité
jamais gap, fsNotExist/EOF génériques pas diagnostic implicite current/complete.

Trois tests ciblés initialement réussis. Auditeur relève repli error de priorité
inférieure à tag gap/missing déjà vu : budget dépassé ou Path invalide pouvait
laisser Failure confiant. Régression mixte échoue avant correction. Flag unverified
force Failure error/Degraded après traversal ; Gap/Missing déjà vus restent rapports.
Quatre tests ciblés/suite/vet/diff Windows réussis après fix, quatre cas régression
mixtes. Delta correctif relu, régression quatre cas exécutée par auditeur sans
blocage ; publication/CI à vérifier.
Cycle/branches nil bornés64, aucune lecture Error/Is/As ; Unwrap doit terminer.
Pas de metrics exporter/compteur/CLI. Données synthétiques, audit assisté.
