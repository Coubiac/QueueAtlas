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
tentatif corrompu couverts. CI de publication à vérifier. Pas de risque restant identifié.

Limites : fermeture/cleanup/output partiel restent au caller jusqu'au lot 73,
IO bloquante non interruptible, pas de snapshot atomic de l'entrée, aucun importeur.
La copie privée détenue viendra dans un lot distinct. Audit assisté par agents.
