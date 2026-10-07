# Recherche HTTP authentifiée — lots 154–155

`internal/httpapi.ParseSearchRequest(rawQuery, now)` valide la chaîne
`URL.RawQuery` et produit un `sqlite.SearchQuery`. Ce lot prépare la recherche
authentifiée de M4. Le lot154 définit les entrées ; le lot155 fournit le handler
de lecture décrit ci-dessous, sans serveur applicatif ni interface Web.
La sélection porte sur des événements indexés, pas sur une identité globale de
message ni une preuve de livraison. Le handler rattache chaque événement à un
candidat reconstruit depuis les faits complets de sa file, ou le laisse non assigné.

## Paramètres

Les noms sont exacts et sensibles à la casse. Tous sont uniques, y compris après
décodage des noms percent-encodés. Tout autre paramètre est refusé.

| Paramètre | Présence | Contrat |
| --- | --- | --- |
| `instance` | Obligatoire | Instance native exacte, non vide, au plus 1024 octets UTF-8 |
| `field` | Obligatoire | `sender`, `recipient`, `queue_id`, `message_id`, `sender_domain` ou `recipient_domain` |
| `value` | Obligatoire | Valeur exacte UTF-8 ; vide permis pour une adresse native ; limites du stockage ci-dessous |
| `from` | Avec `until` | Début inclusif UTC, format strict ci-dessous |
| `until` | Avec `from` | Fin exclusive UTC, supérieure au début ; fenêtre au plus 31 jours |
| `limit` | Facultatif | Entier décimal canonique de 1 à 200 ; défaut 50 ; pas de signe ni zéro initial |
| `cursor` | Facultatif | Position canonique de 66 caractères ; dates explicites obligatoires |

Sans dates, le défaut est `[now.UTC()-24h, now.UTC())`. L'appelant fournit un
instant fiable unique. Les dates explicites n'utilisent pas cette horloge.
Les dates suivent `YYYY-MM-DDTHH:MM:SS[.fraction]Z`, avec 1 à 9 chiffres pour une
fraction présente. Pas d'offset, virgule, casse alternative, seconde intercalaire
ni troncature de précision. Les instants doivent tenir dans les nanosecondes UTC
signées de SQLite ; une horloge par défaut hors de cette plage est refusée.

Les valeurs d'adresse/Message-ID sont limitées à 1024 octets, les Queue-ID à 32
octets et les domaines ASCII DNS conservateurs à 253 octets. Les identifiants ne
peuvent pas être vides. NUL, CR, LF et tabulation sont refusés dans instance/valeur.
Les domaines sont normalisés par la validation SQLite (casse ASCII), sans IDNA ni
point final. Les autres valeurs restent littérales, sans trim, déduction de domaine
ou interprétation de `%`, `_`, quotes et autres caractères comme syntaxe SQL.
`SearchQuery.Validate()` réutilise les règles de `SearchEvents` sans base ni mutation.
La requête retournée conserve la valeur fournie, même pour un domaine.

La chaîne brute est limitée à 8192 octets avant décodage et doit être UTF-8 valide.
Les séquences percent-encodées malformées, `;` brut, séparateurs vides et valeurs
décodées invalides sont refusés. Une URL complète ou son `?` ne sont pas une entrée
valide pour cette fonction. Les filtres statut, direction, IP, SASL, etc. du cadrage
ne sont pas implémentés par ce contrat à six critères ; leur couverture applicative
reste à préciser lors du raccordement de recherche.

Exemple synthétique :

```text
instance=synthetic-postfix&field=sender&value=synthetic%40example.test&from=2026-10-07T00%3A00%3A00Z&until=2026-10-08T00%3A00%3A00Z&limit=50
```

## Pagination

Le résultat de première page communique les dates effectives. La
page suivante transmet ces mêmes dates explicites, instance, critère et valeur,
plus le curseur retourné. La taille de page peut changer. Une recherche avec un
curseur et des dates implicites est refusée pour éviter une fenêtre glissante.

