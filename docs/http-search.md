# Contrat des requêtes de recherche HTTP — lot 154

`internal/httpapi.ParseSearchRequest(rawQuery, now)` valide la chaîne
`URL.RawQuery` et produit un `sqlite.SearchQuery`. Ce lot prépare la recherche
authentifiée de M4 ; il ne fournit pas encore de handler, de route ou de serveur.
La sélection porte sur des événements indexés, pas sur une identité de message
ni une preuve de livraison. Le futur handler devra assembler les résultats avec
la reconstruction complète avant de présenter des messages.

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

Le futur résultat de première page doit communiquer les dates effectives. La
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

## Erreurs et raccordement suivant

Tout échec retourne une requête nulle et une erreur fixe : `ErrSearchRequest`,
`ErrSearchCursor` ou `ErrSearchClock`. Aucun détail d'entrée, URL, base, compte ou
parseur sous-jacent n'est repris dans ces erreurs. Ce lot ne choisit pas encore
de codes HTTP ni de corps JSON. Les filtres d'adresse sont des données privées :
le futur handler et son infrastructure devront éviter leur journalisation.

Le lot suivant doit fournir un handler de lecture protégé par `auth.Protect`,
avec méthode/chemin, délai et annulation de requête, conversion de résultats
bornée et erreurs privées. TLS, montage serveur, permissions et interface restent
des comportements applicatifs distincts. La validation pure ne les remplace pas.

## Vérifications réalisées

Cinq tests HTTP Windows couvrent défauts/valeurs exactes/six critères, paramètres
hostiles/dupliqués/bornés, calendrier/précision/horloge, vecteur binaire calculé
indépendamment, pagination liée au sélecteur et bornes de position. Le vecteur de
domaine vérifie l'accord avec la normalisation native. Dix-neuf tests de recherche
SQLite (dont index, migration, rollback et reconstruction) passent après exposition
du validateur. `go vet` des deux packages, `gofmt` et `git diff --check` passent.
La publication et la CI du commit de ce lot restent à vérifier au moment du commit ;
leur preuve effective sera consignée dans la PR du chantier puis à la reprise.

MIT conservée ; AD/OIDC/Keycloak après MVP. Aucun changement de schéma ni dépendance.
