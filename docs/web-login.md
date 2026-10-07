# Connexion Web locale — lot 162

`httpapi.NewWebLoginHandler(guard)` sert `/login` et, depuis163, POST `/logout`.
Il partage le
`auth.HTTPHandler` de l'API et des vues protégées : compte local, Argon2id,
admission des tentatives, origine HTTPS et magasin de sessions identiques.
Le montage du serveur reste à réaliser ; le constructeur n'ouvre aucun listener.

## Formulaire et soumission

- GET/HEAD `/login`, sans query (même `?` vide), corps ou encodage de contenu :
  formulaire public. TLS direct, Host/origine/Sec-Fetch-Site et cookies sont
  contrôlés avant rendu. Cette lecture ne vérifie pas les identifiants, ne
  consomme pas le budget de connexion et ne résout ni prolonge une session.
- Formulaire français avec labels, lien d'évitement et autocomplete standard ;
  POST vers `/login`, champs `username` et `password`, aucun JavaScript.
- POST réutilise les validations et le parseur de l'[API locale](local-http-auth.md) :
  Origin obligatoire, TLS/Host/Fetch-Site stricts, cookies bornés, formulaire
  URL-encoded UTF-8 <=4KiB, deux champs uniques seulement. Aucun paramètre `next`
  ni destination issue de la requête ou des journaux.
- Succès : cookie `__Host-queueatlas_session` frais, Secure/HttpOnly/SameSite=Strict,
  Path=/, puis **303 Location: /messages**. L'ancienne session présentée est
  révoquée ; les autres sessions ne sont pas modifiées. L'API garde son succès200.
- Erreurs HTML fixes, aucun nom/mot de passe/cause brute reflété, champs vides.
  Nom inconnu et mauvais mot de passe donnent le même401. Budget commun API/Web
  dépassé :429. Autres refus conservent les statuts de l'authentification.

Même CSS embarquée/hash CSP, no-store/Pragma/nosniff, anti-framing et no-referrer
que les [vues de consultation](web-search.md), y compris sur le303. Aucun en-tête
CORS, aucune copie intermédiaire de réponse contenant le cookie. Le callback
`auth.WebLoginRenderer` est du code applicatif de confiance : ne pas y refléter
ou journaliser le corps/credentials, modifier les cookies ou affaiblir les headers.
Le renderer livré ne reçoit que du texte fixe dans les données du template.

Après émission, une erreur d'écriture, un Write partiel, une panique ou une
annulation détectée révoque la nouvelle session comme dans l'API. Une session
ancienne déjà révoquée n'est pas restaurée. Une écriture réussie ne prouve pas
la réception client ; une rupture non détectée laisse l'expiration existante
comme limite. Les contraintes de lecture lente/deadlines restent à l'intégrateur.

## Vérifications et suite

Trois nouveaux tests auth et deux nouveaux tests HTTPAPI (34HTTPAPI au total).
Windows Go1.26 : `go test ./internal/auth ./internal/httpapi -count=1` passe
(auth1.453s, HTTPAPI2.480s), vet sur ces deux packages, gofmt et diff passent.
Rejets avant lecture/hash, ancien token préservé sur refus, budget API/Web partagé,
destination fixe, révocation sur Write échoué/partiel/panique/annulation couverts.
Client HTTPS réel avec vrai Argon2id et cookiejar : formulaire/HEAD, connexion,
rotation, accès à la recherche protégée et logout via l'API existante.
Erreurs privées et absence de credentials dans le HTML vérifiées.

Aucun navigateur réel ni Linux local revendiqué. Le client Go ne vérifie pas le
parcours clavier, l'affichage adaptatif ou SameSite dans un navigateur. Le
logout API204 est testé ; depuis163, [déconnexion Web](web-logout.md) depuis les
trois vues avec retour fixe au formulaire. Les vues protégées continuent
de refuser401 une session absente/révoquée, sans redirection automatique.
Prochain164 : revue Web159–163 avec navigateur réel. Montage serveur et
filtres/diagnostics restent à réaliser.
Même PR #37 brouillon, issue #7/M4 ouvertes ; MIT, AD/OIDC/Keycloak après MVP.
