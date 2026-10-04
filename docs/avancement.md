# Avancement et estimation jusqu'au MVP

État au 4 octobre 2026, après le lot 53. Référence de périmètre :
[plan M0–M5](phase-0-proposal.md#11-roadmap-et-critères-mvp).
État technique et prochaine action : [point de reprise](reprise.md).

## Vue d'ensemble

Six jalons avant le MVP : M0 et M1 terminés, M2 en cours, M3 à M5 à réaliser.
Il reste donc quatre jalons à clôturer, dont un déjà commencé.
Le MVP visé est installable sur Linux, avec ingestion et reprise, reconstruction
prudente des messages/destinataires, recherche Web authentifiée et paquets natifs.

Les lots numérotés sont des unités de reprise après quota. Le numéro 53 compte
surtout les petites étapes FileSource et les revues/corrections des fondations.
Il ne mesure pas un pourcentage du MVP. Une PR développée mais encore en revue
n'est pas comptée comme fusionnée ; un socle de CI n'est pas un paquet installable.

## Estimation des lots restants

Fourchettes de planification, pas engagements ni décompte d'un backlog déjà entièrement
détaillé. Elles incluent développement, tests, documentation et revues habituelles,
à périmètre MVP constant. Les jalons non commencés ont une incertitude élevée.

| Jalon | État constaté | Livrable restant / critère de fin | Lots restants estimés |
| --- | --- | --- | ---: |
| M0 — cadrage | Terminé : nom, MIT, architecture et décisions validés | Réviser les décisions seulement si un risque concret le justifie | 0 |
| M1 — faits Postfix | Terminé : parseurs/corpus/tests, PR #9 fusionnée | Maintenir les régressions pendant les étapes suivantes | 0 |
| M2 — ingestion | En cours : SQLite fusionné ; FileSource développé, PR #11 en revue | Clôture FileSource, récupération des états encore bloquants, import normal/gzip et validations de reprise/import | 15–25 |
| M3 — reconstruction | À réaliser | Instances/générations, destinataires/tentatives, NOQUEUE, liens prouvés, recalcul, recherche indexée et rétention validés sur corpus | 18–30 |
| M4 — consultation sûre | À réaliser | CLI de diagnostic, compte local/sessions, API bornée, recherche/détail/timeline Web, sécurité et accessibilité vérifiées | 15–25 |
| M5 — installation pilote | CI Go partielle existante ; livraison à réaliser | Exécutable/service, paquets, sauvegarde/restauration, installation Linux, sécurité de release et mesures de charge/pilote | 10–18 |
| **Total jusqu'au MVP** | **Quatre jalons à clôturer** | **Application installable répondant aux critères du cadrage** | **58–98, soit environ 60–100** |

Pas d'estimation en jours à partir des heartbeats : quota, disponibilité des outils,
CI et défauts découverts font varier la durée. Les extensions AD/OIDC/Keycloak sont
après MVP et ne sont pas incluses dans ce total.

## Chantier immédiat : terminer la PR #11

Estimation : environ 6–8 petits lots à partir du lot 53 terminé, incluse dans M2.
Les domaines déjà implémentés restent à relire et intégrer :

1. Observation du courant parmi les fichiers rouverts.
2. Transfert au suivi, avec courant connu/nouveau/absent et conservation de propriété.
3. Préparation orchestrée de reprise.
4. Politique zéro et diagnostics de reprise/lacune.
5. Démarrage Run, garde et configuration.
6. Synthèse de revue, validation finale et fusion sur la tête vérifiée.

Certains domaines nécessitent deux lots courts ; un défaut concret peut ajouter
un correctif. Critère de clôture : chemins restants revus, défauts corrigés,
CI verte et PR #11 fusionnée. Cette clôture ne termine pas tout M2 : import et
décisions de récupération ouvertes restent nécessaires.

## Pourquoi les prochains jalons ne devraient pas répéter 53 lots chacun

FileSource combine données partielles, rotation, identité filesystem, reprise,
transactions et propriété des descripteurs. Son développement a été découpé très
finement, puis suivi d'une longue série de revues séparées. Ce nombre n'est pas
un modèle à appliquer mécaniquement aux étapes suivantes.

Conserver des lots courts, mais traiter autant que possible un comportement avec
ses tests utiles, sa documentation et sa revue dans le même lot. Prévoir la revue
pendant le développement et la fusion de périmètres cohérents sans accumuler tout
un jalon dans une PR très large. M3 restera complexe ; les fourchettes doivent
être réévaluées à sa préparation plutôt que supposées exactes aujourd'hui.

## Suivi à maintenir

Chaque clôture indique : jalon/chantiers, livrable effectivement validé, prochain
résultat attendu, reste jusqu'au critère de fin. Réviser ces estimations après la
fusion de #11, puis à chaque clôture de jalon ou changement de périmètre substantiel.
Une revue sans changement de comportement ne doit pas être présentée comme une
nouvelle fonctionnalité livrée. Le rapport d'avancement n'incrémente pas les lots
de développement : le dernier lot validé reste 53.
