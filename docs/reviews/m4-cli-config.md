# Relecture CLI/configuration — lot132, PR #30

## Décision et périmètre

Relecture locale assistée sur `53d4ae07e8454f9dea297a2c731274634d2f17d2`, base
`038c6c90ee515d584669a8d879b5e267d30d8f13`. Aucun défaut bloquant identifié dans
le chantier128–131. Cette relecture n'est pas une approbation humaine indépendante.
Fusion favorable après CI finale132 sur la tête exacte, puis contrôle CI main.
Ce chantier ne clôture pas M4 : diagnostics, auth locale et API/Web restent à livrer.

## Points vérifiés

| Point | Résultat et limites |
| --- | --- |
| Dispatcher CLI | Version/aide sans IO applicative ; check-config exige la forme canonique, pas de fichier implicite, arguments inconnus non recopiés ; os.Exit transmet les codes |
| Version | `dev` par défaut, étiquette de linker déclarée ; aucune certification d'une release |
| Config/défauts | Défauts indépendants, zéro explicite/null refusés, loopback littéral/port1–65535, délais bornés ; aucun mode d'écoute distante sans contrat auth/TLS |
| YAML | 64 Kio avant parsing, UTF-8/document unique ; contrôle profondeur4/128nœuds après parsing, ancres/alias/merge/doublons/champs inconnus/types implicites/custom tags refusés ; pas d'expansion d'alias |
| Chemins | Syntaxe contrôlée avant résolution ; stockage relatif depuis le répertoire lexical absolu de config ; ambiguïtés de lecteur Windows refusées ; aucune expansion env/~/template ou confinement revendiqué |
| IO config | Type régulier avant/après ouverture, fermeture succès/échec, configuration zéro en erreur ; aucune attestation de permissions/ACL/stabilité concurrente, aucune échéance IO |
| Diagnostics | ErrInvalid nomme seulement champ/règle connus ; ErrRead et échec stdout fixes ; aucun argument/chemin/valeur/champ inconnu ou erreur brute recopié |
| Effets CLI | Check-config ne crée/ouvre pas de DB, ne modifie pas la config, ne démarre aucun composant ; validation syntaxique distincte d'un diagnostic de déploiement |
| Tests | Sept tests CLI avec binaire compilé/linker/codes0/1/2/flux/absence DB, douze tests config avec fichiers synthétiques/lecteur défaillant/bornes/chemins Windows/exemple |
| CI/dépendances | Étapes Windows CLI/config ajoutées ; jobs Linux test/vet/race/builds statiques existants conservés ; yaml/v3 v3.0.5 et deux checksums figés, autres dépendances inchangées |

Les sources/testeurs M2/M3 de production sont inchangés. Le diff comprend aussi
l'enregistrement de la clôture M3 déjà vérifiée au lot128. Le lot132 ne modifie
ni code, tests, workflow, dépendances, ni exemple YAML.

## Vérifications réutilisées

- Lots128–130 : tests/vet/config/CLI et exemples Windows acquis, CI entières
  37559870551, 37562294743 et 37564852420 réussies. `go mod verify` passé au lot130.
- Lot131 : sept tests CLI/vet/format/diff et go run sur l'exemple Windows passés ;
  [CI37567003805](https://github.com/Coubiac/QueueAtlas/actions/runs/37567003805)
  entière réussie, SHA exact et trois jobs revérifiés REST à la reprise132.
  Les jobs Linux couvrent aussi tests config, vet et builds statiques amd64/arm64.

Code/contrats/CI/tests acquis confrontés. Aucune modification ou risque nouveau
ne justifie un rerun local des fondations. Diff documentaire132 à contrôler avant
commit ; CI finale et main à vérifier après publication/fusion.

## Suite et état avant publication

Relecture favorable ; publication/CI132, revue COMMENT assistée, passage ready,
fusion #30 et CI main encore à terminer lors de cet enregistrement. Respecter les
SHA exacts avant fusion ; ne pas annoncer la fusion sur cette seule décision.

Prochain lot133 après clôture : ouverture SQLite en lecture seule pour les futurs
diagnostics `db stats`/`doctor`, sans création ou migration de base. Ne pas utiliser
l'ouverture applicative courante comme un diagnostic readonly. Définir et tester
ce contrat de stockage avant de raccorder une nouvelle commande CLI.
M4 reste10–20lots/M5 10–18, total20–38 après132, estimation incertaine. Service,
paquets/pilote restent M5 ; MIT conservée, AD/OIDC après MVP.

## Clôture effective, consignée à la reprise133

Finale132 `16ee19cf7ad3ab0774d4b26fac5d578f2287f6a3`,
[CI37569190697](https://github.com/Coubiac/QueueAtlas/actions/runs/37569190697)
entière réussie, SHA exact/trois jobs vérifiés. COMMENT assisté5437464734 puis ready
et fusion #30 sur `118634fed56640fc4dd3825945d7dd20cfd5c630` ;
[CI main37569292737](https://github.com/Coubiac/QueueAtlas/actions/runs/37569292737)
entière réussie, SHA exact/trois jobs vérifiés. Main actualisé propre et branche
codex/m4-cli supprimée local/GitHub. Les mentions d'attente précédentes sont le
snapshot avant publication132 ; la clôture effective est acquise, M4 reste ouvert.
