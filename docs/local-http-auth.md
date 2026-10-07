# Transport HTTP de l'authentification locale — lot150

[HTTPHandler](../internal/auth/http.go) raccorde le [login149](local-login.md) et
les [sessions148](local-sessions.md). Il est utilisable comme http.Handler ; aucun
listener, certificat, commande serve, page Web ou route de données n'est démarré.

## Construction et requêtes

NewHTTPHandler(login, origin) exige le moteur partagé construit au démarrage et
une origine HTTPS configurée par l'opérateur. Config invalide : ErrInvalidHTTPAuth
fixe, sans valeur privée. Origine textuelle canonique de longueur <=300octets :
HTTPS minuscule, DNS ASCII/punycode sans point final ou IP canonique sans zone,
port optionnel1..65535 sans zéro initial ; port443 implicite, ne pas l'écrire.
Ni identifiants, chemin/slash final, query ou fragment. Exemple synthétique :
`https://queueatlas.example:8443`. Pas de DNS/IO/horloge/hash au constructeur.

Toutes les opérations exigent POST, TLS direct (`Request.TLS`), Host identique
à l'autorité configurée et **un seul Origin exactement égal** à cette origine.
Origine absente/null/étrangère/dupliquée refusée, sans fallback Referer. Aucun
X-Forwarded-Proto/Host/User ou Forwarded ne prouve TLS/origine/identité ; proxy avec
terminaison TLS puis HTTP clair vers le handler refusé, même en loopback. Aucun
mode HTTP clair de développement ou proxy de confiance livré. Il faudra cadrer
ces modes explicitement avant un éventuel support. SameSite n'est pas utilisé seul
comme protection des mutations de login/logout : contrôle d'origine obligatoire.

Chemins exacts, sans alias RawPath encodé, slash final ni paramètres query (même
query vide `?`). Aucune redirection, GET/HEAD/OPTIONS refusés avec Allow: POST.
Content-Encoding refusé ; en-têtes Cookie cumulés bornés4096octets et au plus un
cookie de session reconnu par net/http. Cookie ambigu dupliqué = refus avant
lecture/hash/révocation ; autres cookies n'authentifient personne.

| Opération | Requête | Résultat |
| --- | --- | --- |
| POST /api/auth/login | Content-Type application/x-www-form-urlencoded, charset absent ou UTF-8 ; exactement un username et un password | 200, corps fixe `authenticated` suivi d'un LF, cookie neuf |
| POST /api/auth/logout | Corps vide ; cookie de session facultatif | 204, aucun corps, révocation serveur et suppression du cookie |

Login lit au plus4096octets via MaxBytesReader, y compris taille inconnue/chunked.
Champs dupliqués (même clés encodées), inconnus, manquants ou percent encoding
invalide refusés. Les champs sont décodés par url.ParseQuery : `+` = espace,
`%2B` = plus littéral ; aucune normalisation/trim ensuite. UTF-8/bornes des valeurs
restent vérifiés par LocalLogin. Le secret maximal1024octets est représentable
dans4096octets de formulaire percent-encodé avec le nom maximal64octets.

Refus de protocole avant LocalLogin ne consomme pas le budget d'essais et ne hache
pas ; champs décodés mais identifiants invalides passent par l'admission149 et
comptent. Ceci ne limite pas le nombre de requêtes réseau ni le débit de parsing.
Pas de secrets dans les URL, corps de réponse, messages d'erreur ou logs du handler.
Une URL déjà reçue peut avoir été journalisée par l'infrastructure : ne jamais y
envoyer de secret. Les buffers raw/password sont clear au mieux ; les copies Go
du formulaire ne garantissent pas un effacement global du mot de passe.

## Cookie et cycle de vie

Nom fixe `__Host-queueatlas_session`, Path=/, aucun Domain, Secure, HttpOnly,
SameSite=Strict. Cookie de session navigateur sans Expires/Max-Age persistant ;
les deadlines serveur148 restent seules autorité (restauration du navigateur
peut garder son cookie). Token envoyé seulement dans Set-Cookie, aucune identité
ou credential dans la réponse. Aucun bearer dans Authorization/query/form accepté.

Une connexion réussie crée un token frais puis révoque le token de l'ancien cookie
présenté, sans toucher les autres sessions. Un login refusé conserve l'ancien
cookie/token ; aucun Set-Cookie sur erreur. Le logout révoque sans Resolve, donc
sans activité/horloge : absent, malformé, inconnu ou déjà révoqué donne le même204.
Suppression avec mêmes attributs et MaxAge=-1 (sérialisé Max-Age=0). Ne pas déduire
d'un cookie présent une identité ni un rôle : garde des routes de données à livrer.

Après émission, annulation détectée avant réponse : nouveau token révoqué,503
sans cookie. Erreur/écriture partielle/panique pendant émission de réponse ou
annulation détectée après écriture : révocation différée du nouveau token sans
réessayer la réponse. La panique reste propagée au serveur. Si ancien token déjà
révoqué, il ne revient pas. Un Write réussi ne prouve pas la réception client ;
une rupture non détectée peut laisser une session jusqu'à expiration148.

Toutes les réponses sont no-store, Pragma:no-cache et nosniff ; aucun en-tête CORS.
Erreurs fixes :400 forme/requête ambiguë,401 identifiants incorrects (même texte
pour nom inconnu/mot de passe incorrect),403 TLS/Host/Origin,404 chemin,
405 méthode,413 corps trop grand,415 média/encodage,429 admission149,
431 cookies trop grands,503 moteur/contexte/magasin/IO interne. Aucune cause brute,
nom, valeur privée ou token dans ces messages ; pas de Retry-After calculé.

Les sources [OWASP CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
et [MDN Set-Cookie](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie)
documentent vérification d'origine et contraintes du préfixe de cookie. Les
choix de refus strict/cookie/session ci-dessus sont le contrat QueueAtlas.

## Vérifications et suite

Sept tests150 et36auth Windows passés, vet/format/diff réussis. HTTPS réel via
httptest avec vrai Argon2id et cookiejar : login/relogin/token frais/révocation/
logout/suppression. Enregistreurs : guards avant body/hash/révocation, origines/
config/zero, formulaires hostiles/duplicates/taille chunked/corps4096 exact,
UTF-8 maximal et secrets littéraux,401 identique/429/503 privé, ancien token gardé
après refus, logout ambigu/idempotent, rollback sur écriture échouée/partielle/
panique et contexte annulé pendant entropie. Pas de navigateur réel ou test de
préfixe/SameSite dans un navigateur revendiqué. Pas de sleep.
Linux38/race/CI entière à vérifier après publication150, pas de Linux local.

L'intégrateur devra définir les deadlines serveur de lecture/écriture, bornes des
headers, certificats et contrôle d'accès réseau ; la taille bornée ne borne pas
la durée d'une lecture HTTP lente. Aucune config/YAML/TLS/listener livrée150.
Prochain151 : garde des routes de données avec session/cookie et contrôles de
mutations ; corpus local d'enrôlement adapté/provenance/licence dans un lot distinct,
puis revue/clôture avant exposition. CLI/SQLite/FileSource/modules/workflow
inchangés ; MIT et AD/OIDC/Keycloak après MVP.
