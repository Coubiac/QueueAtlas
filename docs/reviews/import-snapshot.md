# Revue du chantier de copie privée du contenu d'import

## Lot 72 : copie pendant inspection

5 octobre 2026, base main `85effe5b62e0786f32d8cd9bcbf8a0fe7c1c26fb`.
Coordinateur et auditeur indépendant : extraction inspectContent/inspectGzip,
CopyPlain/CopyGzip/tests et ADR-011 relus. Aucun blocage de sécurité ; remarque
diagnostics corrigée/revue avant publication. Pas de filesystem/manifest/ingestion.

Sans output, inspection inchangée ; avec output, mêmes bytes des buffers copiés
pendant SHA, budget contrôlé avant Write, gzip valide tous les membres/CRC.
Output nil refuse avant Read, n invalide/short/error/cancel échouent metadatazero,
sans retry. Le caller doit jeter tout output en erreur (même payload complet mais
CRC invalide) ; input/output pas fermés. Contrats io et consommation exclusive.

Coordinateur : quatre TestCopy* -count=1 Windows, suite/vet/diff réussis. Auditeur :
dix tests Inspect/Copy initiaux exécutés Windows, puis régression ciblée douze
sous-cas après correctif : n+readErr et writer error/short/invalid/cancel, plain/gzipCRC,
EOF exact jamais joint comme échec. Les causes simultanées sont désormais jointes.
Copie normal/gzip/vide/partial/large, bytes et hash concordants, limites et output
tentatif corrompu couverts. Pas de risque restant identifié.
[CI du lot 72 réussie](https://github.com/Coubiac/mailtrace/actions/runs/37245062246)
sur `8aa3fc8b57512eedadc66fe4e5f25df208b84da6`, PR #15 : tests Copy Linux exécutés.

Limites : fermeture/cleanup/output partiel restent au caller jusqu'au lot 73,
IO bloquante non interruptible, pas de snapshot atomic de l'entrée, aucun importeur.
La copie privée détenue viendra dans un lot distinct. Audit assisté par agents.

## Lot 73 : préparation régulière et copie détenue

5 octobre 2026, base `8aa3fc8b57512eedadc66fe4e5f25df208b84da6`.
Coordinateur et auditeur indépendant : prepared.go/tests et contrat relus, aucun
blocage concret ou couverture supplémentaire nécessaire identifié. OpenLog réutilisé,
copie/hash/CRC/budgets du lot 72 conservés, pas d'ingestion/manifest.

Ownership dès MkdirTemp/CreateTemp/réouverture read-only ; refus ferme/nettoie avant
close input. Si input.Close ou contexte échoue après keep, propriétaire détruit et
résultat nil. Writer fermé une fois avant réouverture, taille = Info.Bytes, offset0.
Disque au plus ContentBytes, dir/fichier Unix0700/0600, parent trusted/protected.
Close vide les fields puis ferme et Remove fichier/dir vide, sans récursion ni parent ;
causes jointes/idempotence même sur échec, failed removal signalé sans retry silencieux.

Coordinateur/auditeur : trois tests Test(PrepareRegular|PreparedContent)* -count=1
Windows réussis ; coordinateur suite/vet/diff réussis. Nominal normal/gzip, vide/partial/
large, entrée changée après préparation, read-only/ReadAt/Seek/getters/Close/cleanup,
douze refus options/ctx/input/temp/budgets/CRC/header, close déjà fermé et foreign file
préservé. Première attente de test header corrigée : six octets donnaient UnexpectedEOF,
header invalide complet choisi pour ErrHeader ; pas de changement runtime requis.
[CI du lot 73 réussie](https://github.com/Coubiac/mailtrace/actions/runs/37245585832)
sur `93cf83cec9a1a39dff5600f93ddd5a0bb93fc745` : suite Linux et vérification des
bits privés Unix exécutées, ainsi que les autres contrôles du workflow Go.

Limites : parent temporaire protégé, ACL Windows non vérifiées, propriété exclusive,
IO bloquante non interruptible, copie éphémère/no snapshot atomique original/manifest.
Un failed removal peut laisser un résidu signalé ; pas d'import complet implicite.

## Lot 74 : clôture du chantier

Référence isolée `93cf83cec9a1a39dff5600f93ddd5a0bb93fc745`, checkout et diff propres.
Auditeur indépendant : fichiers runtime/tests identiques aux revues 72–73, aucun
blocage ni risque précis nécessitant de répéter les tests. Cohérence globale
copie/hash, budgets, CRC, propriété/read-only et cleanup conservée. Documentation
actualisée : préparation réalisée, permissions Unix exécutées en CI ; budgets
globaux, manifest et ingestion restent futurs. Audit assisté, sans certification externe.

Avis favorable à la fusion après CI exacte du commit documentaire final. Ce lot
ne change aucun comportement ; il clôture les deux lots développés et relus.
