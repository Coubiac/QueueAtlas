# Sortie M3 — matrice de vérification

État au lot123, 7 octobre 2026. M3 reste ouvert. Cette matrice inventorie les
vérifications acquises et les travaux restants ; elle ne relance pas les tests
et ne livre pas une nouvelle fonction de reconstruction.

Référence : [roadmap](phase-0-proposal.md#11-roadmap-et-critères-mvp) et
[ADR-005](adr/ADR-005-postfix-correlation.md). La sortie M3 concerne les API de
bibliothèque. CLI, HTTP/Web et authentification sont M4 ; installation et pilote
Linux représentatif sont M5. Les réserves sur les logs incomplets restent requises.

## Critères et preuves acquises

« Couvert » signifie testé dans le périmètre indiqué, avec les validations locales
et CI consignées dans les revues. Cela ne certifie ni la collecte entière ni le
fonctionnement d'une application qui n'est pas encore livrée.

| Critère M3 | Vérifications identifiées | État et limites |
| --- | --- | --- |
| QueueInstances et IDs recyclés | [Générations](../internal/correlation/generation_test.go), [clés](../internal/correlation/identity_test.go) : cycles séparés, permutations, import tardif, frontière douteuse non résolue ; `TestQueueInstancesLateImportInvalidatesOldOrdinals` | Couvert en bibliothèque. Ordinal et ancre ne prouvent pas la chronologie entre origines. |
| Tentatives par destinataire | [Destinataires](../internal/correlation/recipient_test.go) : mixed/retries, toutes les preuves, DSN/réponse/orig_to, conflits à date égale ; `TestRecipientsEqualDateConflictDoesNotInventLastVerdict` | Couvert en projection pure. Vérification dédiée du conflit après persistance/reopen à ajouter124. |
| NOQUEUE | [Sessions](../internal/correlation/session_test.go), [parcours SQL](../internal/storage/sqlite/search_reconstruction_test.go) ; `TestSearchReconstructionNoQueueUsesExplicitUnqueuedScope` | Couvert : périmètre sans queue explicitement choisi, pas de rattachement par PID/adresse à une file acceptée, faits non datés conservés. |
| Arcs de réinjection | [Liens](../internal/correlation/link_test.go), [lecteur SQL](../internal/storage/sqlite/projection_read_test.go) ; `TestQueueLinksNeverChooseBetweenOriginsOrRecycledTargetIDs`, `TestCurrentProjectionReconstructsCompleteSnapshotAndCopiesOutput` | Couvert : cible admissible unique et preuves corroborées, mapping SMTP explicite, candidats sans endpoint confirmé en cas d'ambiguïté. Les liens n'améliorent pas le résultat d'un destinataire. |
| États prudents | [Transport](../internal/correlation/delivery_test.go), [synthèses](../internal/correlation/summary_test.go) ; `TestSummariesNeverCertifyCoverageFromReceiptRemovalOrNrcpt` | Couvert : sent SMTP distinct de delivered local, absence de verdict global, coverage_unproven conservé même avec réception/retrait/nrcpt cohérents. |
| Composition et persistance | [Composition](../internal/correlation/projection_test.go), [installation](../internal/storage/sqlite/projection_install_test.go), [lecture](../internal/storage/sqlite/projection_read_test.go) ; `TestProjectionComposesEvidenceWithoutImprovingRecipientResults`, `TestCurrentProjectionRejectsLateFactsAndRequiresExplicitReinstallation` | Couvert : révision commune des clés/liens, scope complet, revalidation sous transaction, refus d'entrée périmée/incomplète, rollback, reconstruction/reopen et snapshot WAL. Pas de recalcul persistant implicite. |
| Recherche indexée | [Recherche→reconstruction](../internal/storage/sqlite/search_reconstruction_test.go) ; `TestSearchReconstructionReadsCompleteQueueScopeAcrossCriteria`, `TestSearchReconstructionLateImportRequiresCompleteRefresh` | Couvert : six critères, pagination distincte du scope de corrélation, IDs recyclés/faits non datés conservés après relecture complète. [Mesures115](search-measurements.md) limitées à recherche/migration. |
| Rétention | [Intégration](../internal/storage/sqlite/retention_integration_test.go) ; `TestRetentionIntegrationSearchRefreshUsesRemainingFullScopeAndRejectsOldFacts`, `TestRetentionIntegrationNonemptyCompletedImportRetryUsesCommitmentAndAnchorChecks` | Couvert : purge bornée/atomique, invalidation des manifests, marqueurs de rejeu, checkpoints conservés, WAL et import retry/reopen. Pas d'effacement sécurisé ni politique automatique. |
| Absence de fusion sans preuve | [Origines](../internal/correlation/partition_test.go), [contexte explicite](../internal/correlation/continuity_instances_test.go) ; `TestContinuityInstancesBindKeysWithoutMergingOrClearingReserves` | Couvert : origines distinctes, réserves préservées. Contrat/clés120–122 ne produisent pas une preuve physique et ne sont pas un nouveau consommateur dans BuildProjection/SQLite. |

Les résultats détaillés restent dans [revue des destinataires](reviews/recipient-projection.md),
[NOQUEUE](reviews/prequeue.md), [liens](reviews/queue-links.md),
[identités](reviews/projection-identities.md), [stockage](reviews/projection-storage.md),
[recherche](reviews/search.md), [rétention](reviews/retention.md) et
[continuité](reviews/continuity.md). PR #19–25, #27–28 fusionnées ; dernière CI main
37543982877 entièrement réussie sur7ec6dd737681f4af878b8cdc4c8b71de0deb4308.

## Premier risque d'intégration à couvrir — lot124

La règle pure de conflit à date égale est testée, et la persistance/reconstruction
générale aussi. L'inventaire des tests SQLite n'a pas trouvé de contrôle dédié de
leur combinaison après fermeture/réouverture : ne pas appeler cette combinaison
validée par la seule présence des deux familles de tests.

Résultat attendu124 : deux tentatives natives contradictoires à date maximale
égale restent deux preuves, avec résultat unknown et OrderUncertain. La synthèse
garde latest_order_uncertain/unknown_result/coverage_unproven et ne compte pas de
succès. IDs d'insertion, ordre des records et reopen ne départagent pas le conflit.

Préparation : variante synthétique de `07-deferred-then-sent`, avec date native
du rapport sent alignée sur deferred avant parsing/Commit. Raw et hypothèses de
date doivent rester cohérents ; aucun log réel, aucune mutation des faits déjà
committés. Réutiliser la construction des batches du corpus, sans changer les
fixtures historiques pour les autres tests.

Vérifier Commit → CorrelationFacts → InstallProjection → CurrentProjection, puis
Close/Open → CurrentProjection. Deux bases avec records insérés dans des ordres
opposés doivent conserver les mêmes références/révisions et résultats. La voie
ordinaire, sans attestations, est le périmètre de ce contrôle.

Ce manque de vérification n'est pas un défaut observé du runtime. Ajouter un test
ciblé ; corriger uniquement si ce test révèle un défaut, puis documenter le résultat.

## Mesures et décision de sortie

L'inventaire des benchmarks existants ne trouve que recherche et migration de
domaines. Reconstruction pure, installation et lecture du manifest n'ont pas de
mesure publiée dans ce dépôt. Les mesures suivantes doivent couvrir ces opérations
avec protocoles/sorties brutes, tailles synthétiques dans les limites actuelles,
préparation hors chronométrage et distinction CPU/allocations/IO. Pas de seuil CI
ni extrapolation en capacité de production depuis un poste Windows.

Après124 : mesurer reconstruction/installation/lecture, puis relire la matrice,
les résultats et les limites avant de déclarer M3 terminé. Réutiliser les tests
scellés ; élargir seulement pour un risque concret. Les logs incomplets demeurent
un cas avec réserves. Une future fusion prouvée exige producteur fiable,
revalidation et règles propres ; elle n'est pas implicitement livrée par ce bilan.

## Vérifications du lot123

Lectures ciblées du cadrage, des revues et des tests, inventaire des benchmarks,
contrôle des noms de tests/liens de la matrice et git diff --check. Runtime/tests
inchangés, aucune suite locale relancée ni mesure faite. Publication et CI123 à
vérifier. Le lot123 est un bilan de critères, pas une nouvelle fonction livrée.
