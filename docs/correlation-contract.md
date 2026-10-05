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
Sans champ natif concordant, le résultat reste sent ; un suffixe ultérieur ressemblant
à status ne peut le remplacer. Les faits durables antérieurs
ne sont pas réécrits par ce correctif ; la projection reste conservatrice.

Les fixtures synthétiques couvrent local, SMTP, LMTP, virtual, pipe, remise partielle,
retries, bounce et expiration. Le test compte toutes les tentatives : deux deferred
puis sent restent trois faits. Il ne calcule pas encore le dernier résultat par
destinataire, un statut global, les générations, NOQUEUE, les arcs ou la rétention.

## Suite concrète

Séparer les instances et les générations de file avant de regrouper les résultats
par destinataire. Conserver les ambiguïtés de chronologie, d'ID recyclé et de
chevauchement inter-source. Les liens confirmés exigent des preuves corroborées ;
le texte distant, Message-ID, PID ou Queue ID seul ne peut fusionner des parcours.
