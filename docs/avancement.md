# Avancement et estimation jusqu'au MVP

**Dernier retour du 8 octobre — fiche de suivi attendue :** le propriétaire
demande les informations mail/routage/SMTP/durée. La correction164 ajoute une
vue opérationnelle HTML des faits déjà collectés, isolée par génération, sans
changement de stockage/API. [Éléments affichés et manquants](message-tracking-view.md).
L'en-tête From:/Subject, états actuels de file et rejets non attribués ne sont
pas inventés. Revue utilisateur du nouveau rendu toujours requise, pas de fusion.

**Retour utilisateur du 8 octobre, lot164 :** connexion et recherche ABC123
confirmées manuellement après correctif b845d95/CI37707149012. Liste des résultats
allégée pendant la revue ; calendrier et recherche expéditeur/destinataire/sujet,
ET/OU, égalité/début/contient/fin demandés et acceptés, pas encore implémentés.
[Travaux et dépendances](search-ux-follow-up.md). L'estimation historique13–28
ci-dessous précède ces extensions et doit être révisée lors de leur découpage.
La revue164 reste incomplète, PR37 brouillon, M4/M5 et issue7 ouverts.

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
Ouverture readonly de diagnostic133 publiée dans #31 sur41fbf0f, CI37571495723
entière réussie. Métadonnées134 publiées sur8e9008b, CI37572091851 entière réussie.
CLI db stats135 publiée surcae194c, CI37574528679 entière réussie. Clôture136
fusionnée #31 sur918ef0c, CI finale37576814504/main37576942301 entières réussies,
branche diagnostics supprimée. Doctor137 publié sur d1feeb9 dans #32,
CI37579618027 entière réussie. Clôture138 fusionnée #32 sur0d2cad4,
CI finale37581947764/main37582076645 entières réussies, branche doctor supprimée.
Contrat source139 publié surcfd2b39 dans #33, CI37585141938 entière réussie.
YAML source140 publié sur455148a, CI37588258338 entière réussie. Conversion141
publiée sur223849c, CI37591355873 entière réussie. Clôture142 fusionnée #33 sur19843d6,
CI finale37594338289/main37594545438 entières réussies, branche sources supprimée.
Contrat auth143 publié dans #34 sure00573c, CI37598069906 entière réussie.
Hash/codec144 publié sur22f1694, CI37601878573 entière réussie. Persistance145
publiée sur19415ba, CI37605851032 entière réussie. CLI admin create146 publiée sur
cfde74d, CI37608644994 entière réussie. Clôture147 fusionnée #34 sur9cd6cec,
CI finale37611573008/main37611762238 entières réussies, branche nettoyée.
Sessions148 publiées surbd862cd dans #35, CI37615586790 entière/trois jobs/SHA exact
et race auth réussis. Login149 publié sur94ef705, CI37619138226 entière/trois jobs/
SHA exact/race auth réussis. HTTP150 publié surc50c678, CI37623116898 entière/trois
jobs/SHA exact/race auth réussis. Garde HTTP151 publiée sur eb5d29c, CI37626687859
entière/trois jobs/SHA exact/race auth réussis, revérifiés REST152. Corpus152 publié
sur e4aaa12, CI37631390279 entière/trois jobs/SHA exact/race auth verts, revérifiés
REST153. Clôture153 fusionnée #35 sur81f9f79 ; CI finale37634089599 et
main37634433660 entières/trois jobs/SHA exact/race auth réussis, branche nettoyée.
Contrat de requêtes154 publié sura2947e4 dans #36, CI37639327832 entière/trois
jobs/SHA exact réussis, revérifiés REST155. Handler155 publié sur4cf3e65,
CI37642563497 entière/trois jobs/SHA exact/HTTPAPI Windows/race Linux réussis,
revérifiés REST156. Identité/détail156 publiés sur2298709, CI37646414652 entière/
trois jobs/SHA exact réussis, race HTTPAPI Linux1.26/journal Windows vérifiés,
revérifiés REST157. Timeline157 publiée sur277d981, CI37650350084 entière/trois jobs/
SHA exact/HTTPAPI Windows/race Linux1.26 réussis, revérifiés REST158. Clôture158
fusionnée #36 sur8c3aa85 ; CI finale37653248114/main37653544179 entières réussies,
branche API nettoyée, revérifiées159. Recherche Web159 publiée dans #37 surb907914,
CI37657564872 entière/trois jobs/SHA exact/HTTPAPI Windows/race Linux1.26 réussis,
revérifiés160. Détail160 publié sur29cf545 dans #37, CI37660252622 entière/trois
jobs/SHA exact/HTTPAPI Windows/race Linux1.26 réussis, revérifiés161. Timeline161
publiée94095f8, CI37664362505 entière/trois jobs/SHA exact/HTTPAPI Windows/race
Linux1.26 réussis, revérifiés162. Connexion162 publiée20f64e0, CI37671925509 entière/
trois jobs/SHA exact/auth et HTTPAPI Windows/race auth et HTTPAPI Linux1.26 réussis,
revérifiés163. Déconnexion163 publiée33596e2, CI37675032968 entière/trois jobs/SHA
exact/auth et HTTPAPI Windows/race Linux1.26 réussis, revérifiés164. Lot164 en cours :
corrections navigateur CSP Windows/focus/texte, test import→rendu et captures.
Corrections164 publiées ef87ceb, CI37676596678 entière/trois jobs/SHA exact verts.
Page HTTPS confirmée visible par l'utilisateur après traitement du certificat ;
accès outil encore refusé par sécurité. Essai manuel ensuite refusé ; correctif
Referrer-Policy strict-origin terminé localement, tests/vet passent, publication/CI
à terminer. Nouvel essai manuel nécessaire ; revue incomplète, #37 brouillon.
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

Les lots numérotés sont des unités de reprise après quota. Le numéro 164 compte
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
| M4 — consultation sûre | CLI/config128–132, diagnostics133–136, doctor137–138, sources139–142, compte143–147, auth148–153 et API154–158 fusionnés/CI main verte ; Web159–164 publiés/CI verte, revue HTTPS incomplète | Terminer revue164, montage serveur et filtres/diagnostics | 3–10 |
| M5 — installation pilote | CI Go partielle existante ; livraison à réaliser | Exécutable/service, paquets, sauvegarde/restauration, installation Linux, sécurité de release et mesures de charge/pilote | 10–18 |
| **Total pendant164** | **Deux jalons à clôturer** | **Application installable répondant aux critères du cadrage** | **13–28** |

