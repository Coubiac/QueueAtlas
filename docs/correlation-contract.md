# Reconstruction M3 : contrats et limites livrés

La reconstruction commence par des fonctions pures depuis les observations
immuables. Ces projections ne remplacent pas les faits, leur provenance ni leurs
hypothèses temporelles. L'intégration dans SQLite et l'application vient ensuite.

## Lot89 : résultat d'une seule tentative

`correlation.DeliveryFrom` accepte une observation Postfix reconnue de remise,
avec Queue ID, service de remise connu, destinataire et statut explicitement
présents, sans erreur de parsing ni NOQUEUE. Chaque appel décrit un seul événement ;
le consommateur doit conserver son identifiant/provenance. Il ne crée aucune
génération ni relation entre files, et n'identifie pas un message par son Queue ID.

Le destinataire reste exact, sans normalisation de casse ou fusion d'alias. Le
statut natif, `orig_to`, relay, DSN et réponse sont conservés comme valeurs déjà
parsées ; la présence distingue un champ absent d'un champ explicitement vide.
La réponse n'est jamais réinterprétée comme HTML, verdict distant ou Queue ID.

| Fait | Projection | Portée de la conclusion |
| --- | --- | --- |
| `smtp status=sent` | sent, smtp_peer | Acceptation par le saut SMTP observé ; pas de remise en boîte |
| `lmtp status=sent` | sent, lmtp_peer | Acceptation par le peer LMTP ; observation Dovecot distincte |
| `pipe status=sent` | sent, pipe_command | Succès rapporté par le transport vers une commande ; traitement ultérieur inconnu |
| local/virtual sent avec réponse exactement `delivered to maildir` ou `delivered to mailbox` et premier champ status natif concordant | delivered, local_mailbox/virtual_mailbox | Remise rapportée par cet agent Postfix ; pas de lecture ou de traitement ultérieur garanti |
| Autre local/virtual sent | sent, local_agent/virtual_agent | Le nom du service ne suffit pas à conclure une remise en boîte |
| deferred ou bounced | deferred ou bounced, portée du transport | Résultat de cette tentative, pas verdict de tout le parcours |
| Statut natif inconnu | unknown, portée du transport | Conserver le statut natif sans déduire un résultat depuis DSN/réponse |
| removed, bounce notification, qmgr expired, rejet/NOQUEUE | Pas de Delivery | Autres faits à projeter séparément ; removed ne prouve aucun succès |

