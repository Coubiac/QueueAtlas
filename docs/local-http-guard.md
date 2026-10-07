# Garde HTTP des routes de données — lot151

Selon [ADR-006](adr/ADR-006-authentication.md), [HTTPHandler.Protect](../internal/auth/http_guard.go)
enveloppe un http.Handler avec le moteur/session/origine du [transport150](local-http-auth.md).
La garde est disponible en bibliothèque ; aucune route de recherche, Web ou serveur
exécutable n'est livrée ici. NewHTTPHandler reste construit une fois au démarrage.

## Assemblage et identité

`guard, err := authHTTP.Protect(dataHandler)` : setup nil/zéro ou next nil refusé
avec ErrInvalidHTTPAuth. Même validation d'origine/moteur que le constructeur150,
sans IO, horloge, hash ou aléa. Monter login/logout séparément et envelopper le
routeur entier des données avec guard. **Aucune exemption publique par chemin** :
même health/assets/login sont protégés s'ils sont placés sous cette garde.
L'intégrateur doit vérifier le montage complet ; aucune protection d'une route
qui ne passe pas par Protect ne peut être déduite de la bibliothèque.

Seul le cookie __Host-queueatlas_session permet l'authentification. Les headers
Authorization/Forwarded/X-Forwarded-User, query, formulaire ou un ancien contexte
ne donnent aucune identité. Cookieheaders<=4096octets, un seul cookie session
reconnu, comme150. Le token doit être canonique, actif et lié à l'identité exacte
du compte local configuré. Manquant, malformé, inconnu, expiré, révoqué ou d'une
autre identité : même401/corps fixe `invalid credentials` avec LF, sans Set-Cookie.

Résolution148 avec filtre d'identité privé **sous le même mutex**, avant mise à
jour LastSeenAt. Un token d'une autre identité est refusé sans renouveler son
activité ; pas de contrôle/usage séparé. Resolve public conserve son contrat148.
Purge des expirées peut se produire durant une résolution refusée. Session valide
avance seulement l'inactivité, jamais ExpiresAt ; expiration exacte148 appliquée.
Échec de magasin/horloge ou annulation détectée :503 fixe, aucun handler appelé.

Après validation, SessionFromContext(r.Context()) retourne Session et bool :
métadonnées copiées par valeur, sans token/password/hash/rôle. Clé de contexte
privée typée, pas de collision avec une clé string d'un autre middleware. La
garde résout à nouveau le cookie à chaque requête et remplace un ancien snapshot
dans une copie de la requête ; contexte de l'appelant inchangé. Retour nil/contexte
sans clé : Session zéro,false. Le code Go de confiance peut fabriquer des contextes :
ce helper ne crée pas une preuve portable d'authentification ni des permissions.

## Méthodes, TLS et origine

| Méthode | Condition d'origine |
| --- | --- |
| GET, HEAD | Origin facultatif ; s'il existe, un seul égal à l'origine configurée |
| POST, PUT, PATCH, DELETE | Un seul Origin exactement égal obligatoire |
| OPTIONS, TRACE, CONNECT, extensions | 405, Allow: GET, HEAD, POST, PUT, PATCH, DELETE |

GET/HEAD du handler doivent être **sans mutation applicative**. TLS direct et
Host exact obligatoires pour toutes les méthodes, même loopback ; aucun header
proxy/Referer ne les remplace. Null, étranger ou doublé =403. RawPath encodé/nil URL
refusé400 ; query de recherche permise, n'authentifie personne. Content-Encoding
refusé415. Contrôles de protocole avant cookie/activité/lecture du corps/next.

Sec-Fetch-Site est une défense complémentaire commune aux routes protégées **et
login/logout150**. Absent : compatibilité, règles Origin ci-dessus toujours requises.
Présent : une seule valeur same-origin ; none aussi admis pour GET/HEAD. Same-site,
cross-site, vide, inconnu ou doublé refusés403. Une metadata same-origin ne dispense
jamais une mutation d'Origin. Politique QueueAtlas volontairement stricte sur les
valeurs inconnues ; vérifier compatibilité au pilote. Pas de confiance aux sous-domaines.

Selon [OWASP CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html),
Fetch Metadata et vérification d'origine sont complémentaires. La politique151
est un contrôle d'origine, sans token CSRF séparé/CORS/exceptions externes. Elle
ne couvre ni XSS ni un client non navigateur capable de forger ses headers ; ce
client doit toujours posséder un token valide. MIT, AD/OIDC après MVP.

Headers communs : no-store, Pragma:no-cache, nosniff, Vary: Cookie, Origin,
Sec-Fetch-Site ; aucune redirection/CORS/log ni cookie émis par la garde. Erreurs
fixes400/401/403/405/415/431/503 sans cause privée/valeur/token. next reste du code
de confiance et doit conserver cache/CORS, valider routes/body/query, appliquer
permissions et respecter son contexte. Cette garde n'inspecte/lit pas le corps.

## Concurrence et limites

Annulation vérifiée avant/après résolution. Une annulation tardive peut avancer
l'activité sans appeler next ; une course après le dernier contrôle peut entrer
dans next, qui doit traiter son contexte. Authentification vérifiée à l'entrée :
révocation/expiration après admission ne stoppe pas un handler déjà commencé.
Le mutex des sessions n'est pas conservé autour de next, aucun goroutine/timer
de surveillance ou hachage login créé. Les nouvelles admissions observent la
révocation ; sessions et concurrence HTTP/processus restent celles de148–150.
Deadlines/headers/réseau/certificats/montage global restent responsabilité du futur
serveur ; next doit borner ses entrées et sa durée. Pas de garde RBAC ou mode proxy.

## Vérifications et suite

Sept tests151,43auth/vet/format/diff Windows passés : HTTPS réel avec vrai Argon2id/
cookiejar/login/données/mutation Origin/logout/refus ensuite ; guards avant body/
horloge/résolution/activité ; token manquant/inconnu/expiré/révoqué/autre identité/
headers/query/contexte sans bypass et401 privé ; idle/absolu exacts/activité/CSRF
refusé sans renouvellement ; setup/metadata copiée/contexte original inchangé/
méthodes autorisées/aucune exemption ; horloge/cancellation fail closed ;32requêtes
concurrentes, révocation puis refus, handler déjà admis finit après révocation.
Tests150 enrichis pour la politique Fetch Metadata commune. Pas de sleep,
navigateur réel/SameSite/préfixe non testé. Linux45/race/CI entière à vérifier
après publication151 ; pas d'exécution Linux locale revendiquée.
Prochain152 : corpus local d'enrôlement adapté/provenance/licence, puis revue/clôture153.
CLI/config/SQLite/FileSource/modules/workflow inchangés ; aucun lot152 commencé151.

## Relecture du chantier au lot 153

Garde151 et corpus152 publiés/CI vertes, tête e4aaa12/CI37631390279 revérifiée
REST153. [Relecture assistée](reviews/m4-local-http.md) favorable à la clôture.
Montage complet du routeur, permissions/entrées des handlers et essais navigateur
restent requis dans l'application. Au commit153, CI finale/fusion/main à terminer ;
preuves effectives dans #35 puis reprise154. Aucune API de messages livrée ici.