Révision160 : la borne2–9 après159 ne comptait pas explicitement connexion Web,
montage serveur et compléments de consultation. Elle était trop optimiste.
Au moins cinq comportements restent : timeline, connexion, montage, compléments
de filtres/diagnostics et revue navigateur. Les compléments peuvent nécessiter
plusieurs lots, avec corrections/intégration : M4 devient5–12, M5 reste10–18.
Le périmètre du cadrage ne change pas ; les lots réalisés ne diminuent pas une
borne qui regroupait encore plusieurs comportements indépendants.

Révision139 : l'ancienne borne4–14 pour M4 après138 était trop optimiste.
Le découpage restant compte YAML/raccordement/clôture sources3lots,
compte/sessions/protections3–5, API2–3, Web et revue2–4, avec marge d'intégration
jusqu'à18lots M4. La fourchette suit ces comportements restants et ne diminue pas
automatiquement à chaque numéro de lot. Le périmètre MVP demeure celui du cadrage.
Après clôture142, contrat/YAML/conversion/revue sources réalisés ; les trois lots
restants de ce chantier après139 sont terminés, d'où7–15lots pour M4,
sans autre changement de périmètre. Clôture142 effective vérifiée à la reprise143.
Révision143 : la borne7–15 sous-estimait l'auth locale à3–5lots. Après le contrat143,
hash/codec, persistance, CLI, sessions, protections HTTP et revue demandent au moins
six lots distincts ; API2–3 et Web/revue2–4 ensuite, marge d'intégration/diagnostic
jusqu'à18. M4 devient10–18, sans élargir le périmètre ; un contrat pur n'est pas
un compte utilisable et les lots ne réduisent pas automatiquement cette fourchette.
Après144, hash/codec réalisé ; cinq lots auth minimum restent, M4 devient9–17,
sans autre changement de périmètre ou de marge.
Après145 et CI Linux réussie, persistance réalisée ; quatre lots auth minimum
restent, M4 devient8–16, sans changement de périmètre ou de marge.
Après146 et CI réussie, initialisation CLI réalisée ; trois lots auth minimum
restent (revue du compte, sessions, protections HTTP), M4 devient7–15.
Après clôture147/CI finale/fusion/main, revue du compte réalisée ; deux lots auth
minimum restent (sessions, protections HTTP incluant liste adaptée au login),
M4 devient6–14. Les critères de login restent ouverts, pas de serveur livré.
Révision148 : le magasin mémoire réalise un comportement précis ; l'ancienne
borne6–14 comprimait le login/protections en un seul lot après les sessions.
Quatre lots auth minimum restent : login/admission/essais, transport HTTP/cookies,
contrôles transversaux/liste d'enrôlement, revue/clôture. API2–3 et Web/revue2–4
portent le minimum à8, marge d'intégration/diagnostic jusqu'à15. M4 devient8–15,
sans ajout de fonctionnalité ni pourcentage livré. Les sessions ne sont pas
encore raccordées au login à148, leurs options ne sont pas dans le YAML.
Après149/CI, login/admission/essais et raccordement sessions réalisés : trois lots
auth minimum restent (transport/cookies, contrôles/liste adaptée, revue/clôture),
puis API2–3 et Web/revue2–4. M4 devient7–14, marge/périmètre inchangés.
Après150/CI, le transport login/logout/cookies est réalisé. La garde des routes de
données et le corpus d'enrôlement auparavant groupés seront deux lots distincts,
puis revue/clôture : trois lots auth minimum restent. Estimation conservée7–14,
sans ajouter de critère MVP ni réduire automatiquement au numéro de lot.
Après151/CI, garde de routes réalisée en bibliothèque ; corpus et revue/clôture
restent avant sortie du chantier auth, puis API2–3/Web/revue2–4. M4 devient6–13,
sans changement de marge ou de périmètre, aucune API/Web/listener livré151.
Après152/CI, corpus d'enrôlement/provenance/licence/guide/tests réalisés ; revue
du chantier auth153 encore requise, puis API2–3/Web/revue2–4. M4 devient5–12,
M5 reste10–18, total15–30, sans modification de marge ou critères MVP.
Après clôture153/CI, bibliothèque auth relue, corpus retenu avec défauts actuels
et limites d'assemblage documentées ; suite API2–3/Web/revue2–4. M4 devient4–11,
M5 reste10–18, total14–29. Le service/pilote/navigateur restent à réaliser.
Après154/CI, le contrat d'entrée pur est réalisé ; les handlers de recherche,
détail/timeline et leur raccordement/revue restent à développer en 2–3lots API
estimés, puis Web/revue2–4 et marge d'intégration/diagnostic. Fourchette conservée
M4 4–11, M5 10–18, total14–29 : préparer les paramètres ne livre pas une route.
Les critères de recherche non couverts par les six index actuels devront être
comptabilisés à ce raccordement ; leur coût n'est pas présenté comme déjà réalisé.
Après155/CI, handler de recherche avec reconstruction complète réalisé en
bibliothèque. Identité révisable/détail et timeline restent distincts, puis revue
du chantier API ; le détail de ces lots et les filtres supplémentaires peuvent
modifier l'estimation. Fourchette M4 4–11 conservée à ce stade (Web/raccordement/
diagnostic inclus), M5 10–18, total14–29 ; ce n'est pas un backlog exhaustif.
Après156/CI, identité révisable et résumé/destinataires du détail sont réalisés.
Suite concrète : timeline157 puis revue du chantier API, au moins2lots ; Web et
revue au moins2lots, filtres complémentaires/raccordement/diagnostic dans la marge.
M4 4–11 conservé, M5 10–18, total14–29 ; les critères non couverts restent visibles
et seront reprécisés avant de déclarer M4 terminé.

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

