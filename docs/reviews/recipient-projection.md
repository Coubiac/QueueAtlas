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
pass. Publication/CI90 à vérifier. Aucun stockage/source/Web changé.
