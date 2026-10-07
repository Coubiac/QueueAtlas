# Relecture diagnostics SQLite/CLI — lot136, PR #31

## Décision et périmètre

Relecture assistée favorable des lots133–135 sur
`cae194cee0bc39e422e0b1ac8da0bd6e78cd7401`, base
`118634fed56640fc4dd3825945d7dd20cfd5c630`. Aucun défaut bloquant identifié dans
ce périmètre. Ce n'est pas une approbation humaine indépendante et ne clôture
pas M4. Fusion après CI finale136 sur la tête exacte, puis contrôle CI main.

## Points confrontés au code, au contrat et aux tests

| Point | Constat / preuve acquise |
| --- | --- |
| Ouverture | Handle Diagnostics distinct de Store, fichier existant ; aucun appel à Open applicatif/migrate/journal_mode=WAL. Absence/annulation/non-régularité refusées sans création. |
| Droits/chemins | Stat base/auxiliaires, refus FIFO/droits Unix exposés avant connexion ; URI échappée, chemin Unicode/caractères URI testé. Parent protégé requis, symlinks suivis/observations sans verrou/ACL Windows non attestées documentés. |
| Connexion | mode=ro et query_only, FK/trusted_schema/defensive/DQS et busy5s ; pool limité à une connexion, paramètres de remplacement testés. Écritures refusées même avec query_onlyOFF. |
| Compatibilité | Version courante et historique attendu dans une transaction ; anciennes/futures/zéro/étrangères/historique absent ou vue refusés. Indicateurs de compatibilité, pas authenticité ni schéma/intégrité complète. |
| Métadonnées | Six champs, chaînes/valeurs bornées, requêtes scalaires fixes, historique revérifié au même snapshot ; zéro résultat en erreur, erreurs fixes/contexte conservé. |
| Concurrence/WAL | Commit visible avant checkpoint, non-commit invisible, nouveaux commits visibles au prochain snapshot ; snapshot conservé pendant croissance et changement atomique de version, prochain appel incompatible refusé. Pas immutable/nolock. |
| Effets | Images rollback comparées octet pour octet et WAL vivant conservé ; aucune migration/checkpoint applicatif. SQLite peut utiliser/créer ses auxiliaires ; zéro écriture auxiliaire non promis. |
| CLI | Dispatch db, syntaxe/config explicites, aides sans IO, chargeur existant ; code0/1/2, messages fixes sans pilote/path/valeur, stdout vide sur erreurs avant sortie. |
| Sortie/ressources | Fermeture avant sérialisation/sortie ; projection JSON explicite indépendante des champs futurs de bibliothèque, six noms/newline. Erreur stdout/code1 ; octets partiels possibles sur flux défaillant. |
| Bornes/sens | Contexte DB coopératif10s après configuration, pas deadline dure IO ; pages logiques avec WAL, pas taille physique/espace disque/compteurs messages/intégrité/sauvegarde. |
| CI/dépendances | Étape Windows diagnostics plus CLI existante ; jobs Linux suite/vet/format/smoke/race source/file/builds statiques. Aucune dépendance nouvelle, tests synthétiques, MIT conservée. |

Code relu : diagnostics.go/diagnostic_metadata.go et leur helper de validation,
db.go/dispatch et tests, contrats immédiats et diff du chantier depuis main132.
Les changements documentaires de133 consignaient aussi la clôture acquise de #30.

## Vérifications effectivement acquises

- Cinq tests portables d'ouverture133 et un test Linux FIFO/droits ; quatre tests
  métadonnées134. Cas de corruption/refus, rollback/WAL, reconnexion, annulation,
  résultat zéro, privacy et snapshot avec écrivain indépendant.
- Onze tests CLI135, dont quatre nouveaux pour db stats et binaire compilé étendu
  aux codes0/1/2/JSON ; base et config inchangées, absence sans création, aide/arguments
  et sortie échouée. Tests/vet/format/diff Windows déjà passés au lot135.
- [CI37574528679](https://github.com/Coubiac/QueueAtlas/actions/runs/37574528679)
  entière réussie sur cae194c ; workflow et trois jobs/SHA exact revérifiés REST
  à la reprise136. Windows CLI/diagnostics et Linux passés.

Lot136 documentaire seulement : code/tests/workflow/dépendances inchangés. Aucun
risque nouveau justifiant un rerun local des fondations. Diff documentaire à
contrôler avant commit ; CI finale et CI main encore à vérifier après publication.

## État avant publication et suite

Relecture favorable ; publication/CI136, revue COMMENT assistée sur tête exacte,
ready/fusion #31 et CI main encore à terminer lors de ce snapshot. Ne pas annoncer
une clôture effective sur cette seule revue. Après succès : main actualisé propre,
branche diagnostics nettoyée, M4 toujours en cours.

Prochain lot137 : `doctor --config` en lecture seule, rapport limité à configuration
et compatibilité SQLite effectivement vérifiées, sans attestation d'intégrité ou
d'aptitude au déploiement. Réutiliser le chargeur/lecteur validés, sans création DB.
M4 reste6–16lots, M5 10–18, total16–34 après136 (estimation incertaine). Doctor,
auth/API/Web/config des composants restent M4 ; service/paquets/pilote restent M5.
MIT conservée, AD/OIDC/Keycloak après MVP.