Validation133 effective : 41fbf0f dans #31 en brouillon,
[CI37571495723](https://github.com/Coubiac/QueueAtlas/actions/runs/37571495723)
entière réussie, trois jobs/SHA exact vérifiés ; Linux FIFO/droits et Windows
ouverture passés. Les attentes précédentes sont le snapshot prépublication133.

## Bilan134 — métadonnées SQLite de diagnostic

Six champs fixes sans contenu sensible, lus au même snapshot après revalidation
de version/historique : versions, journal, taille/pages/pages libres. Résultat zéro
et erreurs fixes en échec/annulation. Aucune lecture/count de journaux ni checkpoint
ou contrôle d'intégrité ; image logique incluant WAL, pas occupation physique du disque.
Quatre tests134 plus cinq tests portables133 et vet/format/diff passés sous Windows.
Snapshot avec écrivain indépendant/changement de version, rollback/freelist/privacy,
historique modifié et annulation/relecture vérifiés. Étape Windows CI élargie.
Publication/CI134 encore à terminer au moment du commit dans la même PR #31.
Prochain lot135 : db stats --config pour ce résultat limité, avec codes et tests du
binaire. M4 reste8–18lots/M5 10–18, total18–36 après134, estimation incertaine.

Validation134 effective : 8e9008b dans #31 en brouillon,
[CI37572091851](https://github.com/Coubiac/QueueAtlas/actions/runs/37572091851)
entière réussie, trois jobs/SHA exact et Windows diagnostics vérifiés. Les attentes
précédentes sont le snapshot prépublication134.

## Bilan135 — CLI db stats

Commande stricte avec config explicite, six métadonnées JSON bornées, fermeture
SQLite avant sortie, config invalide/code2 et erreurs DB fixes/code1. Aides sans IO,
aucune création/migration/checkpoint applicatif ; limites des pages logiques/WAL et
compatibilité conservées, aucune attestation d'intégrité. Onze tests CLI, vet/format/
diff passés Windows ; binaire étendu aux cas db stats/codes0/1/2, refus sans création
et effets readonly vérifiés. Publication/CI135 encore à terminer dans #31 au moment
du commit. Prochain lot136 : relecture/clôture diagnostics133–135, CI finale/fusion/
main ; pas de comportement indépendant. M4 reste7–17lots/M5 10–18, total17–35 après135,
estimation incertaine. Service/auth/Web ne sont pas livrés par cette commande.

Validation135 effective : cae194c publié dans #31 en brouillon,
[CI37574528679](https://github.com/Coubiac/QueueAtlas/actions/runs/37574528679)
entière réussie, trois jobs/SHA exact revérifiés REST à la reprise136 ; Windows CLI
passé. Les attentes précédentes sont le snapshot prépublication135.

## Bilan136 — relecture/clôture diagnostics

[Relecture assistée](reviews/m4-diagnostics.md) favorable surcae194c, aucun défaut
bloquant identifié ; aucune approbation humaine indépendante revendiquée. Contrats,
code, tests synthétiques et CI135 confrontés ; lot documentaire seulement, pas de
rerun local des fondations sans risque nouveau. Publication/CI finale/ready/fusion
#31 et CI main encore à terminer au moment du commit.

Après clôture effective : M4 toujours en cours, M5 à réaliser. Prochain lot137 :
doctor --config en lecture seule, rapport de config/compatibilité DB limité aux
vérifications disponibles, sans intégrité/déploiement attestés. M4 reste6–16lots,
M5 10–18, total16–34 après136, estimation incertaine. MIT conservée, AD/OIDC après MVP.

Clôture136 effective : #31 fusionnée sur918ef0c après revue COMMENT assistée sur
3d78970, CI finale37576814504/main37576942301 entières réussies ; checkout principal
actualisé propre et branche diagnostics supprimée. Les attentes136 précédentes
sont le snapshot avant publication, désormais terminé.

## Bilan137 — doctor --config

Rapport JSON avec deux statuts fixes, configuration valide et schéma compatible,
après chargement et ouverture readonly existants puis fermeture. Aucun compteur,
scan, création/migration ou démarrage de service ; limites du lecteur conservées,
aucune attestation d'intégrité/déploiement. Quatre nouveaux tests CLI et binaire
étendu, quinze tests CLI/vet/format/diff Windows passés. Publication/CI137 encore
à terminer au moment du commit ; nouveau chantier codex/m4-doctor depuis main136.
Prochain lot138 : relecture/clôture doctor137 avec CI finale/fusion/main.
M4 reste5–15lots, M5 10–18, total15–33 après137, estimation incertaine.

Validation137 effective : d1feeb9 publié dans #32 en brouillon,
[CI37579618027](https://github.com/Coubiac/QueueAtlas/actions/runs/37579618027)
entière réussie/trois jobs/SHA exact revérifiés à la reprise138. Les attentes
précédentes sont le snapshot avant publication137.

## Bilan138 — relecture/clôture doctor initial

[Relecture assistée](reviews/m4-doctor.md) favorable, aucun défaut bloquant dans
le diagnostic initial livré ; aucune approbation humaine indépendante revendiquée.
Sources/formats/checkpoints/lacunes restent au raccordement applicatif ultérieur.
Lot documentaire seulement ; pas de rerun local sans risque nouveau, CI137
revérifiée. Publication/CI finale/fusion #32 et CI main encore à terminer au commit.
Après succès : M4 toujours en cours. Prochain lot139 : contrat pur de configuration
d'une source fichier, tests/doc ; YAML et raccordement dans des lots suivants.
M4 reste4–14lots, M5 10–18, total14–32 après138, estimation incertaine.

Clôture138 effective : #32 fusionnée sur0d2cad4 après revue COMMENT assistée5438508226
sur84be3c6 ; CI finale37581947764/main37582076645 entières réussies/trois jobs/SHA
exact. Main actualisé propre, branche doctor supprimée local/GitHub.
Les attentes138 précédentes sont le snapshot prépublication, terminé.

## Bilan139 — configuration pure de source fichier

Valeur indépendante et défauts/validation sans IO, champs requis, identité stable,
chemin littéral, politique beginning/end et bornes FileSource réutilisées.
Quatre nouveaux tests, seize tests config/vet/format/diff Windows passés.
Aucun raccordement YAML/CLI/ingestion, aucun fichier ouvert/créé ou composant lancé.
Publication/PR/CI encore à terminer au commit. Prochain lot140 : chargement YAML
strict/borné, défauts des champs absents et chemins relatifs depuis config, tests/doc.
M4 reste10–18lots selon découpage explicité ci-dessus, M5 10–18, total20–36 après139.

Validation139 effective : cfd2b39 publié dans #33,
[CI37585141938](https://github.com/Coubiac/QueueAtlas/actions/runs/37585141938)
entière réussie/trois jobs/SHA exact. Attentes139 ci-dessus prépublication terminées.

## Bilan140 — chargement YAML source

Section facultative source avec defaults stricts, budgets décimaux canoniques,
chemins relatifs depuis config, bornes globales conservées et erreur/config zéro.
Une source fichier uniquement ; aucune ouverture de journal/DB ou ingestion lancée.
Cinq nouveaux tests, vingt-et-un tests config et quinze tests CLI Windows passés,
vet/format/diff et exemple check-config passés ; binaire compilé avec log absent.
Publication/CI dans #33 à terminer au commit. Prochain lot141 : conversion pure
vers paramètres FileSource, tests/doc ; puis revue/clôture142 du chantier.
M4 reste9–17lots, M5 10–18, total19–35 après140, estimation incertaine.

Validation140 effective : 455148a publié dans #33,
[CI37588258338](https://github.com/Coubiac/QueueAtlas/actions/runs/37588258338)
entière réussie/trois jobs/SHA exact. Attentes140 ci-dessus prépublication terminées.

## Bilan141 — conversion pure FileSource

Paramètres chargés revalidés puis copiés dans une valeur FileSource indépendante,
chemin absolu exigé, identité/start/délais/budgets conservés et reprise stricte.
Trois nouveaux tests, vingt-quatre tests config/vet/format/diff Windows passés ;
constructeur beginning/end accepté sans lecture d'état/normalisation/fichier créé.
Aucune ingestion applicative ; publication/CI141 dans #33 à terminer au commit.
Prochain lot142 : relecture/clôture139–141, CI finale/fusion/main et nettoyage.
M4 reste8–16lots, M5 10–18, total18–34 après141, estimation incertaine.

Validation141 effective : 223849c publié dans #33,
[CI37591355873](https://github.com/Coubiac/QueueAtlas/actions/runs/37591355873)
entière réussie/trois jobs/SHA exact. Attentes141 ci-dessus prépublication terminées.

## Bilan142 — relecture/clôture configuration source

[Relecture assistée](reviews/m4-source-config.md) favorable au chantier139–141,
aucune approbation humaine indépendante revendiquée. Lot documentaire seulement,
CI141 entière revérifiée ; publication/CI finale/fusion/main à terminer au commit.
Aucune ingestion applicative ni diagnostic source physique ; M4 reste en cours.
Prochain lot143 : contrat pur compte administrateur local et paramètres de hachage
bornés, tests/doc ; persistance/CLI/sessions/protections dans les lots suivants.
M4 reste7–15lots après succès de clôture, M5 10–18, total17–33, estimation incertaine.

Clôture142 effective : #33 fusionnée sur19843d6 après revue COMMENT assistée5439690236
sur2e813fc ; CI finale37594338289/main37594545438 entières réussies/trois jobs/SHA
exact, main propre et branche sources supprimée local/GitHub.
Les attentes142 précédentes sont le snapshot prépublication, terminé.

## Bilan143 — contrat local d'authentification

Identité locale littérale/casse conservée et paramètres Argon2id validés/bornés
sans IO/hash ; aucun compte/session/route utilisable. Quatre tests auth et
vet/format/diff Windows passés ; publication/PR/CI à terminer au commit.
Prochain lot144 : hash/vérification/codec strict, tests/vecteurs/doc, puis stockage
et CLI dans des lots distincts. Estimation révisée selon découpage ci-dessus :
M4 10–18, M5 10–18, total20–36, deux jalons et incertitude élevée.

Validation143 effective : e00573c publié dans #34,
[CI37598069906](https://github.com/Coubiac/QueueAtlas/actions/runs/37598069906)
entière réussie/trois jobs/SHA exact ; attentes143 prépublication terminées.

## Bilan144 — hash et vérification Argon2id

Sel aléatoire, codec strict/borné/coûts validés avant hash, comparaison des32octets
à temps constant, secrets UTF-8 bornés/littéraux ; dépendance Go officielle épinglée.
Cinq nouveaux tests/neuf auth, vet/format/diff Windows et vecteur indépendant passés ;
fuzz codec5s/633884exécutions passé. CI Windows étendue à auth, publication/CI144
à terminer au commit. Pas de compte/session/CLI/route utilisable.
Prochain lot145 : persistance atomique/lecture bornée du compte et tests/doc.
M4 9–17, M5 10–18, total19–35 après144, estimation incertaine.

Validation144 effective : 22f1694 publié dans #34,
[CI37601878573](https://github.com/Coubiac/QueueAtlas/actions/runs/37601878573)
entière réussie/trois jobs/SHA exact ; attentes144 prépublication terminées.

## Bilan145 — persistance du compte local

Record JSON versionné/borné, identité/hash revalidés sans dérivation, création
complète par lien dur sans écrasement, lecture stricte et erreurs privées/zéro.
Cinq nouveaux tests communs/quatorze auth et vet/format/diff Windows passés,
concurrence huit créateurs/un gagnant ; deux tests Linux droits/links/FIFO écrits,
à vérifier en CI avec nouveau gate race auth. Publication/CI145 à terminer au commit.
Aucune CLI/login/session ; durabilité physique et ACL Windows non attestées.
Prochain lot146 : CLI admin create/tests du binaire/doc et contrôle des mots de passe.
M4 8–16 après CI Linux, M5 10–18, total18–34, estimation incertaine.

CI initiale14537605619494 échouée : fixtures Linux TempDir0755 refusées par le
contrat privé. Fixtures corrigées0700, tests/vet Windows repassés ; production
inchangée, validation Linux/race encore attendue sur la tête corrective.

Validation145 effective :19415ba publié dans #34,
[CI37605851032](https://github.com/Coubiac/QueueAtlas/actions/runs/37605851032)
entière réussie/trois jobs/SHA exact, tests auth Linux/droits/liens/FIFO et race verts.
Les attentes145 prépublication/correction sont terminées.

## Bilan146 — initialisation CLI du compte

admin create disponible, dir existant/username/password-stdin explicites ; secret
borné UTF-8/record unique, pas de saisie console/argv, valeurs littérales hachées,
liste locale initiale finie de valeurs courantes/compte/service ; création145 sans
remplacement, sorties privées/codes0/1/2. Cinq nouveaux tests CLI et un auth,
20CLI/15auth/vet/format/diff Windows passés, test binaire existant enrichi/rebuild
réutilisé. Publication/CI146 à terminer au commit ; aucune route login/session.
Prochain147 : revue/clôture #34 et limites de la liste avant futur login ; sessions/
HTTP dans chantier suivant. M4 7–15 après CI146, M5 10–18, total17–33 incertain.

Validation146 effective : cfde74d publié dans #34,
[CI37608644994](https://github.com/Coubiac/QueueAtlas/actions/runs/37608644994)
entière réussie/trois jobs/SHA exact ;20CLI/binaire et auth Windows15/Linux17,
race auth/builds statiques réussis. Attentes146 prépublication terminées.

## Bilan147 — revue et clôture du compte local

[Relecture](reviews/m4-local-account.md) favorable à la clôture143–146 après
correction ciblée : secrets espaces seuls ASCII/Unicode refusés à la création,
secret accepté et vérification des comptes existants inchangés. Trois tests ciblés
Windows/auth/CLI/binaire, vet/format/diff passés. Publication/CI finale/revue COMMENT/
fusion/main à terminer au commit147. Liste initiale non déclarée suffisante pour
une release login : évaluation/enrichissement/corpus local et essais bornés requis
dans les protections HTTP, ainsi que sessions/cookies/CSRF/TLS/routes protégées.
Prochain148 : sessions en mémoire bornées/émission/expiration/révocation/tests,
sans transport HTTP. M4 6–14 après clôture147, M5 10–18, total16–32 incertain.

Clôture147 effective : #34 fusionnée sur9cd6cec,
[CI finale37611573008](https://github.com/Coubiac/QueueAtlas/actions/runs/37611573008)
et [main37611762238](https://github.com/Coubiac/QueueAtlas/actions/runs/37611762238)
entières réussies/trois jobs/SHA exact ; branche locale/distante supprimées.

## Bilan148 — sessions en mémoire bornées

Émission de bearer256bits/empreinte serveur, lookup canonique et refus privé,
expiration absolue/inactive exacte, activité sans prolongation absolue, révocation
unitaire/globale, capacité stricte sans éviction, concurrence mutex/scan borné.
Six nouveaux tests/21auth/vet/format/diff Windows passés ; Linux23/race à vérifier
en CI. Publication/nouvelle PR à terminer au commit. Pas de login/HTTP/cookie.
Prochain149 : login borné/admission/essais/comptes inconnus, raccordement hash/
sessions sans HTTP. M4 8–15 après CI148 (révision expliquée), M5 10–18, total18–33.

Validation148 effective : bd862cd dans #35, CI37615586790 entière/trois jobs/SHA
exact/race auth réussis, revérifiés REST149 ; PR cohérente réutilisée en brouillon.

## Bilan149 — login et admission bornés

Compte immuable et magasin partagé ; vrai VerifyPassword pour tout nom bien formé,
y compris inconnu, refus fixe sans session. Budget unique tous noms/origines,
fenêtre exacte, succès sans reset, plafonds essais/hachages sans file d'attente.
Annulation ne libère pas une place avant fin du calcul, session seulement après
authentification ; erreurs privées/horloge/capacité échouent sans token partiel.
Limites fenêtre fixe/annulation pendant Issue/DoS du budget/processus documentées
dans [contrat du login](local-login.md). Huit nouveaux tests/29auth/vet/format/diff
Windows passés ; publication/CI/Linux31/race à terminer au commit149.
Prochain150 : HTTP login/logout/cookies, puis protections transversales/liste adaptée
et revue. M4 7–14 après CI149, M5 10–18, total17–32. Pas de serveur/Web livré149.

Validation149 effective : 94ef705 dans #35, CI37619138226 entière/trois jobs/SHA
exact/race auth réussis, REST revérifié150 ; PR réutilisée en brouillon.

## Bilan150 — transport HTTP login/logout et cookie

Deux routes POST, TLS direct/Host/Origin stricts avant lecture/hash/mutation,
formulaire4096octets exact/champs uniques, cookie Secure/HttpOnly/SameSiteStrict
préfixe __Host-, reconnexion révoque ancien token après succès, logout204
idempotent révoque et supprime cookie. Réponse échouée/partielle/panique ou
annulation détectée invalide nouveau token ; no-store et erreurs fixes privées.
Sept tests/36auth/vet/format/diff Windows passés, cycle HTTPS réel/Argon2id/cookiejar
et cas hostiles ; publication/CI/Linux38/race à terminer au commit150.
[Contrat et limites](local-http-auth.md). Aucun listener/serve/YAML/TLSconfig/
page Web/garde générale de données livré. Prochain151 : garde routes de données,
puis corpus local d'enrôlement adapté/provenance/licence et revue en lots distincts.
M4 7–14 après CI150, M5 10–18, total17–32 ; découpage explicité ci-dessus.

Validation150 effective : c50c678 dans #35, CI37623116898 entière/trois jobs/SHA
exact/race auth réussis, REST revérifié151 ; PR réutilisée en brouillon.

## Bilan151 — garde HTTP des données

Protect(next) sans exemption, cookie/session/identité locale vérifiés à chaque
requête ; mutations Origin exact, TLS/Host et Fetch Metadata partagés avec auth.
Filtre privé d'identité sous mutex avant activité, session dans contexte par copie,
pas de bypass headers/query/contexte. Expiration/révocation et refus privés testés.
Contrat/limites [garde HTTP](local-http-guard.md), dont snapshot d'entrée/révocation
non interruptive, GET/HEAD sans mutations et responsabilité montage/handler.
Sept tests/43auth/vet/format/diff Windows passés ; HTTPS réel et32accès concurrents.
Publication/CI/Linux45/race à terminer au commit151. Pas d'API/Web/serveur livré.
Prochain152 : corpus local d'enrôlement adapté/provenance/licence, revue/clôture153
ensuite. M4 6–13 après CI151, M5 10–18, total16–31 ; périmètre/marge inchangés.

## Bilan du lot 152 — corpus local d'enrôlement

Garde151 publiée sur eb5d29c dans #35, CI37626687859 entière/trois jobs/SHA exact/
race auth réussis, revérifiés REST152. Le [corpus](password-blocklist.md) ajoute
10 898 empreintes embarquées de valeurs longues d'une source publique SecLists
figée sous MIT. Provenance/licence/notice, import reproductible et intégrité
complète documentés ; aucune vérification réseau ou fichier à la création.
Valeurs complètes seulement, exemples/dérivés conservés, octets acceptés littéraux.
Les comptes existants restent utilisables malgré l'évolution de la liste.

Deux nouveaux tests/45auth Windows,20CLI enrichis dont binaire et un test d'import
avec données synthétiques ; vet/format/diff passés. Deux générations identiques.
Publication/CI/Linux47/race encore à terminer au commit152. Corpus historique fini,
sans preuve de force/conformité NIST ; revue153 doit évaluer la sélection avec
les essais renouvelables. Aucun listener/API/Web livré ; MIT/AD/OIDC après MVP.

Prochain153 : revue/clôture de #35 puis API2–3/Web/revue2–4. M4 5–12 après CI152,
M5 10–18, total15–30/deux jalons ; marge et périmètre inchangés.

## Bilan du lot 153 — revue/clôture auth

Corpus152 publié sur e4aaa12 dans #35 ; CI37631390279 entière/trois jobs/SHA exact/
race auth réussis, revérifiés REST153. [Relecture](reviews/m4-local-http.md) assistée
favorable à la clôture148–152 : sessions/login/transport/garde/corpus examinés,
aucun blocage identifié, deux commentaires Go obsolètes corrigés sans changement
de comportement. Dix tests auth ciblés et un du générateur/vet/format/diff Windows
passés ; CI finale153/fusion/main/nettoyage à terminer au commit.

Corpus retenu pour le compte local MVP/défauts actuels, gate147 traité ; liste
historique/budget renouvelable/concurrence/annulation conservés sans promesse
de force ou conformité NIST. ADR006 précise TLS direct livré, proxy non supporté.
Montage complet, limites réseau, flux navigateur et rendu Web restent à tester
avec l'application. Aucun listener/API de messages/Web livré153 ; M4 en cours.

Prochain154 : contrat de requêtes de recherche HTTP bornées (filtres/période/
limite/curseur/erreurs privées), puis adaptateur aux recherches/reconstruction.
Après clôture153/CI : M4 4–11, M5 10–18, total14–29/deux jalons. MIT/AD/OIDC
après MVP ; aucun lot154 commencé.

## Bilan du lot 154 — paramètres et curseurs de recherche HTTP

Clôture153 effective : #35 fusionnée sur81f9f79, finale f49381d/
[CI37634089599](https://github.com/Coubiac/QueueAtlas/actions/runs/37634089599)
et [CI main37634433660](https://github.com/Coubiac/QueueAtlas/actions/runs/37634433660)
entières/trois jobs/SHA exact/race auth réussis, branche sessions nettoyée.

Contrat pur [http-search](http-search.md) : six critères indexés exacts, chaîne
RawQuery bornée/unique/UTF-8, dates UTC précises/fenêtre31jours, défaut24h/50résultats,
limite1..200 et curseur canonique lié au sélecteur/période. Validation native
réutilisée sans base ni schéma, erreurs fixes privées et sortie nulle aux échecs.
Cinq tests HTTP et dix-neuf régressions SQLite de recherche passent Windows ;
vecteurs indépendants, bornes/calendar/pagination/domaines, vet/format/diff réussis.
Publication/PR/CI du lot154 encore à terminer au commit sur codex/m4-search-api.

Pas de handler, route, transport ou Web ; les événements trouvés devront servir à
la reconstruction complète. Critères supplémentaires du cadrage encore à préciser
au raccordement. Prochain155 : handler de lecture authentifié/borné ; même PR API.
M4 4–11 après154/CI, M5 10–18, total14–29/deux jalons. Fourchette conservée pour
les handlers/détail/revue API, Web et marge ; pas de baisse au seul numéro de lot.
MIT conservée, AD/OIDC/Keycloak après MVP, aucun lot155 commencé.

## Bilan du lot 155 — recherche HTTP authentifiée

Lot154 publié `a2947e4`, [PR #36](https://github.com/Coubiac/QueueAtlas/pull/36)
brouillon, [CI37639327832](https://github.com/Coubiac/QueueAtlas/actions/runs/37639327832)
entière/trois jobs/SHA exact réussis, état et tête revérifiés REST155.

Handler GET/HEAD `/api/v1/messages` directement protégé par auth151, validation154,
admission partagée sans attente, contexte annulable/délai et lecture complète des
scopes exacts. Reconstruction par file incluant faits hors page/période, autres
origines et sans date ; clés indépendantes des autres files de page. DTO privé
avec provenance/compteurs/réserves/NOQUEUE warning et ambiguïtés, offsets décimaux,
JSON <=1MiB avant succès, erreurs fixes sans réponse partielle. Contrat :
[http-search](http-search.md). Aucun write SQL, projection installée ou listener.

Sept nouveaux tests/douze HTTPAPI total passent Windows : SQLite réel/pagination/
faits complets, protocole avant stockage, NOQUEUE/conflits, contexte/admission,
disparition/volume sans fuite, HTTPS réel/HEAD/révocation. Vet/format/diff passés.
CI enrichie HTTPAPI Windows et race HTTPAPI Linux1.26 ; publication/CI155 encore
à terminer au commit. Régressions SQLite154 réutilisées, pas de Linux local affirmé.

Prochain156 : identité révisable et détail, timeline/revue ensuite dans #36.
Pas de messages globaux uniques ou détail/timeline/Web/service promis par cette
route de matches ; six critères seulement, compléments du cadrage à préciser.
M4 4–11lots, M5 10–18, total14–29/deux jalons ; pas de décrément automatique,
estimation à revoir avec les comportements API restants. MIT, AD/OIDC après MVP.

## Bilan du lot 156 — identité révisable et détail

Lot155 publié `4cf3e65`, #36 brouillon, [CI37642563497](https://github.com/Coubiac/QueueAtlas/actions/runs/37642563497)
entière/trois jobs/SHA exact/HTTPAPI Windows/race Linux1.26 réussis, revérifiés REST156.
ID canonique borné version1 ajouté aux candidats de recherche, détail GET/HEAD
dans le même handler protégé/budget partagé. Lecture complète de file et révision
recontrôlée avant sélection : ancienne clé après import tardif409, génération
inexistante404, budget dépassé422 entier, autres erreurs privées503.

Résumé/destinataires exacts avec réserves, counts prudents, latest refs/conflits,
nombre de tentatives, ancre/removal ; pas de lignes/messages bruts ou maps.
Contrat [http-detail](http-detail.md). Quatre nouveaux tests/seize HTTPAPI passent
Windows Go1.26, codec avec vecteur indépendant, SQLite réel jusqu'au détail et
stale-refusal, protocole/auth/budgets, HEAD, byte-preservation du DTO/conflits/deadline ;
vet/format/diff passés. Publication/CI156 à terminer au commit, même #36.

Prochain157 : timeline paginée sur la révision, puis revue API. Aucun listener/
YAML/service/Web livré156. M4 reste4–11lots, M5 10–18, total14–29/deux jalons ;
timeline+revue au moins2 et Web+revue au moins2, marge des compléments/raccordements.
MIT, AD/OIDC/Keycloak après MVP ; aucun157 commencé.

## Bilan du lot 157 — timeline paginée

Lot156 publié `2298709`, #36 brouillon, [CI37646414652](https://github.com/Coubiac/QueueAtlas/actions/runs/37646414652)
entière/trois jobs/SHA exact réussis, race HTTPAPI Linux1.26 et journal HTTPAPI
Windows réussis vérifiés ; état revérifié REST157.

Route GET/HEAD events dans le même handler/budget protégé. Chaque page relit la
file complète et vérifie sa révision ; uniquement les faits de la génération,
provenance/qualité/réserves conservées, pas de verdict déduit de l'ordre à date égale.
Curseur public lié à ID/révision/mode brut, taille de page variable. DTO explicitement
borné avec tentatives/DSN/réponse ; lignes brutes désactivées par défaut, permission
serveur plus demande raw=1 nécessaires. Aucun nouveau stockage/source/interface.

Contrat [http-timeline](http-timeline.md). Cinq nouveaux tests/21HTTPAPI passent
Windows Go1.26, SQLite réel/pagination/stale-refusal et octets bruts binaires exacts,
bindings/protocole/auth/politique/HEAD, conflits/presence/échappement JSON, cap entier
et annulation ; vet/format/diff passés. Publication/CI157 à terminer au commit,
preuve effective dans #36 puis prochaine reprise.

Prochain158 : revue/clôture API154–157 si conforme. Timeline réalisée : revue API
au moins1lot puis Web+revue au moins2, marge compléments/filtres/raccordement/diagnostic
conservée. M4 devient3–10 après157/CI, M5 10–18, total13–28/deux jalons. Pas de
listener/serve/YAML/Web livré157, API en bibliothèque. MIT, AD/OIDC après MVP.

## Bilan du lot 158 — revue et clôture API

Lot157 publié `277d981`, #36 brouillon, [CI37650350084](https://github.com/Coubiac/QueueAtlas/actions/runs/37650350084)
entière/trois jobs/SHA exact/HTTPAPI Windows/race Linux1.26 réussis, revérifiés REST158.
[Relecture API](reviews/m4-search-api.md) favorable : aucun défaut bloquant identifié
dans154–157, aucun changement de code/tests nécessaire. Frontières auth/SQL,
révisions/faits complets, pagination/DTO/provenance/réserves, politique brute,
budgets/cap/cache/erreurs relus. Revue assistée, pas une approbation indépendante.

21tests HTTPAPI relancés Windows Go1.26 passent, format/diff passés ; CI complète
157/fondations réutilisées. Limites de mémoire, calculs coopératifs, snapshots,
montage serveur et rendu navigateur documentées. Aucune mesure de charge/browser
ajoutée, aucun nouveau comportement. Publication/CI finale/revue COMMENT/prêt/
fusion/main/nettoyage encore à terminer au commit158 ; preuves dans #36 puis159.

Prochain159 : page Web de recherche protégée, formulaire/résultats échappés et
réserves à six critères, détail/timeline Web ensuite. Revue API réalisée : son lot
de clôture retiré, Web+revue au moins2, marge filtres/état sources/health/ready/liens/
raccordement/diagnostic conservée. M4 2–9 après158/fusion/CI, M5 10–18,
total12–27/deux jalons, à préciser au Web sans annoncer M4/MVP terminés.
MIT conservée, AD/OIDC/Keycloak après MVP ; aucun159 commencé.

## Bilan du lot 159 — première vue Web

Clôture158 effective : #36 fusionnée sur8c3aa85 ; tête finale e8bde71,
CI37653248114/main37653544179 entières/trois jobs/SHA exact/HTTPAPI Windows/race
Linux1.26 réussis, revue COMMENT5445385773, branche API nettoyée. Revérifiés REST159.

[Recherche Web](web-search.md) réalisée sur codex/m4-web : formulaire protégé,
six critères, pagination aux dates figées, événements/réserves/provenance en texte
échappé, budgets partagés avec l'API, CSP CSS embarquée sans JavaScript, réponse
HTML <=1MiB avant succès. Vide et NOQUEUE conservateurs.25tests HTTPAPI Windows
Go1.26/vet/format/diff passent, incluant SQLite et client HTTPS réels. Publication
et CI159 encore à terminer au commit ; preuve après publication dans la PR Web.
Rendu/attaque XSS dans un navigateur réel et parcours clavier pas encore validés.

Prochain160 : détail Web, même PR ; timeline et revue navigateur ensuite. L'ancienne
borne Web+revue regroupait les vues ; au moins deux lots restent, découpage précis
des vues/raccordements dans la marge. Conserver M4 2–9 après159/CI, M5 10–18,
total12–27/deux jalons sans décrément artificiel. L'issue #7 reste ouverte,
mise à jour de progression après publication ; M4/MVP non terminés. MIT conservée,
AD/OIDC/Keycloak après MVP ; aucun160 commencé.

## Bilan160 — détail Web

Recherche159 publiée surb907914 dans #37 brouillon ; CI37657564872 entière/trois
jobs/SHA exact/HTTPAPI Windows/race Linux1.26 réussis, revérifiés REST160. Main
fusionné reste1588c3aa85/CI37653544179 ; #37 est publiée, pas intégrée à main.

[Détail Web](web-detail.md) réalisé : liens canoniques depuis recherche, mêmes
faits complets/révision/génération de l'API, destinataires/références/réserves en
texte échappé, conflits inconnus et adresses exactes/vide/base64 conservés. Erreur
409 si faits modifiés, aucune sélection silencieuse ; HTML entier plafonné et
admission/délai communs API/Web.28tests HTTPAPI Windows Go1.26/vet/format/diff
passent ; SQLite/client HTTPS réels, hostilité/protocole/révocation/conflits et
budget/annulation/HTML>1MiB couverts. Pas de navigateur réel revendiqué.

Publication/CI160 à terminer au commit dans la même #37. Prochain161 : timeline
Web, pas commencé. Estimation corrigée pour compter explicitement les comportements
restants, sans nouveau périmètre : M4 5–12, M5 10–18, total15–30/deux jalons,
voir détail de révision160 au tableau. #7/M4 restent ouverts, MIT conservée,
AD/OIDC/Keycloak après MVP.

## Bilan161 — timeline Web

Détail160 publié29cf545/#37 brouillon, CI37660252622 entière/trois jobs/SHA exact/
HTTPAPI Windows/race Linux1.26 réussis, revérifiés REST161. Main reste1588c3aa85,
aucune nouvelle fusion. Timeline Web161 réalisée localement : lien depuis détail,
pagination liée au candidat/révision/raw, tentatives/métadonnées/provenance/réserves
en texte ; raw interdit par défaut, permission et demande explicites nécessaires.
Contrôles/direction en notation visible, octets binaires en base64 étiqueté ;
HTML entier borné, mêmes budgets API/Web. [Contrat](web-timeline.md).

32tests HTTPAPI Windows Go1.26/vet/format/diff passent, incluant SQLite/HTTPS réels,
stale/budget/protocole/permission avant base, hostile/contrôles/vide/absence/binaire,
concurrence partagée/annulation/volume brut sans troncature. Pas de navigateur réel
revendiqué. Publication/CI161 encore à terminer au commit dans la même #37.
Prochain162 : connexion Web locale ; pas commencé. Timeline réalisée retire un
comportement de la révision160 : M4 4–11, M5 10–18, total14–29/deux jalons après
161/CI. #7/M4/MVP restent ouverts, MIT conservée, AD/OIDC/Keycloak après MVP.

## Bilan162 — connexion Web locale

Timeline161 publiée94095f8/#37 brouillon, CI37664362505 entière/trois jobs/SHA exact/
HTTPAPI Windows/race Linux1.26 réussis, revérifiés REST162. Main reste1588c3aa85.
Formulaire local public, POST sécurisé partageant l'auth API, cookie frais avec
rotation et303 fixe vers la recherche. Erreurs HTML privées et aucun credential
réinjecté ; l'API login/logout conserve ses réponses. [Contrat](web-login.md).

Trois nouveaux tests auth et deux HTTPAPI, 34HTTPAPI total ; tests Windows Go1.26,
vet/gofmt/diff passent. HTTPS réel/vrai Argon2id/cookiejar, refus avant body/hash,
budget partagé, session sur rotation/révocation/IO/cancellation couverts.
Publication/CI162 encore à terminer au commit dans la même #37. Pas de navigateur
réel ou serveur applicatif livré ; le logout API204 testé n'est pas un parcours
Web. Prochain163 : déconnexion Web avec retour au formulaire et navigation depuis
les vues protégées. Aucun163 commencé.

Précision162 : le comportement « connexion » de l'ancienne estimation regroupait
encore connexion et déconnexion. Le formulaire/succès est réalisé ; la déconnexion
Web reste un lot distinct avant montage/compléments/revue. Borne M4 maintenue4–11,
M5 10–18, total14–29 après162/CI, deux jalons. Aucun pourcentage déduit du numéro.
#7/M4/MVP ouverts, MIT conservée, AD/OIDC/Keycloak après MVP.

## Bilan163 — déconnexion Web

Connexion162 publiée20f64e0/#37 brouillon, CI37671925509 entière/trois jobs/SHA exact/
auth et HTTPAPI Windows/race auth et HTTPAPI Linux1.26 réussis, revérifiés REST163.
Main reste1588c3aa85. Déconnexion Web depuis les trois vues, révocation avant
réponse, cookie supprimé et303 fixe vers le formulaire. Même contrôle d'origine
et magasin que l'API204 conservée, autres sessions préservées, refus sans mutation
et idempotence ; IO échouée ne restaure pas la session. [Contrat](web-logout.md).

Deux nouveaux tests auth et un HTTPAPI,35HTTPAPI total ; tests Windows Go1.26,
vet/gofmt/diff passent. HTTPS réel/SQLite/cookiejar, formulaire des trois vues,
origine hostile, retry, replay du token révoqué401, garde avant body/hash et
échecs d'écriture couverts. Publication/CI163 à terminer au commit, même #37.
Pas de navigateur réel ou montage serveur applicatif livré.

Prochain164 : revue Web159–163 avec navigateur réel et corrections nécessaires,
puis clôture #37 si critères satisfaits. Montage et compléments dans les chantiers
suivants. Déconnexion réalisée retire un comportement : M4 3–10, M5 10–18,
total13–28 après163/CI, deux jalons. #7/M4/MVP restent ouverts. MIT,
AD/OIDC/Keycloak après MVP ; aucun164 commencé.

## Bilan164 — corrections du rendu, revue HTTPS incomplète

Lot163 publié33596e2/#37, CI37675032968 entière/trois jobs/SHA exact/auth et
HTTPAPI Windows/race Linux1.26 verts, revérifiés164. Main reste1588c3aa85.
Le navigateur a révélé trois défauts corrigés : hash CSP/CSS Windows CRLF,
focus du lien d'évitement et contrôles de direction visibles dans le détail.
ImportFile synthétique→Postfix→SQLite→handlers protégés→HTML : DOM hostile texte,
clavier raw et captures desktop/mobile contrôlés sur snapshots statiques.
Une régression automatisée ajoutée ; fixtures manuelles opt-in, hors CI normale.
Tests auth/HTTPAPI Windows Go1.26, vet/format/diff passent.

**La revue164 reste incomplète** : certificat HTTPS de test refusé, intervention
humaine requise par Computer Use. Aucun login/logout ou SameSite navigateur
revendiqué par les snapshots HTTP. Corrections à publier et CI à contrôler dans
la même #37 brouillon ; [preuves, limites et reprise](reviews/m4-web.md).
Reprendre164, aucun165 commencé, pas de fusion à ce stade. Deux jalons restent,
M4 3–10/M5 10–18/total13–28 conservés ; clôture de revue non acquise.
Issue7/M4/MVP ouverts, MIT, AD/OIDC/Keycloak après MVP.

Reprise164 : corrections publiées ef87ceb/CI37676596678 entière/trois jobs/SHA
exact verts. Utilisateur confirme la page HTTPS après certificat ; l'outil refuse
encore l'accès par sécurité. Rapport manuel du parcours demandé, en attente.
Cette confirmation ne clôture pas la revue ; même lot164, estimation inchangée.

Suite de la même revue164 : essai humain « Requête de connexion refusée » avec
capture, aucune connexion réussie. Défaut identifié : no-referrer rend Origin
null sur un formulaire POST natif ; en-tête Web strict-origin corrigé, guards
Origin/CSRF inchangés. Régression HTTPS reproduite sous ancien header puis corrigée,
suite auth/HTTPAPI et vet passent ; test NOQUEUE stabilisé avec dates explicites.
Port de fixture optionnel loopback pour la reprise sur50104, démarrage/arrêt testés.
Aucun serveur temporaire actif. Publier/CI du correctif puis nouvel essai manuel,
même #37 brouillon, aucun165 et aucune baisse de l'estimation avant fin de revue.
[Diagnostic et limites](reviews/m4-web.md#refus-de-connexion-et-correction-de-referrer-policy).