La distinction par transport suit les manuels Postfix : [SMTP/LMTP](https://www.postfix.org/lmtp.8.html),
[pipe](https://www.postfix.org/pipe.8.html), [local](https://www.postfix.org/local.8.html)
et [virtual](https://www.postfix.org/virtual.8.html). Local peut utiliser des commandes
ou déléguer la remise ; la reconnaissance limitée des réponses de boîte est une
règle conservatrice QueueAtlas, pas un catalogue de toutes les configurations.
Un champ de log reste une observation, pas une vérification indépendante du serveur.

Le lot89 corrige le parser qui retirait à tort les chevrons entourant une réponse.
Les adresses/IDs gardent leur traitement existant. La projection vérifie aussi le
premier champ status du texte Postfix original `Message`, avec les mêmes frontières
bornées que le parser : une ancienne réponse stockée déjà
normalisée, telle que `<delivered to maildir>`, ne suffit pas à établir la remise.
Sans reply natif concordant, la remise en boîte n'est pas établie ; un suffixe
ultérieur ressemblant à status ne peut le remplacer. Depuis le lot92, la
classification d'un statut connu exige aussi sa concordance avec le premier token
status du texte natif. Sans Message concordant, le résultat est unknown.
Les faits durables antérieurs
ne sont pas réécrits par ce correctif ; la projection reste conservatrice.

Les fixtures synthétiques couvrent local, SMTP, LMTP, virtual, pipe, remise partielle,
retries, bounce et expiration. Le test compte toutes les tentatives : deux deferred
puis sent restent trois faits. Ce comportement seul ne calcule aucun résultat
global, génération, rattachement NOQUEUE, arc ou rétention ; les fonctions suivantes
restent séparées.

## Lot90 : index borné des faits candidats

`PartitionFacts` reçoit un snapshot explicite et une limite positive <=4096 faits.
Provenance source/origine/offsets, SourceID de l'observation et instance configurée
doivent être cohérents ; doublons ou chevauchements physiques sont refusés sans
résultat partiel, même pour les lignes non reconnues. Une origine ne peut recevoir
deux instances de confiance différentes. Cette limite compte les faits ; elle ne
remplace pas les limites de tailles du parser ou de configuration en amont.

L'index rassemble les candidats `(instance configurée, Queue ID)` sans utiliser
hôte déclaré ou Message-ID. **Un index candidat n'est pas une QueueInstance ni un
parcours.** Chaque flux `(SourceID, OriginID)` garde ses propres références, sans
déduplication textuelle. Plusieurs flux indiquent `CrossStreamUncertain` : aucune
preuve de continuité ou de chevauchement entre eux. Un flux unique ne certifie pas
l'absence de lacunes ou de réutilisation d'ID à l'intérieur du fichier.

Les références sont indépendantes des IDs d'insertion SQLite. Les faits datés sont
triés selon leurs instants/hypothèses conservés, puis offset physique pour égalité ;
les non datés restent séparés, triés par offset. Ce classement est déterministe
sous permutation du snapshot, pas une preuve d'ordre réel en cas d'horloge incertaine.
Le classement des sources/origines est lexical, jamais une chronologie de rotation.
NOQUEUE, unknown et autres faits hors file restent dans `Other` ; rien n'est supprimé.
Le résultat contient des références par valeur, aucune map/date empruntée au caller.

## Lot91 : générations candidates dans un flux

`BuildGenerations` réutilise le snapshot borné/validé et garde les flux séparés.
Une génération candidate a un premier fait (ancre révisable), ses références et
l'éventuel fait `removed`. Celui-ci ne prouve ni succès, ni couverture complète.
`ReceiptObserved` décrit le premier fait de réception : cleanup/message-id,
smtpd/client, pickup/uid ou qmgr/from+size+nrcpt sans statut. Ce booléen ne garantit
pas que tous les événements de création sont présents.

Après `removed`, une nouvelle génération candidate exige une réception observée
à une date strictement postérieure, selon les hypothèses conservées. Date égale,
activité sans réception ou seconde removal : frontière non prouvée. Plusieurs
Message-IDs cleanup divergents sans frontière removal rendent aussi l'identité
ambiguë ; Message-ID n'est jamais une clé de fusion. Un fait non daté dans le flux
empêche ce découpage temporel. Dans ces cas, **tout le flux de cette file est
Unresolved**, sans générations partielles confiantes ni faits perdus.

Le tri temporel ne peut masquer une frontière physique contradictoire : un fait
avant removal ne peut être daté après lui ; tout fait après removal doit avoir une
date strictement ultérieure. Sinon le flux entier reste boundary_unproven. Les
dates hors ordre au sein d'un cycle restent admises ; cette vérification ne prouve
pas la justesse de l'horloge.

Les motifs fixes sont undated, boundary_unproven et conflicting_message_ids.
Les autres flux restent indépendants. HasNonExplicitTime indique les dates
configurées/inférées plutôt qu'explicites ; même une date explicite ne certifie pas
la synchronisation de l'horloge. CrossStreamUncertain reste présent : aucune
continuité entre générations de fichiers, rotation et import n'est certifiée ici.
Les références ordonnées sont déterministes pour un snapshot donné, pas une
identité globale persistante : l'arrivée de faits plus anciens peut réviser l'ancre.

## Lot92 : tentatives et dernier résultat observé par destinataire

`BuildRecipients` conserve toutes les tentatives dans chaque génération candidate,
avec provenance, date/hypothèses, résultat de transport, DSN, réponse et orig_to.
Les adresses restent exactes : pas de fusion par casse ni par alias orig_to.
Une adresse explicitement vide reste visible avec AddressUnspecified et résultat
unknown ; les observations ne sont ni supprimées ni utilisées comme succès.

Latest référence toutes les tentatives à la date la plus récente. Si leurs résultats
normalisés se contredisent, ObservedStatus est unknown et OrderUncertain est vrai.
Un offset ne choisit jamais le résultat « vraiment dernier ». Une observation à
une date strictement ultérieure peut remplacer ce résultat ambigu, sans supprimer
l'historique. Des résultats identiques à date égale restent tous visibles.

L'absence de création/removal, les hypothèses de date et les incertitudes entre
flux restent portées par la génération. Les flux Unresolved et Other restent
conservés sans attribution à un destinataire. Une observation KindDelivery non
projectable reste référencée dans UnprojectedDeliveries. Removed ne transforme
jamais un deferred en succès. L'expiration qmgr est encore un fait distinct ; le
résultat actuel est celui des tentatives de remise, pas un verdict final du parcours.
Aucun statut global ou certificat de complétude n'est produit ici.

Le parser préserve maintenant aussi les chevrons du statut : `<sent>` n'est pas
`sent`. La régression a échoué avant correction. HasNativeStatus vérifie le premier
champ natif borné pour ne pas interpréter une ancienne valeur déjà normalisée ou
un fragment xstatus ultérieur comme un résultat connu. Une ancienne valeur de
champ peut rester altérée dans les faits historiques ; elle n'est pas réécrite,
son résultat projeté reste unknown quand le texte natif ne la corrobore pas.

## Lot94 : preuves explicites d'expiration de file

`BuildRecipients` ajoute `Expirations` à chaque génération candidate. Chaque
`QueueExpiration` conserve sa référence physique, sa date/hypothèses copiée et le
statut natif `expired`. Seul un fait qmgr/KindMessage, avec Queue ID, sans NOQUEUE
ni ParseError, et statut présent exactement corroboré par le premier champ natif
borné est reconnu. Les fragments xstatus, réponses distantes et anciens champs
normalisés depuis `<expired>` ne suffisent pas. Un événement de log reste une
observation du serveur, pas une certification indépendante.

Tous les rapports distincts restent visibles, même à date égale ou texte identique.
Les sources/origines restent séparées ; la génération porte les mêmes réserves
de date et de continuité. Une date inconnue conserve le flux dans Unresolved,
avec toutes ses références, sans rattachement certain à une génération.

Ces événements ne sont pas des tentatives de remise : le deferred observé reste
deferred, et un sent/delivered existant n'est pas remplacé. Removed, notification
bounce, délai long ou absence de logs ne créent aucune expiration. Aucune adresse
non observée n'est inventée. La synthèse des états et les réserves de complétude
seront un comportement distinct ; ni statut global ni persistance ajoutés ici.

## Lot95 : comptes observés et réserves de synthèse

`BuildSummaries` reprend le snapshot borné et les refus de BuildRecipients. Pour
chaque génération candidate, il garde toutes ses projections et compte les adresses
observées selon leur dernier résultat : unknown, sent, delivered, deferred, bounced.
Une adresse vide compte en unknown et porte une réserve ; une observation de remise
non projectable reste référencée, sans adresse inventée. Trois retries du même
destinataire comptent pour une adresse. Les rapports d'expiration sont comptés
séparément, sans devenir des tentatives ou des destinataires expired.

Les réserves fixes et ordonnées sont coverage_unproven (toujours),
receipt_not_observed, removal_not_observed, non_explicit_time,
cross_stream_uncertain, no_recipients_observed, address_unspecified,
latest_order_uncertain, unknown_result et unprojected_deliveries, selon les faits.
L'API ne reçoit aucune preuve de couverture ; même réception + retrait + nrcpt
concordant ne permet pas de la certifier. Le nrcpt rapporté ne crée pas d'adresses
manquantes et n'est pas utilisé pour déduire la couverture des alias ou des retries.
Ce ne sont pas des statuts finaux du parcours. Les états sent et delivered gardent
leurs portées distinctes ; aucune conclusion de lecture ou de remise distante.

Les comptes ne sont jamais fusionnés entre origines candidates. Unresolved et
Other sont conservés hors des comptes, avec leurs références ; les réserves ne
remplacent pas les hypothèses/qualités de date et les preuves natives du détail.
Sans réception ou sans tentative, la file reste visible sans succès inventé.
Aucun certificat de complétude, stockage, arc, session NOQUEUE ou Web ajouté.

## Lot97 : faits NOQUEUE séparés

`BuildPrequeue` réutilise le snapshot borné/validé de PartitionFacts. Un fait
smtpd/KindReject, NOQUEUE sans Queue ID ni erreur de parsing, est projeté seulement
si son Message borné commence par le préfixe natif reject: ou reject_warning:.
Le premier est un rejet rapporté (sans transformer un code temporaire en bounce),
le second un avertissement : [warn_if_reject](https://www.postfix.org/postconf.5.html#warn_if_reject)
journalise ce diagnostic au lieu de rejeter la requête au titre de cette règle.
Un avertissement ne prouve pas non plus une acceptation ultérieure.

Chaque tentative garde sa référence, instance configurée, PID comme métadonnée,
date/hypothèses copiée et Message comme chaîne ordinaire, ainsi que les champs
from/to/proto/helo présents. Absence et valeur explicitement vide restent distinctes.
Une extraction de métadonnées refusée par le parser conserve le rapport sans
attribuer d'adresses. Les champs hostiles restent des données, sans HTML ou URL
interprétés ; le rendu Web sécurisé sera vérifié au jalon M4.

Les tentatives sont classées par provenance, pas par chronologie supposée. Les
dates inconnues ne créent pas de session ni de file ; elles gardent leur qualité.
Même PID, adresse, date ou texte n'unifie aucun rapport et ne le rattache à la file
acceptée après un RCPT refusé. Origines et hôtes de confiance restent distincts ;
l'hôte déclaré ne crée aucune identité. Chaque autre fait reste dans l'index
candidat Queued ou Other. Postscreen/autres services restent non projetés ici.
Les sessions et rattachements prouvés seront un comportement séparé.

## Lot98 : sessions NOQUEUE candidates fermées

`BuildPrequeueSessions` conserve la partition NOQUEUE entière et rattache seulement
les rapports situés physiquement entre connect/disconnect natifs dans la même
instance configurée, source, origine et PID. L'ancre de la candidate est la référence
connect ; un PID réutilisé après disconnect ouvre une autre candidate. Plusieurs
origines restent indépendantes, sans continuité présumée.

Les trois clients natifs (connect, rapports, disconnect) doivent porter exactement
le même token host[address]. Aucun lookup DNS, alias ou comparaison par adresse
SMTP. Les stages reconnus sont CONNECT/HELO/EHLO/MAIL/RCPT/DATA/END-OF-MESSAGE ;
un format absent/non reconnu reste non attribué. Les dates doivent être utilisables
sous leurs hypothèses, le disconnect strictement après connect, chaque rapport dans
leur intervalle inclusif. L'ordre physique ne remplace pas une date incompatible.
HasNonExplicitTime garde l'hypothèse de date ; CoverageUnproven est toujours vrai.

Un seul rapport de client/date douteux rend tous les rapports de cette fenêtre
non attribués. Les raisons fixes sont boundary_unproven, client_mismatch, undated
et time_conflict. Sans connect/disconnect, avec fenêtre encore ouverte ou nouveau
connect avant fermeture, les anciens rapports restent boundary_unproven ; la
nouvelle fenêtre exige à nouveau ses preuves. Dans une fenêtre fermée, le premier
motif de refus connu est préservé entre les contrôles ; une frontière manquante ou
interrompue impose boundary_unproven, même après un client douteux. Aucune candidate
confiante partielle de la fenêtre refusée.

Les rapports rejet/warning, leurs données et les index Queued/Other restent tous
conservés ; connect/disconnect restent aussi dans Other comme faits d'origine.
Les références de la candidate les citent comme preuves, sans créer de nouveaux
faits. Les sorties sont ordonnées par provenance pour l'affichage. Aucun lien vers
une file acceptée dans la même fenêtre, statut final, couverture certifiée ou
session globale entre fichiers. Aucun stockage/source/API/Web ajouté.

## Suite concrète

Les chantiers purs #19/#20 sont fusionnés. Clôturer les faits NOQUEUE/sessions
candidates après revue/CI, puis traiter les liens candidats et corroborés sans
identité par PID seul. Conserver les ambiguïtés de chronologie, d'ID recyclé et de
chevauchement inter-source. Les liens confirmés exigent des preuves corroborées ;
le texte distant, Message-ID, PID ou Queue ID seul ne peut fusionner des parcours.
