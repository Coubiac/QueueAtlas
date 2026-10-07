# Déconnexion Web locale — lot163

Le handler `httpapi.NewWebLoginHandler(guard)` sert désormais `/login` et
`/logout`. Monter ces deux chemins vers le même handler, l'API auth et les vues
protégées séparément, en partageant le même `auth.HTTPHandler` et magasin de
sessions. Aucun listener applicatif n'est créé par ce constructeur.

## Parcours et contrôles

Les vues recherche, détail et timeline ont un bouton « Se déconnecter » dans la
navigation de session. Un formulaire natif POST vers le chemin fixe `/logout`,
sans champ/token/query/JavaScript, envoie un corps vide. La CSS embarquée garde
focus visible et permet le retour à la ligne de l'en-tête ; le hash CSP est
calculé sur les octets rendus comme précédemment.

POST `/logout` réutilise le logout de l'[API locale](local-http-auth.md) : TLS
direct, Host canonique, Origin exact obligatoire, Sec-Fetch-Site same-origin si
présent, aucun en-tête proxy de confiance. Query (même `?` vide), alias encodé,
fragment, cookie ambigu/surdimensionné, encodage de contenu ou corps non vide
sont refusés. Les gardes de protocole/origine/cookies précèdent lecture et
révocation. GET/HEAD/autres méthodes donnent405 avec Allow:POST, sans déconnecter.

Le token présenté est révoqué **avant** la réponse ; les autres sessions restent
valides. Pas de Resolve, hash, tentative de login ou activité de session.
Absent, malformé, inconnu ou déjà révoqué : même succès idempotent, cookie
`__Host-queueatlas_session` supprimé avec Secure/HttpOnly/SameSite=Strict/Path=/,
Max-Age=0, puis **303 Location: /login**. Aucune destination issue des requêtes
ou des journaux. L'API garde son204 sans corps ni redirection.

Annulation Web détectée avant révocation :503 sans changer le cookie. Après
révocation, erreur/écriture partielle/panique de la réponse ne restaure jamais
la session ; une nouvelle soumission reste idempotente. Une réponse non reçue
peut laisser un cookie dans le navigateur, mais le token ne donne plus accès.
Une requête sensible déjà autorisée avant révocation n'est pas rétroactivement
annulée. Le magasin local perd toutes ses sessions au redémarrage.

Erreurs HTML fixes, aucun credential/token/valeur hostile réinjecté et aucun
cookie modifié sur refus. CSP/no-store/Pragma/nosniff/anti-framing/no-referrer
également sur le303 ; pas de CORS. Le formulaire public reste accessible après
logout ; les vues/API sensibles donnent401 avec un token révoqué, sans redirection
automatique des routes de données.

## Vérifications et suite

Deux nouveaux tests auth et un nouveau test HTTPAPI (35HTTPAPI total).
Windows Go1.26 : `go test ./internal/auth ./internal/httpapi -count=1` passe
(auth1.505s, HTTPAPI2.513s), vet sur ces deux packages/gofmt/diff passent.
Rejets avant body/hash/révocation, corps non vide normal/chunked, annulation avant
commit, idempotence/cookie/horloge non consultée, autre session conservée, API204
et révocation persistante après écriture échouée/partielle/panique sont couverts.
Client HTTPS réel, cookiejar et SQLite synthétique : formulaire des trois vues,
soumission de son action, refus cross-origin sans révocation, cookie supprimé,
retour au formulaire, retry idempotent et replay du token révoqué401 sur toutes
les vues et l'API de recherche.

Aucun navigateur réel/Linux local revendiqué ; clavier, affichage adaptatif,
SameSite et XSS/CSP de la source au rendu restent à valider au lot164 de revue
Web159–163. Même PR #37 brouillon ; montage serveur et compléments ensuite.
Issue #7/M4 restent ouvertes. MIT, AD/OIDC/Keycloak après MVP.
