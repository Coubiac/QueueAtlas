# Avancement et estimation jusqu'au MVP

État au 5 octobre 2026, après diagnostic/intégration87 et clôture88 fusionnés
(PR #18, CI finale et main réussies). Projections pures M3 validées aux lots89–92,
CI verte ; clôture93 de #19 fusionnée, CI finale et main réussies. Expiration94 et
synthèse95 fusionnées dans #20 ; clôture96 terminée, CI finale/main réussies.
Rapports NOQUEUE97 et sessions98 fusionnés #21, clôture99 terminée avec CI finale/main
vertes. Indices natifs100 et relations101 fusionnés dans #22, clôture102 terminée,
CI finale/main réussies. Clés103 et composition104 fusionnées #23, clôture105
terminée/CI finale et main vertes. Lecture SQLite106 et schéma107 publiés dans #24,
CI vertes ; installation108/lecteur109 et clôture110 fusionnés dans #24 après
CI finale entière réussie. Main110 actualisé, CI push en attente ; recherche111 en revue.
Référence de périmètre :
[plan M0–M5](phase-0-proposal.md#11-roadmap-et-critères-mvp).
État technique et prochaine action : [point de reprise](reprise.md).

## Vue d'ensemble

Six jalons avant le MVP : M0 et M1 terminés, socle M2 fusionné en bibliothèque,
M3 commencé, M4/M5 à réaliser. Il reste trois jalons à clôturer. Cette sortie M2
suit les livrables de la roadmap ; les issues
#4/#5 restent ouvertes pour leurs critères applicatifs aux jalons suivants.
Le MVP visé est installable sur Linux, avec ingestion et reprise, reconstruction
prudente des messages/destinataires, recherche Web authentifiée et paquets natifs.

Les lots numérotés sont des unités de reprise après quota. Le numéro 111 compte
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
| M2 — ingestion | Socle fusionné : SQLite/FileSource/reprise/import (#10–17), diagnostics et intégration #18 CI finale/main vertes | CLI/exporteur/projections et preuves de chevauchement applicatives restent au backlog des jalons suivants | 0 |
| M3 — reconstruction | En cours : projections pures et stockage #19–24 fusionnés, CI finale verte ; recherche111 en revue | Continuité prouvée, complétude applicative, fin recherche et rétention, intégration/mesures | 10–18 après clôture110, incluant validation111 |
| M4 — consultation sûre | À réaliser | CLI de diagnostic, compte local/sessions, API bornée, recherche/détail/timeline Web, sécurité et accessibilité vérifiées | 15–25 |
| M5 — installation pilote | CI Go partielle existante ; livraison à réaliser | Exécutable/service, paquets, sauvegarde/restauration, installation Linux, sécurité de release et mesures de charge/pilote | 10–18 |
| **Total après clôture110** | **Trois jalons à clôturer** | **Application installable répondant aux critères du cadrage** | **35–61** |

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

Les compléments M2 ont occupé trois lots86–88, dans la prévision 2–5 : qualification
missing/gap/degraded sans PII, intégration des vrais parsers/import/SQLite et des
provenances distinctes, contrat de reprise/chevauchement puis clôture. CI87 verte,
revues sans blocage. PR #18 fusionnée sur `8886427`, CI finale37288318666 et push
main37288519438 success vérifiés sur les SHA exacts.

La CLI hors service actif, l'exporteur et la projection canonique indépendante de
l'ordre restent des dépendances ultérieures des issues #4/#5 ; elles restent
ouvertes. Aucun claim de déduplication inter-source/FileSource. Les inconnus
multiples restent refusés, sans nouveau mécanisme administratif implicite.

## Bilan du premier chantier M3 et suite

Quatre comportements purs89–92 validés : résultat/portée d'une tentative, index
candidat par instance et flux, générations candidates après frontières observées,
puis tentatives et dernier résultat observé par adresse exacte. CI89–92 vertes,
revues sans blocage après correction des preuves natives et des dates contradictoires.
Lot93 clôture ce chantier avec la fusion de #19 ; il ne termine pas M3.

Les générations restent séparées par provenance, sans identité globale entre
fichiers ni preuve de continuité. Dates inconnues/frontières douteuses restent
non résolues ; aucun statut global, persistance de projection, recherche ou Web.

Expiration explicite et comptes/réserves traités aux lots94–95 ; clôture96 fusionnée
avec #20, finaleCI37295702313 et main37295859068 success vérifiées. Ils ne certifient
pas la couverture et n'inventent pas de destinataires
expired. Le résumé conserve les résultats des tentatives et les rapports qmgr
distincts. Le travail de complétude/NOQUEUE inclut les critères applicatifs restants.

NOQUEUE et fenêtres de sessions candidates traités aux lots97–98, clôture99 fusionnée
avec #21, CI finale37323621470 et main37323880818 success vérifiées. Aucun lien de
file accepté, identité globale par PID ni couverture certifiée.
La réestimation vient des comportements encore nécessaires, et non d'une simple
soustraction de trois au compteur. Ces regroupements restent à découper :

| Travail M3 restant | Lots estimés |
| --- | ---: |
| Continuité prouvée entre origines et intégration des clés révisables ; liens et clés/composition100–105 fusionnés | 1–3 |
| Complétude applicative restante (avec réserves existantes) | 0–1 |
| Persistance/reconstruction au snapshot : chantier106–110 fusionné, CI finale verte | 0 |
| Recherche indexée et rétention cohérente : adresse111, autres critères/index, domaines, intégration et suppression avec invalidation | 6–8 |
| Intégration corpus, mesures et revue/clôture M3 | 3–6 |
| **M3 restant après clôture110** | **10–18** |

M4 et M5 gardent leurs fourchettes. Le backlog applicatif #4/#5 est inclus dans
les jalons suivants ; aucun critère de MVP n'est supprimé par ces clôtures.

Les lots100–101 conservent les indices natifs puis corroborent des relations
SMTP/local/bounce sous preuves des deux côtés. CI37324927168 et37326579775 success,
revues favorables après correction de la répétition des preuves positives. Le
lot102 clôture la fusion de #22, CI finale37327741368 et main37328022256 success
vérifiées sur les SHA exacts. Clés103/CI37329247549 success, composition104 publiée
avec revue favorable et CI37330343331 entière success. Clôture105 terminée,
#23 fusionnée sur50ba6a7, CI finale37330738903 et main37331003071 success vérifiées.
Lecture106 et schéma107 publiés dans #24, CI37333408517 et37336085923 success.
Installation108/lecteur109 et clôture110 fusionnés dans #24 suraaa95f8,
CI finale37368191438 entière success après relance d'une annulation de runner stable.
CI main37372128796 en attente ; recherche111 développée/testée en revue.
Le schéma et les memberships conservent tous les faits, y compris les réserves ;
aucun résultat dérivé n'est sérialisé. Le chantier de stockage est fusionné ; CI main
à vérifier. La prévision2–4 lots après107 a couvert108–110. Recherche111 développée
et testée ; autres critères/index/domaines et rétention explicitement séparés :
6–8 lots au lieu de4–6, incluant validation111 et clôture de ces chantiers.
Continuité entre origines, recherche et rétention restent séparées. Réestimation
10–18 M3, 35–61 total après110, remplace10–20/35–63 après107 ; critères inchangés.
Les lots de clôture ne livrent pas de nouveaux
comportements ; le compteur n'est pas décrémenté mécaniquement à chaque PR.

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
