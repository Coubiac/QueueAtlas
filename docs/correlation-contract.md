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

## Lot100 : indices natifs de changement de file

`BuildQueueHints` conserve l'index borné/validé entier et extrait au plus un indice
par fait, trié par provenance. Trois catégories distinctes : smtp_queue_hint,
local_forward_hint et bounce_notification_hint. Chaque indice cite la file source,
la référence physique, l'ID cible rapporté, la preuve native et le relay présent.
Aucune QueueInstance cible, génération, relation confirmée ou modification d'état.

SMTP exige sent et la réponse native exacte de forme limitée
`250 2.0.0 Ok: queued as ID`. Local exige sent avec `forwarded as ID`. Les gardes
du premier statut/réponse natifs protègent les champs historiques normalisés et
les suffixes trompeurs. Bounce exige la phrase native exacte
`sender non-delivery notification: ID` et le champ présent concordant. L'ID doit
suivre la même grammaire bornée que le parser (IsQueueID) ; reconnaître sa syntaxe
ne prouve ni existence ni identité de file. Les autres formats restent dans les
faits d'origine, sans extraction permissive de texte distant.

L'instance cible SMTP reste absente : même loopback, localhost, hôte déclaré ou
présence d'une file portant cet ID ne prouve pas son espace d'identité. Les rapports
local/bounce indiquent l'instance configurée de leur agent, mais restent candidats.
Les [filtres après mise en file](https://www.postfix.org/FILTER_README.html) peuvent
réinjecter ou changer la destination ; [local](https://www.postfix.org/local.8.html)
réintroduit un message transféré. La corroboration exige les observations des deux
côtés et une configuration explicite pour les pairs SMTP ; ces manuels ne sont pas
une preuve de configuration du serveur observé.

Une date inconnue garde l'indice et l'index Untimed : aucune attribution temporelle
à une génération n'est tentée ici. Plusieurs origines gardent leurs indices
distincts. Message-ID, adresse ou texte identique ne fusionne rien. Aucun statut de
la file initiale ne bénéficie d'un résultat de la file citée ; bounce notification
reste distincte d'une remise du destinataire initial. Aucun stockage/source/Web.

## Lot101 : relations corroborées sous preuves et hypothèses

`BuildQueueLinks` conserve tous les indices et les générations candidates ; chaque
indice produit une relation candidate ou corroborée, sans fusion de générations,
parcours ou états de destinataires. Les ancrages source/cible et les preuves
positives sont des références copiées aux faits immuables, au plus huit par lien.
Une seule référence positive par champ est citée ; toutes les répétitions sont
contrôlées pour contradiction et restent dans les faits de la génération.
La corroboration reste
révisable si des faits historiques arrivent ; ce n'est ni couverture certifiée ni
identité globale immuable. Bounce garde sa catégorie distincte d'un transfert.

Options explicites : fenêtre positive <=24h et au plus64 bindings SMTP. Chaque
binding mappe littéralement `(instance source, relay rapporté)` vers une instance
cible ; chaque valeur non vide <=1024 octets, sans NUL/CR/LF/tabulation, aucune
paire dupliquée. Les options invalides donnent l'erreur fixe ErrLinkOptions sans
résultat partiel. Les limites de snapshot/provenance restent celles de PartitionFacts.
Un mapping est une affirmation de configuration du caller, pas un fait extrait des
logs ni une certification indépendante du relais. La relation conserve sa copie.

La source doit appartenir à une génération datée non ambiguë. La cible doit avoir
une réception observée strictement après celle de la source et dans la fenêtre
avant/après le rapport natif. La source et la cible gardent leurs hypothèses de
date : même une date explicite ne certifie pas l'horloge. Une seule génération
cible doit être plausible dans ces conditions temporelles ; un ID recyclé peut
être corroboré si une seule génération satisfait la fenêtre et les autres preuves.
On ne choisit jamais entre plusieurs générations temporellement admissibles ou
origines en comparant leurs adresses/Message-IDs. Une génération cible non résolue
ou des sources multiples empêchent la corroboration. La même clé source/cible est
refusée, et l'ordre strict des réceptions empêche un cycle corroboré.

Les deux côtés doivent avoir des expéditeurs qmgr présents, non contradictoires.
Pour SMTP, le mapping est obligatoire, expéditeurs égaux, Message-IDs cleanup
présents/non vides concordants et destinataire du rapport également observé sur
la cible. Pour local/forwarded, l'expéditeur doit concorder ; si deux Message-IDs
sont présents, ils ne doivent pas se contredire. Pour bounce, la source doit avoir
un expéditeur non vide, la notification un expéditeur explicitement vide et une
tentative cible vers l'expéditeur source. Ce sont des règles conservatrices de
QueueAtlas : les changements d'adresse/enveloppe non corroborables restent candidats.
Une tentative cible deferred/bounced peut prouver son adresse sans prouver un succès.

Les motifs candidats fixes sont source_unresolved, source_ambiguous,
target_instance_unknown, target_absent, target_ambiguous, evidence_insufficient et
self_reference. Sans preuves suffisantes, aucune ancre cible n'est assignée. Les
indices et faits restent visibles ; un lien ne modifie jamais une tentative ni son
dernier résultat. HasNonExplicitTime garde les hypothèses des deux générations.
Aucun stockage, parcours global, preuve de continuité entre fichiers ou Web ajouté.

## Lot103 : clés candidates liées à une révision d'entrée

`BuildQueueInstances` conserve les générations, flux non résolus et autres faits
de `BuildGenerations`, sous les mêmes limites/refus. Chaque génération candidate
reçoit la clé `(révision, instance configurée, queue_id, ordinal)`. L'ordinal
commence à zéro pour chaque instance/ID, selon l'ordre déterministe des sources,
origines puis cycles dans l'origine ; ce n'est pas une chronologie inter-origines.
Plusieurs origines restent plusieurs candidats avec CrossStreamUncertain ; aucune
clé n'est inventée pour un flux non résolu ou NOQUEUE.

La révision SHA256 est calculée sur tous les faits du snapshot, y compris Other,
les provenances, l'instance configurée, tous les attributs de l'observation et
les hypothèses de date. Domaine `queue-instances-v1`, chaînes cadrées par longueur
en octets, nombres/booléens canoniques, références physiques et clés de maps triées.
Les octets UTF8 invalides sont distincts ; les instants sont normalisés en UTC,
sans perdre les nanosecondes ou limiter les années à UnixNano. Les IDs SQLite et
l'ordre d'arrivée n'interviennent pas. Une limite de nombre de faits ne constitue
pas une limite indépendante de taille des métadonnées fournies par le caller.

Réordonner les mêmes faits conserve la révision et les clés. Un import tardif,
une modification d'attribut ou d'hypothèse change toute la révision ; un ordinal
ancien ne peut pas désigner silencieusement une autre génération. Même une ancre
inchangée obtient une clé d'une autre révision. Le caller doit garder la clé entière
avec la projection et remplacer atomiquement une révision lors du futur stockage.
La version du domaine doit évoluer si le framing ou les règles d'identité/génération évoluent.
Cette révision concerne uniquement les entrées/règles des instances candidates,
pas les options SMTP ou une future projection complète des parcours.

L'empreinte versionne une entrée ; elle ne certifie pas l'authenticité, la couverture,
la continuité entre sources ni la déduplication de leurs contenus. Les preuves
restent les FactRef. Le snapshot vide possède une révision sans instance ; un
snapshot invalide est refusé avant toute révision/sortie partielle. Aucun état de
remise, lien, parcours global ou schéma SQLite changé par ce lot.

## Lot104 : composition sous une révision complète

`BuildProjection` compose les résumés par génération, leurs clés candidates,
les liens et les sessions/rapports NOQUEUE, sous le même snapshot immuable.
Le caller garde les observations immuables pendant l'appel. Les bornes/options et
refus restent ceux des primitives : aucune sortie partielle sur erreur.
Un désaccord interne d'ancres retourne l'erreur fixe ErrProjectionInvariant.

La révision complète utilise le domaine `correlation-projection-v1`, la révision
d'entrée103 et les options de liens validées : durée exacte et tous les mappings
SMTP littéraux. La copie des mappings est triée par source/relay ; leur ordre dans
la configuration ne change pas la version. Une option modifiée, même un mapping
inutilisé, produit une nouvelle révision. Le domaine doit évoluer si les règles
de composition ou les comportements des projections changent.

`InputRevision` identifie les entrées des instances candidates sans les options ;
`Revision` identifie la composition complète. Toutes les clés des queues et des
endpoints utilisent cette dernière. Les ancres From/To natives et les preuves
FactRef restent dans Observed. Une relation candidate garde une cible non assignée ;
sa source peut être assignée seulement si la primitive101 avait déjà une ancre.
Les clés/pointeurs et options sont copiés ; une sortie ne modifie pas les entrées.

Résumés, réserves, flux non résolus, Other et rapports NOQUEUE sont conservés.
Chaque référence du snapshot appartient à exactement une génération, un flux
non résolu ou Other ; les preuves supplémentaires peuvent la citer. Aucun lien
ni session ne change un résultat de destinataire ou fusionne des générations.
La composition ne certifie toujours ni couverture, continuité ni authenticité ;
aucun parcours/statut global, persistance, API ou schéma SQLite ajouté.

## Suite concrète

Le lot 120 ajoute un [contrôle d'attestations explicites de continuité](continuity-contract.md).
Il vérifie révision, références et graphe des frontières ; il ne produit pas une
preuve physique et ne change pas les générations/projections existantes. La
production fiable des attestations reste à raccorder. Le lot121 lie les clés
candidates à ce contexte via `BuildQueueInstancesWithContinuity`, sans fusion ni
suppression de réserves. Garder le plan avec ses clés ; les clés ordinaires et
contextuelles restent distinctes. `BuildProjection` et SQLite restent inchangés.

Les chantiers purs, stockage, recherche et rétention #19–27 sont fusionnés.
Le contrat et les clés120–121 sont fusionnés dans #28, CI finale/main réussies.
La [matrice de sortie M3](m3-exit-checklist.md) inventorie les vérifications.
Le contrôle124 publié/CI verte valide le conflit de résultats à date égale après
persistance et reopen, avec insertion inverse et réserves conservées.
Les [mesures locales125–126](projection-measurements.md) couvrent un profil
synthétique borné de reconstruction/installation/lecture. La [relecture127](reviews/m3-exit.md)
valide la sortie M3 en bibliothèque ; #29 fusionnée, CI finale/main réussies ;
aucune capacité ou couverture complète certifiée.
Conserver les ambiguïtés de chronologie, d'ID recyclé et de
chevauchement inter-source. Les liens confirmés exigent des preuves corroborées ;
le texte distant, Message-ID, PID ou Queue ID seul ne peut fusionner des parcours.
