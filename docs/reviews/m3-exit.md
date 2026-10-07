# Relecture de sortie M3 — lot127

## Décision et périmètre

Relecture locale assistée de la PR #29, lots123–126 sur
`f4c24d22e64baa8d09a2ec0583ae46294882dfac`, base main
`7ec6dd737681f4af878b8cdc4c8b71de0deb4308`. Aucun défaut bloquant identifié.
Cette relecture n'est pas une approbation humaine indépendante.

Sortie M3 favorable dans le périmètre **bibliothèque** du
[cadrage](../phase-0-proposal.md#11-roadmap-et-critères-mvp) : QueueInstances,
tentatives par destinataire, NOQUEUE, liens de réinjection, états prudents,
recherche indexée et rétention. La [matrice](../m3-exit-checklist.md) relie ces
critères aux tests/revues acquis. La clôture effective reste conditionnée à la
CI finale127, la fusion de #29 et la CI main correspondante.

CLI/configuration, auth locale, API/Web relèvent de M4. Service/paquets,
sauvegarde et pilote Linux représentatif restent M5 ; AD/OIDC après MVP.
La fin de M3 ne signifie pas une application installable ou une couverture
certifiée des journaux. Les issues aux critères applicatifs restent au backlog.

## Points relus

| Point | Résultat |
| --- | --- |
| Critères de reconstruction | Générations/IDs recyclés, dates incertaines, provenances distinctes, tentatives et transports reliés aux contrôles déjà scellés ; pas de nouveau verdict global |
| NOQUEUE et réinjection | Scope sans queue explicite ; corroboration des endpoints et mappings requis, réserves conservées ; Message-ID/PID/adresse seuls ne fusionnent pas les parcours |
| Composition/stockage | Snapshot complet et manifests révisables, refus stale/incomplet, revalidation transactionnelle et lecture WAL/reopen vérifiés par #23–24 ; pas de réparation persistante implicite |
| Recherche/rétention | Six critères exacts, pagination distincte de la reconstruction complète ; purge bornée des faits éligibles, invalidation atomique et garde de rejeu, checkpoints/imports préservés ; politique automatique et sauvegarde applicative restent à livrer |
| Test124 | Native sent daté comme deferred avant parsing/Commit ; six faits, deux bases, inversion réelle des IDs SQL, deux Latest/tentatives, unknown/OrderUncertain, DSN/réponses/relais et quatre réserves après reopen ; aucun faux succès |
| Helper partagé | Seule extraction de la construction du batch vers un helper acceptant les raw ; comportement des corpus ordinaires inchangé, suite/vet SQLite acquis au lot124 |
| Benchmarks125 | Quatre profils bornés4096faits/64parts, mêmes entrées natives pour les trois opérations, warmup/préparation hors timer ; Install inclut revalidation et remplacement, Current reconstruit réellement |
| Méthode126 | 36échantillons intégraux,5opérations/3répétitions, moyennes puis médianes/plages ;24cellules numériques confrontées aux sorties au lot126 ; allocations cumulées distinctes d'un pic/RSS |
| Limites des mesures | Build1024 bruité, pas de comparaison causale/SLA ; profil régulier/cache chaud Windows, sans coût de liens/NOQUEUE/origines multiples/concurrence/pilote Linux |
| CI ajoutée | Smoke Linux Go1.26 exécuté1x sur12cas, sans seuil de temps ; reste des contrôles de format/vet/tests/race/builds statiques conservé |
| Continuité | Contrat/clés #28 ne produisent pas de preuve physique ; origines non prouvées distinctes, aucun nouveau consommateur d'attestations dans BuildProjection/SQLite |

Le diff de #29 comprend tests, benchmarks, trois lignes de CI et documentation,
sans modification de code de production. Les fixtures partagées restent inchangées.
Le lot127 ne modifie ni tests, ni benchmarks, ni workflow, ni dépendances.

## Vérifications acquises et réutilisées

- Lot123 : noms/liens de la matrice et diff vérifiés ;
  [CI37546917054](https://github.com/Coubiac/QueueAtlas/actions/runs/37546917054) entière réussie.
- Lot124 : test ciblé/deux ordres, suite/vet SQLite/format/diff Windows ;
  [CI37549552434](https://github.com/Coubiac/QueueAtlas/actions/runs/37549552434) entière réussie.
- Lot125 : smoke local12cas et vet/diff ;
  [CI37552260931](https://github.com/Coubiac/QueueAtlas/actions/runs/37552260931) entière réussie,
  smoke Linux Go1.26 passé.
- Lot126 : campagne répétée et tableaux vérifiés, aucune valeur supprimée ;
  [CI37554852072](https://github.com/Coubiac/QueueAtlas/actions/runs/37554852072) entière réussie
  surf4c24d2, trois jobs/SHA exact vérifiés REST à la reprise127.

Diff complet #29 relu, roadmap/ADR/matrice/contrats et résultats précédents
confrontés. Aucun risque nouveau justifiant une remesure ou un rerun local des
fondations. Format/diff documentaire127 à vérifier avant commit ; nouvelle CI
finale et main à contrôler après publication/fusion.

## Suite et état au moment de l'enregistrement

Relecture favorable, M3 techniquement prêt à clôturer. Publication/CI finale127,
revue COMMENT assistée sur tête exacte, passage ready, fusion et CI main encore
à terminer. Ne pas annoncer la fusion sur la seule décision favorable.
Après clôture effective : M3 zéro lot restant, M4 15–25 et M5 10–18, soit25–43MVP,
estimations incertaines. Premier lot M4 proposé128 : point d'entrée CLI
`queueatlas version`, petit lot autonome ; détailler la configuration dans le lot
suivant. Conserver MIT et l'auth AD/OIDC après MVP.

## Clôture effectivement vérifiée

Lot127 publié surd26e0bd97987b42417853338cab8fac6d7105675,
[CI finale37557272778](https://github.com/Coubiac/QueueAtlas/actions/runs/37557272778)
entièrement réussie, trois jobs/SHA exact vérifiés REST. Revue COMMENT assistée
5436561773, passage ready et fusion #29 sur038c6c90ee515d584669a8d879b5e267d30d8f13.
[CI main37557390479](https://github.com/Coubiac/QueueAtlas/actions/runs/37557390479)
entièrement réussie sur ce merge, trois jobs/SHA exact vérifiés. Main local actualisé
propre, branche du chantier supprimée localement et sur GitHub. M3 terminé en
bibliothèque ; états conditionnels précédents conservés comme historique précommit.
Clôture revérifiée à la reprise128, aucun rerun de127.