`EncodeSearchCursor` produit du base64url canonique sans padding : 49 octets,
soit version 1 (1 octet), révision SHA-256 du sélecteur (32), instant UTC signé
(8) et identifiant de ligne strictement positif (8), nombres en ordre big-endian.
Le décodeur refuse variantes, bits inutilisés non nuls, espaces, CR/LF, padding,
versions inconnues et lignes non positives. Le parseur vérifie ensuite la révision
instance/critère/valeur/fenêtre et l'instant dans `[from, until)`.

Ce format n'est pas signé : un client peut fabriquer une position dans une
recherche autorisée. Le curseur n'est ni secret, ni cookie de session, ni
autorisation, ni identité de message. Il ne prouve pas qu'une ligne existe.
Chaque page utilise un nouveau snapshot SQLite ; les imports tardifs placés avant
la position demandent de recommencer la recherche. Aucune page n'est à elle seule
un ensemble complet de faits de corrélation.

## Erreurs du parseur

Tout échec retourne une requête nulle et une erreur fixe : `ErrSearchRequest`,
`ErrSearchCursor` ou `ErrSearchClock`. Aucun détail d'entrée, URL, base, compte ou
parseur sous-jacent n'est repris dans ces erreurs. Les codes de la route155 sont
décrits ci-dessous. Les filtres d'adresse sont des données privées : le handler
ne les journalise pas ; l'infrastructure doit également éviter les journaux d'URL.

## Handler de lecture — lot 155

`NewSearchHandler(guard, store, options)` retourne uniquement un handler déjà
enveloppé dans `auth.HTTPHandler.Protect`. L'appelant fournit un Store SQLite ouvert,
partagé, et le garde construit pour son origine HTTPS. Login/logout sont montés
séparément. La route exacte est `GET /api/v1/messages`, avec `HEAD` de mêmes
validation/lecture/statut mais sans corps. Toute requête passe par le garde, même
pour un chemin inexistant : cookie, TLS direct, Host et Origin/metadata selon151.
Un compte local authentifié accède à cette lecture ; aucun rôle supplémentaire
ni restriction par instance n'est introduit.

La route refuse méthodes d'écriture, chemin alternatif, corps annoncé non vide,
longueur inconnue et Transfer-Encoding. Elle ne lit pas de corps et ne le mélange
pas aux paramètres. Un `SearchOptions` valide a un délai de 1 à 30 secondes,
un budget global de 1 à 4096 faits et 1 à 8 requêtes simultanées. Défauts : 5 secondes,
1024 faits et 2 requêtes. Partager un seul handler pour conserver ce budget.
Admission immédiate sans file d'attente ; le slot est gardé jusqu'au retour.

Le contexte SQL hérite de l'annulation client et du délai. Annulation vérifiée
avant/après les lectures, entre reconstructions et avant publication JSON ; une
réussite retournée après expiration est jetée. Les calculs purs restent bornés
par le nombre de faits, sans interruption au milieu d'un calcul. Le délai de
contexte ne remplace pas les délais réseau/écriture du futur serveur.

La page sélectionne au plus200 événements, puis leurs files exactes uniques
(instance/Queue-ID), ou les faits sans file de l'instance pour NOQUEUE. Le scope
agrégé est limité à64parties ; une seule lecture complète `CorrelationFacts`
inclut tous les faits stockés des scopes sélectionnés, toutes origines et dates.
Le budget de faits s'applique à l'ensemble de cette lecture, pas par file. Aucun
truncate, filtre temporel de reconstruction, parcours implicite de files liées,
installation de projection ou écriture SQL. Dépassement : échec entier ; réduire
la taille de page peut réduire les scopes, sans garantir qu'une file très grande
tienne elle-même dans le budget.

Chaque file est reconstruite séparément avec `BuildProjection`, fenêtre de liens
1minute sans binding SMTP. Sa révision inclut toutes ses origines/cycles/faits
sans date et ces options, indépendamment des autres files de la page. Les faits
sans file sont reconstruits à part. Résumé conservateur : compteurs destinataires,
rapports d'expiration et réserves existantes ; `sent` SMTP reste distinct de
`delivered`. Une ambiguïté garde `candidate:null` et `unresolved_reason`. NOQUEUE
garde `candidate:null`, `no_queue:true` et, si reconnu, `prequeue_disposition`
(`warning` ou `rejected`) : un warn_if_reject ne devient pas un rejet réel.

