# Avancement et estimation jusqu'au MVP

État au 5 octobre 2026, après import ordonné fusionné au lot85 (PR #17,
CI finale et main réussies). Référence de périmètre :
[plan M0–M5](phase-0-proposal.md#11-roadmap-et-critères-mvp).
État technique et prochaine action : [point de reprise](reprise.md).

## Vue d'ensemble

Six jalons avant le MVP : M0 et M1 terminés, M2 en cours, M3 à M5 à réaliser.
Il reste donc quatre jalons à clôturer, dont un déjà commencé.
Le MVP visé est installable sur Linux, avec ingestion et reprise, reconstruction
prudente des messages/destinataires, recherche Web authentifiée et paquets natifs.

Les lots numérotés sont des unités de reprise après quota. Le numéro 85 compte
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
| M2 — ingestion | En cours : SQLite/FileSource/récupération/end et import ordonné/reprise fusionnés (#10–17) | Compléments/diagnostics de source, état incertain des chevauchements et bilan de sortie M2 | 2–5 |
| M3 — reconstruction | À réaliser | Instances/générations, destinataires/tentatives, NOQUEUE, liens prouvés, recalcul, recherche indexée et rétention validés sur corpus | 18–30 |
| M4 — consultation sûre | À réaliser | CLI de diagnostic, compte local/sessions, API bornée, recherche/détail/timeline Web, sécurité et accessibilité vérifiées | 15–25 |
| M5 — installation pilote | CI Go partielle existante ; livraison à réaliser | Exécutable/service, paquets, sauvegarde/restauration, installation Linux, sécurité de release et mesures de charge/pilote | 10–18 |
| **Total jusqu'au MVP** | **Quatre jalons à clôturer** | **Application installable répondant aux critères du cadrage** | **45–78, soit environ 45–80** |

Pas d'estimation en jours à partir des heartbeats : quota, disponibilité des outils,
CI et défauts découverts font varier la durée. Les extensions AD/OIDC/Keycloak sont
après MVP et ne sont pas incluses dans ce total.

## Chantier achevé et suite immédiate

La PR #11 est fusionnée le 4 octobre sur `27b9d98bba1f51749f10e8b9930e9c4da0302068` :
sept lots de clôture après le lot 53, dans la fourchette 6–8 annoncée. Les 19 parties
de revue et la [synthèse](reviews/pr-11.md) sont conservées. Cette fusion ne termine
pas M2 ni toute l'issue #4.

La récupération unique est fusionnée le 5 octobre avec la PR #12 sur `69dfe6b` :
cinq lots 61–65, dans la fourchette 4–5. Classification, preuves et transition seule
livrées comme opération de bibliothèque, tests utiles/revue intégrés aux lots de
développement, CI main verte. Aucun unknown repris automatiquement par Run.

Le départ end est fusionné le 5 octobre avec la PR #13 sur `df7e8a3` : trois lots
66–68 comme prévu. Frontière complète, vide→append depuis zéro, reprises/rotations
et ACK perdus vérifiés, CI main verte. Toujours une bibliothèque, pas un service.

La prévalidation normale/gzip est fusionnée avec la PR #14 sur `85effe5` au lot 71 :
trois lots 69–71 comme prévu, taille/digest, limites, membres/CRC/EOF gzip vérifiés,
CI main verte. Aucune ingestion d'import encore livrée.

La copie privée est fusionnée avec la PR #15 sur `f8e58e3` au lot 74 : trois lots
72–74 comme prévu. Octets du digest conservés avant ingestion, propriété/read-only,
fermeture/échecs/cancel/cleanup vérifiés, CI finale verte. Manifest et ingestion futurs.

Le manifest est fusionné avec la PR #16 sur `677a618` au lot 79 : cinq lots 75–79.
Identité source/contenu, migration/lecture et préparation/progression atomiques
livrées en bibliothèque ; CI finale/main vertes, anciens imports non adoptés.
L'écriture a été séparée en préparation puis progression pour garder chaque lot
reviewable. La prévision précédente 5–8 pour manifest + application était trop
courte : le détail des contrats/transactions et deux défauts reproduits ont été
traités avant l'application. Ce chantier n'est pas encore un importeur complet.

Le chantier d'application est développé aux cinq lots 80–84 : constructeur prouvé,
CommitNext/ACK/EOF, association au CP partagé, pilote d'une tentative propriétaire,
puis liste ordonnée avec deadline globale et nombre borné. Le lot85 clôture les
revues/CI et la fusion sur `fcb6ad9`, soit six lots dans la fourchette 4–6 annoncée après79.
La revue a précisé qu'une sentinelle EOF du Sink ne prouve pas une fin acquittée ;
la régression et le pilote vérifient le status du run. Aucun défaut runtime non
corrigé identifié ; [synthèse](reviews/import-application.md).

Critères développés/vérifiés : contenu entier validé ingéré, reprise/rejeu par
provenance, pas de faux complete en cas de checksum/partial/annulation, bornes
de fichiers/durée. CI finale37265820369 et push main37265917012 success vérifiées.
Compléments du suivi #4 et diagnostics à valider séparément. Bibliothèque seule
jusqu'aux jalons CLI/service ; aucune CLI import livrée par ce chantier.
Les ensembles inconnus multiples restent refusés ; leur résolution administrative
ne doit pas devenir une reprise automatique sans preuves.

M2 réestimé 2–5 pour compléments et bilan : qualification missing/gap/degraded sans
PII, incertitude de chevauchement et conditions de reprise explicites, puis clôture.
La CLI hors service actif et la projection canonique indépendante de l'ordre restent
des dépendances ultérieures des issues #4/#5 ; elles restent ouvertes. Aucun claim
de déduplication inter-source/FileSource. Les inconnus multiples restent bloqués,
sans nouveau mécanisme administratif implicite. Autres jalons inchangés.

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
nouvelle fonctionnalité livrée. Une mise à jour d'estimation seule n'incrémente
pas les lots ; le lot 60 clôture la revue et la fusion FileSource.
