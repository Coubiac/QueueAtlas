# Avancement et estimation jusqu'au MVP

État au 7 octobre 2026, après diagnostic/intégration87 et clôture88 fusionnés
(PR #18, CI finale et main réussies). Projections pures M3 validées aux lots89–92,
CI verte ; clôture93 de #19 fusionnée, CI finale et main réussies. Expiration94 et
synthèse95 fusionnées dans #20 ; clôture96 terminée, CI finale/main réussies.
Rapports NOQUEUE97 et sessions98 fusionnés #21, clôture99 terminée avec CI finale/main
vertes. Indices natifs100 et relations101 fusionnés dans #22, clôture102 terminée,
CI finale/main réussies. Clés103 et composition104 fusionnées #23, clôture105
terminée/CI finale et main vertes. Lecture SQLite106 et schéma107 publiés dans #24,
CI vertes ; installation108/lecteur109 et clôture110 fusionnés dans #24 après
CI finale entière réussie. Main110 actualisé, CI push réussie ; recherche111–115
fusionnée/CI finale et main vertes ; rétention116–119 fusionnée #27, CI finale/main
vertes. Contrat et clés120–122 fusionnés dans #28, CI finale37543845063 et main
37543982877 entièrement réussies. La clôture porte sur contrat/clés, sans preuve
physique. Matrice123 et contrôle124 publiés dans #29, CI37546917054 et37549552434
entières réussies. Benchmarks125 publiés suraa5f3c9, CI37552260931 entière réussie
avec smoke Linux. Campagne126 Windows5x/count3 passée,36échantillons/12cas analysés ;
Lot126 publié surf4c24d2 avec CI37554852072 entière réussie. Relecture127 favorable à la
sortie M3 en bibliothèque ; #29 fusionnée sur038c6c9, CI finale37557272778 et
main37557390479 entières réussies. CLI128 publiée dans #30 sur921155a,
CI37559870551 entière réussie. Contrat129 config/défauts publié surd9a2fb8,
CI37562294743 entière réussie. Chargeur YAML130 publié sur8bba11e,
CI37564852420 entière réussie. CLI check-config131 publiée sur53d4ae0,
CI37567003805 entière réussie. Clôture132 fusionnée #30 sur118634f,
CI finale37569190697/main37569292737 entières réussies, branche CLI supprimée.
Ouverture readonly de diagnostic133 validée localement ; publication/PR/CI encore
à terminer au moment du commit.
Référence de périmètre :
[plan M0–M5](phase-0-proposal.md#11-roadmap-et-critères-mvp).
État technique et prochaine action : [point de reprise](reprise.md).

## Vue d'ensemble

Six jalons avant le MVP : M0 et M1 terminés, socle M2 fusionné en bibliothèque,
M3 terminé en bibliothèque/CI finale et main vertes, M4 commencé128, M5 à réaliser.
Deux jalons restent. Cette sortie M2
suit les livrables de la roadmap ; les issues
#4/#5 restent ouvertes pour leurs critères applicatifs aux jalons suivants.
Le MVP visé est installable sur Linux, avec ingestion et reprise, reconstruction
prudente des messages/destinataires, recherche Web authentifiée et paquets natifs.

Les lots numérotés sont des unités de reprise après quota. Le numéro 133 compte
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
| M3 — reconstruction | Terminé en bibliothèque, #19–29 fusionnés, CI finale/main vertes | Maintenir ambiguïtés/réserves et contrôles pendant les développements applicatifs | 0 |
| M4 — consultation sûre | CLI/config128–132 fusionnés/CI main verte ; ouverture diagnostic133 validée localement | Publication/CI133, lectures/CLI de diagnostic, config des composants, compte local/sessions, API bornée, recherche/détail/timeline Web, sécurité et accessibilité | 9–19 après133 |
| M5 — installation pilote | CI Go partielle existante ; livraison à réaliser | Exécutable/service, paquets, sauvegarde/restauration, installation Linux, sécurité de release et mesures de charge/pilote | 10–18 |
| **Total après133** | **Deux jalons à clôturer** | **Application installable répondant aux critères du cadrage** | **19–37** |

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
CI main37372128796 entière success vérifiée REST. Recherche111–115 fusionnée dans #25 surf68ae85, CI finale37423566878 et main37423754491 entières success. Rétention116–117 publiés/CI vertes, purge118 publiée/CI verte, intégration119 testée/revue favorable.
Le schéma et les memberships conservent tous les faits, y compris les réserves ;
aucun résultat dérivé n'est sérialisé. Le chantier de stockage est fusionné ; CI main
vérifiée. La prévision2–4 lots après107 a couvert108–110. Recherche111 développée
et testée ; autres critères/index/domaines et rétention explicitement séparés :
6–8 lots au lieu de4–6, incluant validation111 et clôture de ces chantiers.
Continuité entre origines, recherche et rétention restent séparées. Réestimation
10–18 M3, 35–61 total après110 était le bilan précédent ; critères inchangés. Après clôture115, réestimation par reste : rétention3–4, continuité1–3, complétude0–1, intégration/validation finale3–6, soit7–14 M3/32–57 total. Recherche111–115 aura pris5 lots ; mesures Windows synthétiques locales ne remplacent pas le pilote Linux représentatif M5. Clôture115 validée par CI entière/fusion/main. Rétention prévue116–119 : preview readonly, protection du rejeu/provenance, suppression/invalidation transactionnelle, intégration/clôture ; reste dans la borne4 de la prévision3–4.
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

## Bilan après clôture rétention119

Quatre lots116–119 dans la prévision3–4 : aperçu readonly, protection persistée des
rejeux, purge/invalidation transactionnelles et intégration WAL/recherche/import.
PR #27 fusionnée sur5667e2da711150303447d7ca2c1184f5eed3f108 après CI finale
37500212011 entièrement réussie ; CI main37500456255 entièrement réussie.
Rétention en bibliothèque, pas un service ni une tâche cron.

Après clôture119, le reste prévu est M3 4–10 : continuité prouvée1–3, complétude
applicative0–1 et intégration/mesures/revue finale M3 3–6. M4 reste15–25 et M5 10–18,
soit29–53 lots MVP. Réévaluation par comportements restants, pas pourcentage livré
ni garantie de durée. Trois jalons demeurent ; AD/OIDC/Keycloak aprèsMVP exclus.

Lot120 livre le contrat de cohérence d'attestations, six tests Windows réussis,
publié16784eb297dfc0aead2212e63c13bb828899f936 dans #28 avec CI37537740519 verte.
Il ne produit pas une preuve physique et ne fusionne aucune génération. La
prévision de continuité1–3 reste incertaine : le raccordement à un producteur fiable
nécessitera une réévaluation si son périmètre dépasse l'intégration pure. Les
rotations/imports non prouvés doivent conserver leurs réserves dans le MVP.

Lot121 : intégration pure du contexte aux clés, cinq tests Windows et suite de
corrélation réussis, CI37540913337 entière verte sur720a054. Cette intégration ne fusionne pas les générations et n'apporte
pas une preuve physique. Réestimer à la clôture122 du chantier ; le dernier bilan
après119 reste la référence, sans soustraire mécaniquement les deux lots de contrat.

## Bilan122 du chantier contrat/clés et sortie M3

Trois lots120–122 : deux comportements de bibliothèque et leur revue/clôture.
Onze tests synthétiques, suites locales acquises et CI12137541123250 entière
réussie ; runtime inchangé à la relecture122, aucun défaut bloquant. PR #28
fusionnée sur7ec6dd737681f4af878b8cdc4c8b71de0deb4308 après CI37543845063 verte ;
CI push main37543982877 entièrement réussie. Le tableau historique ci-dessous
décrit l'estimation après122 ; la table de tête utilise le bilan128.

Ce chantier ne livre pas la continuité physique automatique. Le cadrage demande
des états prudents et l'absence de faux parcours/succès sur logs incomplets ; les
origines non prouvées restent distinctes dans le MVP. Une future intégration de
preuves fiables exige un producteur, revalidation et règles de fusion propres ;
elle n'est pas comptée comme livrée ou incluse implicitement dans cette fourchette.

| Travail M3 restant après122 | Lots estimés |
| --- | ---: |
| Matrice de critères, complétude et intégration ciblée sur les risques non couverts | 1–2 |
| Mesures de reconstruction/intégration sur corpus synthétique borné | 1–3 |
| Revue et clôture du jalon M3 | 1–2 |
| **M3** | **3–7** |

M4 15–25 et M5 10–18 donnent28–50 lots MVP ; estimation par travail restant,
pas pourcentage ni garantie. Le lot123 commence par la matrice de sortie et
réutilise les validations déjà acquises. Le pilote Linux représentatif reste M5.

## Bilan123 — matrice de sortie

[Matrice des critères et vérifications](m3-exit-checklist.md) établie sans nouveau
comportement ni rerun des suites. Un contrôle ciblé manque : conflit de deux
tentatives à date égale après persistance/reopen, prévu124. Les cas recyclés,
non datés, NOQUEUE, liens, stale/WAL et rétention ont déjà des parcours SQL testés.

Les mesures115 ne concernent que recherche/migration ; reconstruction,
installation et lecture du manifest restent à mesurer. Reste M3 : contrôle124
un lot, mesures1–3, revue/clôture1–2, soit3–6 ; total MVP28–49 avec M4/M5.
Ce bilan ne ferme pas M3 et ne certifie pas la couverture des journaux.

## Bilan124 — conflit persistant à date égale

Contrôle ciblé SQLite réussi dans les deux ordres d'insertion puis après reopen :
deux tentatives et deux Latest, verdict unknown/OrderUncertain et quatre réserves
conservés, aucun succès comptabilisé. Références et révisions identiques ; DSN,
réponses et relais natifs préservés. Aucun correctif runtime nécessaire.
Test ciblé, suite/vet SQLite et diff locaux réussis ; publication/CI124 à terminer
dans #29. Matrice123 déjà publiée avec CI entière37546917054 réussie.

Reste M3 après124 : mesures1–3, revue/clôture1–2, soit2–5 lots ; M4 15–25 et
M5 10–18 donnent27–48 MVP. Prochain lot125 : protocole/benchmarks de reconstruction
pure, installation et lecture ; mesures synthétiques bornées, aucun SLA ni pilote
Linux livré. M3 toujours ouvert. Cette prévision remplace le reste après123.

## Bilan125 — benchmarks et protocole

Lot124 publié sur2f573a2 dans #29, CI37549552434 entière réussie. Trois benchmarks
préparés pour BuildProjection, InstallProjection et CurrentProjection ; quatre
profils synthétiques16/1024/4096faits avec1à64parts du scope. Smoke local1x sur les
12cas et vet réussis. Préparation hors mesure, invariants vérifiés, sortie courte
conservée ; [protocole](projection-measurements.md). CI125 à vérifier après publication.
Un smoke Linux Go1.26 ajouté à la CI valide réellement le parcours sans seuil de temps.

La campagne répétée et l'analyse restent au lot126 : ce lot125 prépare la mesure,
il ne publie pas une capacité ni une comparaison de performance. Reste M3 après125 :
mesures/analyse1–2, revue/clôture1–2, soit2–4lots ; au moins un lot de mesures et
un de clôture restent nécessaires. M4 15–25/M5 10–18 donnent27–47MVP, estimation
incertaine. Le pilote représentatif Linux reste M5 ; AD/OIDC après MVP.

## Bilan126 — campagne répétée et analyse

Benchmarks125 publiés suraa5f3c9 dans #29, CI37552260931 entière réussie et smoke
Linux Go1.26 passé. Campagne126 Windows/amd64 Go1.26.2 sur cette tête, harness
inchangé : 12cas × 3répétitions × 5opérations,36échantillons entiers réussis.
[Sorties/médianes/plages/allocations et limites](projection-measurements.md).
À4096faits, environ26ms Build,134–137ms Install et74–76ms Current ; allocations
Go cumulées42/112/69Mo par opération dans le cas dense, pas un pic mémoire.
Build1024 varie davantage ; aucun classement, seuil CI, capacité/SLA ou optimisation
n'est déduit. Série/cache chaud, profil régulier ; pilote représentatif Linux M5.

Publication/CI126 à terminer. Lot127 : relecture des critères, matrice et PR #29,
puis clôture si résultats/CI/limites acceptables. Reste M3 : revue/clôture1–2lots,
soit26–45MVP avec M4 15–25/M5 10–18, sous réserve de défaut précis découvert.
M3 n'est pas fermé par une campagne de mesure ; M4/M5 restent à réaliser.

## Bilan127 — sortie M3 en bibliothèque

[Relecture locale assistée](reviews/m3-exit.md) du diff #29, critères/matrice et
limites favorable : aucun défaut bloquant identifié. Test d'intégration124 et
benchmarks125 relus ; validations acquises et campagne126 réutilisées, aucun
runtime modifié ni fondation relancée. CI126 entière37554852072 vérifiée exacte.
La relecture n'est pas une approbation humaine indépendante.

Sortie M3 techniquement validée pour la bibliothèque. CI finale127, fusion #29
et CI main encore à terminer au moment de l'enregistrement. Après succès de ces
étapes : M3 terminé, deux jalons M4/M5,25–43lots estimés jusqu'au MVP (15–25 et10–18).
Prochain petit lot128 : CLI `queueatlas version`, puis configuration par petits
lots distincts. Pas de service/Web/auth/paquet annoncé livré par la clôture M3.
Les logs incomplets gardent leurs réserves, la continuité physique non prouvée
reste distincte, et le pilote représentatif demeure M5. AD/OIDC après MVP, MIT.

Clôture127 effectivement validée : finale d26e0bd/37557272778 et merge038c6c9 /
CI main37557390479 entières réussies ; main actualisé, branche du chantier supprimée.

## Bilan128 — début M4, CLI version

Point d'entrée `cmd/queueatlas`, commande version et aide, codes0/1/2 et flux
stdout/stderr définis. Build ordinaire `QueueAtlas dev`, étiquette injectable au
linker ; binaire compilé et exécuté dans les tests synthétiques. Quatre tests,
vet/format/diff ciblés et go run locaux Windows réussis ; étape Windows CLI ajoutée,
CI Linux/tests/builds statiques existants couvrent aussi le point d'entrée.
[Contrat CLI](m4-cli.md). Lot128 publié dans #30, tête921155a,
CI37559870551 entièrement réussie/trois jobs/SHA exact vérifiés.
Aucun service/auth/Web/paquet livré.

Suite129 : contrat de configuration et valeurs par défaut, puis chargement YAML
durci par lot distinct. M4 reste14–24lots estimés, M5 10–18, total24–42 après128.
Ces fourchettes couvrent développement, tests et revues ; incertitude élevée,
aucune échéance garantie. M3 terminé en bibliothèque, deux jalons restants.

## Bilan129 — contrat de configuration initial

`internal/config` : défauts indépendants, validation pure et diagnostics par champ
sans valeur privée. IP loopback littérale/port valide, délais positifs bornés,
syntaxe d'un chemin SQLite fichier ; aucun DNS, ouverture de base, création
d'état ou démarrage de serveur. [Contrat et limites](configuration.md), cinq tests
synthétiques/vet/format/diff locaux Windows réussis. Étape Windows config ajoutée,
suite Linux existante couvre le package. Lot129 publié surd9a2fb8 dans #30,
CI37562294743 entière réussie, trois jobs/SHA exact vérifiés.

Suite130 : YAML strict/borné et résolution des chemins relatifs depuis le fichier
de config ; sources/auth et CLI restent des lots distincts. M4 reste13–23lots,
M5 10–18, total23–41 après129, fourchettes incertaines à périmètre MVP constant.

## Bilan130 — chargeur YAML strict et borné

`config.Load/Decode` : lecture UTF-8 de64Kio maximum, un seul document, champs
inconnus/doublons/types implicites/nulls/ancres/alias refusés. Défauts des champs
absents, durées textuelles, chemins relatifs résolus depuis le fichier de config.
Diagnostics sans valeurs privées, zéro configuration en erreur, aucune base
ouverte/créée. Exemple du dépôt chargé dans les tests ; parseur yaml/v3 v3.0.5 figé,
MIT QueueAtlas conservée. [API et limites](configuration.md).

Sept tests130 et cinq tests129 (suite config), vet/format/diff Windows passés ;
go mod verify réussi. Lot130 publié sur8bba11e dans #30, CI37564852420 entière
réussie/trois jobs/SHA exact, étape Windows config vérifiés. Prochain lot131 : CLI check-config et codes/flux,
sans démarrage de serveur. M4 reste12–22lots/M5 10–18, total22–40 après130,
deux jalons restants, estimation incertaine ; AD/OIDC après MVP.

## Bilan131 — CLI check-config

`queueatlas check-config --config <chemin>` appelle le chargeur130 : code0/message
fixe stdout en succès, code2 pour arguments/contenu refusés, code1 pour lecture
ou sortie stdout impossible. Diagnostics sans chemins/valeurs privés, aide globale
et de commande, aucune DB créée/ouverte ni composant démarré. [Contrat CLI](m4-cli.md).
Trois tests nouveaux et binaire étendu aux codes0/1/2 ; sept tests CLI/vet/format/diff
et go run sur l'exemple passés sous Windows. Chargeur et fondations inchangés,
CI CLI Linux/Windows existante. Lot131 publié sur53d4ae0 dans #30,
CI37567003805 entière réussie, trois jobs/SHA exact revérifiés à la reprise132.

Prochain lot132 : revue/clôture du chantier CLI/config128–131, contrôle CI de tête,
prêt/fusion #30 puis CI main ; pas d'ajout de fonction indépendante dans cette revue.
M4 reste11–21lots/M5 10–18, total21–39 après131, estimation incertaine.

## Bilan132 — revue et clôture CLI/configuration

[Relecture assistée](reviews/m4-cli-config.md) favorable sur53d4ae0 ; aucune
approbation humaine indépendante revendiquée. Code/contrats/tests acquis et CI131
confrontés, aucun blocage identifié. Lot documentaire sans modification de code,
tests, workflow ou dépendances ; pas de rerun local des fondations. Publication/
CI finale/ready/fusion #30 et CI main encore à terminer au moment de l'enregistrement.

Après clôture effective : M4 toujours en cours, M5 à réaliser, total20–38lots
estimés (M4 10–20/M5 10–18). Prochain lot133 : ouverture SQLite en lecture seule
pour diagnostics, sans création/migration de base, avant une commande db stats/doctor.

Clôture132 effective : finale16ee19c/CI37569190697, #30 fusionnée sur118634f,
CI main37569292737 entièrement réussie, trois jobs/SHA exact vérifiés ; branche
CLI supprimée et main propre. Les attentes précédentes sont le snapshot prépublication.

## Bilan133 — ouverture SQLite des diagnostics

[Contrat readonly](sqlite-diagnostics.md) : handle distinct de Store, base existante
et fichiers réguliers/privés Unix, URI mode=ro/query_only et gardes de connexion,
version7/historique lus au même snapshot, sans création/migration/changement de
journal. WAL validé visible, verrous normaux conservés ; créations/usages auxiliaires
SQLite possibles. Aucune attestation complète du schéma/ACL Windows/disque local.

Cinq tests Windows ciblés, vet/format/diff passés ; refus sans création/mutation,
WAL/concurrence/reconnexion/écritures refusées et chemin URI littéral. Test FIFO/droits
Linux ajouté, exécution attendue en CI ; étape Windows dédiée ajoutée. Publication/
PR/CI133 encore à terminer au moment du commit. Prochain lot134 : métadonnées de
diagnostic bornées sans contenu sensible ; commande CLI dans un lot séparé.
M4 reste9–19lots/M5 10–18, total19–37 après133, estimation incertaine.
