# Revue de la projection des résultats de remise

## Lot89 : une tentative et sa portée

DeliveryFrom pure : KindDelivery, Queue ID, transport whitelist, champs to/status
présents, aucune erreur parser ni NOQUEUE. NativeStatus conservé, Unknown pour
statut inconnu ; sent SMTP/LMTP/pipe ne devient jamais boîte. Local/virtual ne sont
promus que sur phrase exacte de boîte et premier champ status natif concordant. Valeurs simples
sans références maps/time ; absence/vide des champs distingués, aucune fusion.

Trois tests initiaux ciblés passés par coordinateur/auditeur. Revue trouve reply
normalisé par trimAngle : `<delivered to maildir>` devenait la phrase admise et
pouvait être promu. Deux sous-cas régression échouent avant correction. Parser
préserve désormais les chevrons de reply ; traitement adresses/IDs inchangé.
Guard Message original protège aussi les anciennes observations déjà normalisées,
sans modifier les faits stockés. Régression legacy quatre combinaisons et absence
Message ; test parser cinq cas de ponctuation/hostile. Une première garde HasSuffix
était contournable par un fragment xstatus ultérieur ; auditeur relève le cas,
régression legacy échoue avant seconde correction. HasExactStatusReply utilise les
frontières parser32 et compare le premier champ status exact, pas un suffixe.
Test neuf cas : statut ultérieur/nested/forgé/absent/différent et message trop long.

Quatre TestDelivery*, suite package parser Postfix, vet des deux packages et
diff Windows réussis. Dix fixtures vérifient toutes tentatives, 18cas transports,
dix refus, valeurs présentes/vide et copies sans alias. Deux attentes initiales
de tests corrigées (fixture08 contient trois tentatives ; ponctuation hostile
reconnue selon parser), puis vraie régression de canonicalisation ajoutée.
Delta final relu sans blocage, neuf cas du helper et variantes legacy/local/virtual
exécutés par auditeur sur Windows : pass. Documentation alignée. Publication/CI
exacte à vérifier.

Aucun état global, génération, lien, expiration/NOQUEUE, lecteur DB ni persistance
de projection livré. Données synthétiques, revue assistée.

## Lot90 : index candidat par instance et flux de provenance

PartitionFacts pur et borné4096, refus sans résultat partiel pour provenance invalide,
source/instance incohérente, duplicate/overlap. QueueKey instance configurée/QueueID,
flux source/origine distincts et CrossStreamUncertain si multiples ; jamais une
QueueInstance/génération/parcours. Timed selon hypothèses puis offset, Untimed
séparé, Other conserve NOQUEUE/unknown ; aucun effet Host/MessageID ou hashtexte.

Quatre tests et suite correlation/vet/diff Windows réussis : 25permutations fixture16,
date égale départagée par provenance, 2hôtes mêmeID/MessageIDdupliqué/importoverlap,
conservation tousfaits/NOQUEUE/undated, limites et refus de snapshot sans résultat
partiel. Références par valeur, pas maps/dates empruntées. Deux corrections de tests
avant validation : mapnil de removed et fixture26 comportant deux lignes reconnues.
Revue finale sans blocage, quatre tests exécutés par auditeur sur version finale :
pass. Publié83c1011, CI37290765408 success. Aucun stockage/source/Web changé.

## Lot91 : frontières de générations candidates dans un flux

BuildGenerations appelle PartitionFacts ; aucune fusion entre origins/sources,
ancre First révisable et Removed seulement observé, pas succès/complet. Réception
après removal et date strictement supérieure seules autorisent le nouveaucycle.
Undated, frontière non prouvée ou MessageIDs cleanup divergents sans removal rendent
tout le flux concerné Unresolved avec tousfaits, aucune génération partielle.
Dates restent hypothèses ; crossStream/incertitudes conservées.

Cinq tests Windows pass : IDrecyclé13 deuxcycles +25permutations et copies ;
horsordre16/partial17/retries08/encours18, sourcesidentiquestexte distinctes ;
cinq ambiguïtés sans perte/présomption, NOQUEUE/datesexplicites/refus/soutien des
quatre marqueurs réception. Revue trouve retrait physique masqué par tri des dates :
receipt postRemoved antidaté rejoignait anciencycle. Régression échoue avant fix.
Passe physique triéeoffset puis linéaire maxdate/removal refuse contradictions
dans les deux sens, en conservant horsordre interne. Trois cas régression passent
après fix. Suite correlation/vet/diff Windows finale réussis ; delta relu sans blocage,
régression ciblée trois scénarios exécutée par auditeur sur Windows : pass.
Publié34c3f72, CI37291866747 success. Pas de projection DB ni état global.

## Lot92 : tentatives et dernier résultat observé par destinataire

BuildRecipients reprend les générations candidates sous leurs limites/refus et
incertitudes. Adresse exacte, toutes tentatives avec références/date copiée/DSN/
réponse/orig_to. Latest garde toutes les tentatives à date maximale ; conflit de
statuts donne unknown/OrderUncertain, aucun choix par offset. Adresse vide garde
ses faits mais résultat unknown ; unresolved/Other sont conservés, aucune synthèse
de succès global. Removed n'améliore pas le résultat de remise.

Quatre tests Recipient et suites correlation/parser Postfix, vet/diff Windows
réussis : mixed04, trois retries08 et25permutations/copies, tie07 contradictoire,
cycles13/sources/casse/alias/vide, partial17/NOQUEUE/undated/refus.
En préparant la synthèse, statut `<sent>` observé transformé en sent par trimAngle :
régression échoue avant fix. Parser conserve status+reply, garde du premier token
HasNativeStatus protège les champs historiques et xstatus ultérieur. Test Delivery
trois statuts/legacy et helper parser huit cas réussis. Test tie07 corrigé après
lecture des indices de fixture (cleanup et qmgr retry conservés).
Revue finale code et documentaire sans blocage ; quatre tests Recipient et deux
régressions de statut natif exécutés par auditeur : pass. Publiédf0bc0e,
CI37293123113 success. Pas de DB/Web/global/expiration ajoutés.

## Lot93 : clôture du premier chantier de projections pures

Référence isolée propre df0bc0e, dix fichiers runtime/tests identiques aux versions
relues ; aucun delta ni risque nouveau. Aucun test relancé. CI92 entière réussie,
matrice Linux, race FileSource, builds statiques et chemins Windows compris.
Avis de clôture sans blocage, bilan et contrat cohérents avec les limites livrées.
M3 reste en cours ; reprise/README/estimation actualisés. Publication finale93
9fd0259, CI37293504453 success ; #19 fusionnée sur
8b2d969e12960b4efe52c700c90c4fd9d26d4667, CI push main37294124969 success
vérifiée sur SHA exact. Commentaire de revue5412853694 ; revue assistée.
