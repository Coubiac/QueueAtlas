# Timeline HTTP d'un candidat — lot 157

GET/HEAD `/api/v1/messages/{id}/events`, sur le même handler protégé que
[la recherche](http-search.md) et [le détail](http-detail.md). Session opérateur
locale, contrôle d'origine/transport, absence de cache, admission partagée et
délai identiques. Aucun corps lu ou fusionné avec la query.

## Pages liées au snapshot

Chaque page relit les faits complets de l'instance/file, reconstruit avec les
options du détail, puis vérifie la révision et sélectionne la génération. Les
événements renvoyés appartiennent uniquement à cette génération/source/origine.
Les autres cycles, origines et faits sans date participent à la révision, mais
ne sont pas ajoutés à sa timeline. Aucun parcours implicite vers une autre file.
Les faits après la période de recherche restent consultables.

Query fermée, chaque paramètre au plus une fois, au plus8192octets :

| Paramètre | Contrat |
| --- | --- |
| `limit` | Entier décimal canonique1..200, défaut50 |
| `cursor` | Continuation canonique émise par la page précédente |
| `raw` | `0` ou `1`, défaut0 ; permission serveur nécessaire pour1 |

Paramètres inconnus, vides, doublons, mauvais encodage, séparateurs vides,
limites non canoniques et changement de candidat/mode brut avec un curseur sont
refusés400 avant lecture. Query vide, y compris `?` vide, accepte les défauts.
L'ID est celui de la recherche, validé par le codec du lot156.

Curseur version1,35octets : version1octet, digest32octets, index suivantuint16
big-endian. Base64url sans padding,47caractères, re-encodage identique exigé.
Digest SHA-256 de `queueatlas.timeline.v1`, NUL, ID canonique, NUL, octet du mode
brut0/1. Index1..4095 ; une position hors de la génération courante retourne400.
Le nombre d'événements de la page peut changer entre deux requêtes. Curseur
public, non signé, forgeable : il transporte une position, aucune autorisation.
Pas de curseur suivant lorsque le dernier événement est renvoyé.

Un import tardif modifiant les faits de la file rend l'ID périmé :409
`stale_candidate`, avant d'appliquer une position, même si elle n'existe plus.
Relancer la recherche ; aucune révision de remplacement révélée par cette erreur.
File/génération absente404 n'est pas une preuve d'absence dans les journaux.

## Événements et limites d'interprétation

DTO explicite : `candidate_id`, `coverage_unproven`, `has_non_explicit_time`,
`cross_stream_uncertain`, `ordering`, `limit`, `raw_included`, `events` et éventuel
`next_cursor`. Réserves de génération identiques sur chaque page. Ordre annoncé
`timestamp_then_provenance` : dates conservées, puis provenance/offset physique
pour départager les égalités. Ce départage sert à l'affichage et à la pagination ;
il ne prouve pas l'ordre causal des tentatives simultanées.

Chaque événement expose source/origine/offsets décimaux int64, date UTC,
qualité de date, texte de timestamp, hôte déclaré, service, PID, kind,
`parse_failed` booléen, expéditeur et Message-ID observés. Pas de sérialisation de
maps, Observation, Message, chemin du fichier ou détail d'erreur du parseur.

Une tentative reconnue expose destinataire exact, destinataire original,
statut natif, statut interprété et périmètre du transport, relay, DSN et réponse.
Ces derniers sont des métadonnées privées accessibles à l'opérateur authentifié,
même sans demande de ligne brute. Aucune anonymisation n'est promise.
`sent` SMTP signifie le rapport du transport SMTP, pas une livraison finale.
Les rapports contradictoires à date égale restent deux événements ; le résumé
du détail conserve `unknown` et `order_uncertain`.

Valeurs natives : `{encoding: utf8, value: …}` ou base64 standard paddé pour des
octets non UTF-8 fournis par le lecteur. Champs facultatifs : `present` booléen et
`value` objet ou null ; absence distincte d'une valeur explicitement vide. Casse,
octets et texte conservés, JSON échappé. Champs d'adresse/ID/hôte/service/PID/
statut/DSN au plus1024octets ; timestamp, relay, réponse et ligne brute au plus
64KiB chacun. Le codec de champs SQLite existant utilise JSON et n'est pas changé :
la préservation binaire de champs déjà altérés à l'import n'est pas revendiquée.

## Accès aux lignes brutes

`DefaultSearchOptions().AllowRawLogs` vaut false. Le programme qui instancie le
handler doit fixer cette option à true pour autoriser les lignes brutes aux
opérateurs locaux authentifiés de ce handler. Il s'agit d'une permission de
configuration explicite, pas d'un système de rôles par compte ou de fournisseurs.
Son raccordement YAML/serveur reste à réaliser au jalon applicatif.

Même avec cette permission, seules les requêtes `raw=1` incluent `raw` dans chaque
événement. Sinon la clé est absente. Permission désactivée :403 `raw_forbidden`
avant lecture ; la garde d'authentification s'exécute avant cette politique.
Raw provient des octets du record SQLite, sans reparsing, trim ou suppression
de fin de ligne. UTF-8 littéral ou base64 conserve notamment octets invalides,
NUL et CR/LF. Le JSON ne doit pas être réinterprété comme du HTML par le futur Web.

## Budgets, erreurs et vérifications

`FactLimit` porte sur toute la file, indépendamment de `limit`. Dépassement422
`timeline_too_broad`, aucun résultat partiel. Concurrence occupée429 `busy`.
Erreurs de base, données incohérentes, annulation/délai et JSON dépassant1MiB :503
`unavailable` fixe, aucun préfixe de données. Pour des lignes volumineuses, réduire
`limit` (jusqu'à1) ; le plafond est celui de la réponse encodée, pas de toute la
mémoire de reconstruction. HEAD applique les mêmes contrôles/lectures sans corps.
Les autres erreurs/propriétés du handler de recherche restent identiques.

Cinq nouveaux tests/21HTTPAPI au total passent Windows Go1.26 : vecteur indépendant
hashlib/struct/base64 et paramètres hostiles, bindings/bornes, SQLite réel/pagination/
faits hors période/autres origines, budget entier et import tardif409 avant position,
protocole/auth/politique brute avant base/HEAD, record SQLite binaire exact, permission
seule sans exposition, métadonnées/presence/casse/conflits simultanés/échappement JSON,
cap1MiB expansé atomique puis page1 réussie et annulation avec réponse tardive refusée.
Admission155 actualisée pour le slot commun recherche/détail/timeline. Vet/format/diff
passent ; publication/CI157 à terminer au commit, preuve effective dans #36.

Prochain158 : revue du chantier API154–157 et clôture de #36 si les vérifications
le permettent. Web, filtres supplémentaires et raccordements applicatifs restent
au backlog M4. MIT conservée, AD/OIDC/Keycloak après MVP.
