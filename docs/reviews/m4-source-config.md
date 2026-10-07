# Relecture et clôture de la configuration source fichier — lot142

Périmètre : [PR #33](https://github.com/Coubiac/QueueAtlas/pull/33), lots139–141,
tête `223849c950beac3f48dd22695697db8bd3d5533c`, base main1380d2cad4.
Résultat attendu142 : bilan, publication/CI finale, revue COMMENT assistée sur tête
finale, fusion/CI main et nettoyage. Aucun comportement indépendant ajouté.

Relecture assistée favorable : aucun défaut bloquant identifié dans ce périmètre.
Ce bilan ne constitue pas une approbation humaine indépendante.

## Contrats confrontés au code et aux preuves

| Point | Résultat de la relecture | Preuve acquise |
| --- | --- | --- |
| Présence et défauts | Source nil si absente ; ID/nom/chemin requis ; champs absents seulement gardent leurs défauts ; zéro/null/mode vide refusés | Config, FileSourceDefaults/Validate, TestLoadSourceDefaultsAndNoSourceIO et tables de refus |
| Identité | ID ASCII128, nom UTF-8128 sans contrôles/espaces périphériques, TrustedHost facultatif ASCII255 ; aucune résolution DNS | Tests identité139, erreurs fixes et validation sans mutation |
| Chemins | Validation lexicale avant résolution ; relatifs depuis répertoire YAML, revalidation de longueur après ; pas d'expansion | Tests syntaxe139 et chemins140, Windows ambigus refusés ; absolu exigé par LibraryConfig141 |
| YAML | Mapping unique d'une source fichier ; champs inconnus/doublons/ancres/alias/tags refusés ; aucun type ou override de reprise | readFileSource/readMapping/checkTree, tables140 ; anciennes configs sans source restent valides |
| Budgets et temps | Entiers décimaux canoniques non quotés, conversion bornée ; durées positives dans les limites de la bibliothèque | readBudget/readDuration/Validate ; bornes inclusives/débordements/zéros, entiers explicitement tagués et quotés testés |
| Bornes globales | 64Kio,128nœuds,profondeur4,un document UTF-8 conservés ; pas d'expansion d'alias | Chargeur existant et TestDecodeSourcePreservesByteAndStructureBounds |
| Échecs sûrs | Config zéro/Source nil pour erreur de chargeur ; config bibliothèque zéro pour erreur de conversion ; champ/règle fixe sans valeur privée | Tables140/141 et branches d'erreurs relues |
| Propriété | Source distincte pour chaque Decode ; conversion par valeur sans pointeurs/mutations ; Config copié ne fait pas une copie profonde | Tests ownership140/141 et contrat documenté |
| Conversion | ID/Name/TrustedHost, Kind=file, Path, StartAt typé, délais et budgets copiés ; ResumePolicy strict, AllowZeroCheckpoint=false | TestFileSourceLibraryConfigPreservesSettingsAndOwnership, refus relatif/revalidation141 |
| Compatibilité | Config convertie acceptée par New beginning/end avec dépendances sentinelles, sans lecture d'état/normalisation/création | TestFileSourceLibraryConfigAcceptedByConstructorWithoutIO ; aucun Run |
| CLI et effets | Exemple et source/journal absent acceptés sans ingestion ; doctor conserve config/compatibilité SQLite | Exemple synthétique140, test du binaire CLI étendu et documentation |
| Intégration | CLI production/stockage/FileSource/modules/workflow inchangés ; config testée Windows/Linux | Diff139–141 et CI37591355873 entière/trois jobs sur SHA exact |

## Limites et reste du jalon

Une seule source fichier configurable ; sources multiples/autres types ultérieurs.
Validation lexicale sans vérification de lisibilité, droits, format, inode, liens,
montage ou checkpoints. Chemins relatifs peuvent sortir du répertoire config via
`..` ; aucune restriction à ce répertoire ni verrouillage contre substitution.
TrustedHost est une clé d'instance configurée, pas une preuve d'hôte du journal.
Le mot end ne démontre pas les conditions physiques/durables du bootstrap ;
Run conserve ses contrôles existants lors du futur raccordement.

La conversion ne construit ni ne lance de composant. L'appelant doit vérifier
Source non nil et traiter l'erreur ; aucune garantie d'immuabilité d'un Config
partagé entre goroutines. Load lit la configuration, pas le journal ou la base ;
ses limites de remplacement de chemin et de lecteur bloquant restent inchangées.
Doctor n'est pas élargi par cette PR. Aucun service/API/Web/compte/session livré.
La clôture de ce chantier ne termine pas M4 ni les critères applicatifs #4/#5.
MIT conservée, AD/OIDC/Keycloak après MVP.

## Vérifications acquises et état avant publication142

Quatre tests nouveaux139, cinq140 et trois141 ; vingt-quatre tests config et
vet/format/diff Windows acquis141. Quinze tests CLI et binaire réel étendu acquis140.
[CI14137591355873](https://github.com/Coubiac/QueueAtlas/actions/runs/37591355873)
entière réussie/trois jobs Windows/Linux Go1.26.x/stable sur223849c exact revérifiés
REST à la reprise142. Tests/vet/format Linux, smoke, race source/file et builds
statiques réussis ; config/CLI/diagnostics Windows réussis.

Fetch effectué, checkout propre, têtes locale/origin/PR identiques ; base main138
inchangée. PR ouverte en brouillon, mergeable/clean, aucune revue/thread en attente.
Lot142 documentaire seulement ; code/tests/workflow/dépendances inchangés, aucun
risque nouveau justifiant un rerun local des fondations. Vérifier diff documentaire
avant commit ; CI finale/revue COMMENT/ready/fusion/main encore à terminer au commit.
Consigner les SHA/runs effectifs dans la PR puis au prochain point de reprise.

Prochain lot143 : contrat pur du compte administrateur local et paramètres de
hachage bornés, tests/doc selon ADR-006. Persistance, CLI d'initialisation et
sessions/protections dans les lots suivants. M4 reste7–15lots, M5 10–18,
total17–33 après142, estimation incertaine à périmètre constant.
