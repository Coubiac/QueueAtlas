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
blocage ; publié 541a24e, CI 37266530472 success, PR #18.
Cycle/branches nil bornés64, aucune lecture Error/Is/As ; Unwrap doit terminer.
Pas de metrics exporter/compteur/CLI. Données synthétiques, audit assisté.

## Lot87 : intégration parser/import/SQLite et contrat d'exploitation

Runtime inchangé. Test portable réel Parse Postfix et syslog TimeContext sous
Attempt/import/SQLite : plain/gzip, interruption/restart et contexte de date changé,
observations acquittées non remplacées, RFC5424 explicit/CRLF/host vsinstance,
deux lignes identiques distinctes et EOF partagé sur reimport sans normalization.
Test portable Windows exécuté par coordinateur et auditeur avant interruption quota.
Suite/vet/diff Windows et compilation GOOS=linux réussis à la reprise.

Test Linux FileSource réel sur même texte/offsets/TrustedHost : huit faits et
origines distinctes, pas de collapse inter-source. Revue statique favorable,
exécution Linux réussie en CI87. Windows Device/Inode indisponible refuse insufficient,
restriction de plateforme explicite. Assertion unlink->FDclosed retirée après
remarque de revue ; preuves ownership FileSource scellées antérieures réutilisées.
Doc ingestion-contract.md expose décisions, limites, reprises et chevauchement
incertain sans protocole prouvé entre sources. Revue documentaire terminée sans
blocage ; publié 0b3e362, CI37287896648 success. Pas de metrics exporter/CLI/corrélateur ajouté.

## Lot88 : clôture du chantier et bilan du socle M2

Checkout isolé propre 0b3e362 ; versions diagnostic/tests pipeline/contrat identiques
aux versions relues. Aucun delta runtime ni risque nouveau, aucune suite relancée.
CI87 entière success : intégration Linux exécutée, matrice Go, race FileSource,
builds statiques et chemins Windows réussis. Revue de clôture sans blocage.
Bilan du socle M2 conforme aux livrables de la roadmap : sources, checkpoints,
rotation/copytruncate diagnostiqué, reprise et import. Les issues #4/#5 conservent
leurs critères applicatifs futurs ; aucune fonctionnalité CLI/exporteur/projection
ou preuve de chevauchement ajoutée. Clôture b1544a7, CI37288318666 success ; #18
fusionnée sur 8886427, CI push main37288519438 success vérifiée REST.
