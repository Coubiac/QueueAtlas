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

## Suite concrète

Clôturer ce premier chantier de fonctions pures, puis traiter les réserves de
complétude, expiration explicite et résumés prudents. Conserver les ambiguïtés de chronologie, d'ID recyclé et de
chevauchement inter-source. Les liens confirmés exigent des preuves corroborées ;
le texte distant, Message-ID, PID ou Queue ID seul ne peut fusionner des parcours.