### Réponse

JSON contient `from`, `until`, `limit`, `coverage_unproven:true`, `matches:[]` et
éventuellement `next_cursor`. Les dates effectives permettent la page suivante
figée. Chaque match contient ref physique, date/qualité, kind, NOQUEUE, candidat
ou réserve d'assignation. Le candidat contient revision/instance/queue_id/generation,
compteurs, expiration_reports et reserves. Les offsets de provenance sont des
chaînes décimales pour préserver int64 dans les clients JavaScript.

Le nombre de matches pagine des événements ; le même candidat peut apparaître
plusieurs fois, y compris sur plusieurs pages. Pas de total de messages distincts,
de déduplication interpages ou d'identité globale promise. Les clés de candidat
sont révisables, pas encore des liens vers une route de détail. Les lignes brutes,
messages, maps de champs, adresses recherchées, credential et cookie sont exclus
du DTO. Les données textuelles du DTO sont UTF-8 bornées et encodées par
`encoding/json` avec son échappement standard, jamais injectées en HTML.

JSON encodé en buffer plafonné à1MiB avant tout succès HTTP ; un dépassement
retourne une erreur fixe sans préfixe de données. Le garde conserve no-store,
nosniff et Vary ; aucun CORS, redirection ou cookie n'est ajouté par la recherche.
Les erreurs du garde151 restent inchangées. Erreurs propres à cette route :

| HTTP | Code JSON fixe | Sens |
| --- | --- | --- |
| 400 | `invalid_request` | Paramètres/curseur/corps/chemin encodé invalides |
| 404 | `not_found` | Chemin inconnu après authentification |
| 405 | `method_not_allowed` | Méthode autre que GET/HEAD ; Allow GET, HEAD |
| 422 | `search_too_broad` | Trop de scopes/faits ; aucun résultat partiel |
| 429 | `busy` | Budget partagé de concurrence occupé |
| 503 | `unavailable` | Horloge, base, annulation/délai, résultat incohérent ou trop volumineux |

Recherche et faits sont deux snapshots de lecture distincts. Un import tardif
peut changer la révision ; si une rétention fait disparaître un match entre les
lectures, toute la réponse est refusée. Cette bibliothèque ne crée pas de listener,
ne configure pas le YAML ou le service et ne livre pas encore le Web. Prochain156 :
identifiant révisable et lecture de détail/timeline, dans cette même PR.

## Vérifications réalisées

Lot155 : sept nouveaux tests de handler, douze tests HTTPAPI au total passés
Windows Go1.26. SQLite réel avec faits après période/origine sans date/autre
instance, stabilité de candidat entre tailles de page/scopes différents, pagination,
budget de reconstruction, NOQUEUE warning/conflits non assignés ; protocole/auth
avant stockage, HTTPS réel, HEAD, révocation, deadline/cancel/slot occupé/libéré,
absence de match entre lectures et réponse JSON expansée dépassant1MiB sans fuite.
Le premier fixture a été corrigé pour donner un ID d'origine distinct par source
(identité globale immutable SQLite). Vet et format/diff passent. La CI ajoute
HTTPAPI Windows et race HTTPAPI Linux Go1.26 ; résultat final à vérifier après push.

Lot154 : cinq tests HTTP Windows couvrent défauts/valeurs exactes/six critères, paramètres
hostiles/dupliqués/bornés, calendrier/précision/horloge, vecteur binaire calculé
indépendamment, pagination liée au sélecteur et bornes de position. Le vecteur de
domaine vérifie l'accord avec la normalisation native. Dix-neuf tests de recherche
SQLite (dont index, migration, rollback et reconstruction) passent après exposition
du validateur. `go vet` des deux packages, `gofmt` et `git diff --check` passent.
La publication et la CI du commit de ce lot restent à vérifier au moment du commit ;
leur preuve effective sera consignée dans la PR du chantier puis à la reprise.

MIT conservée ; AD/OIDC/Keycloak après MVP. Aucun changement de schéma ni dépendance.
