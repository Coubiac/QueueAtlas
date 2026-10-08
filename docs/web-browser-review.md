# Fixture synthétique pour la revue Web

Ces fixtures de test opt-in servent exclusivement sur loopback et créent un
fichier Postfix/une base SQLite temporaires synthétiques. Elles ne démarrent pas
le futur serveur applicatif. Aucun journal ou compte réel n'est nécessaire.

Depuis la racine du dépôt, dans PowerShell :

```powershell
$env:QUEUEATLAS_BROWSER_REVIEW = '1'
go test ./internal/httpapi -run '^TestWebBrowserReview$' -count=1 -v -timeout=35m
```

La sortie fournit `BROWSER_REVIEW_URL` et le chemin `BROWSER_REVIEW_STOP`.
Ouvrir **l'URL réellement affichée**, dont le port change à chaque lancement.
Compte de test préexistant : `SyntheticOperator`, mot de passe
`synthetic secret phrase`. Aucun enregistrement dans un gestionnaire de mots
de passe n'est nécessaire. Le certificat de `httptest.StartTLS` est un certificat
de test ; si le navigateur le refuse, l'opérateur doit traiter lui-même son
avertissement, ou fournir un environnement HTTPS de test accepté. L'agent ne
franchit pas cet avertissement et ne désactive pas la validation TLS.

Parcours restant : GET login, mauvais secret/erreur privée, connexion réussie,
rotation, accès recherche→détail→timeline, logout depuis les vues, retour login,
accès refusé après révocation, cookies et SameSite. Les refus des vues protégées
restent401 sans redirection automatique. Utiliser exclusivement ce compte et les
données synthétiques. Les tests Go HTTPS/cookiejar couvrent déjà les contrôles
serveur ; consigner séparément ce que le navigateur démontre effectivement.

Si l'outil navigateur refuse toujours l'accès après l'intervention humaine,
l'agent ne doit pas chercher un autre accès pour contourner ce refus. L'opérateur
peut effectuer le parcours dans sa page ouverte et transmettre ses observations :

1. Connexion avec le compte synthétique ci-dessus : recherche affichée.
2. Instance `synthetic-postfix`, champ **Expéditeur exact**, valeur
   `synthetic@example.test`, dates `2026-10-07T00:00:00Z` à
   `2026-10-08T00:00:00Z` : candidat ABC123 retrouvé.
3. Lien détail puis timeline : trois faits observés.
4. Déconnexion : retour au formulaire ; rouvrir `/messages` refuse l'accès.

Rapporter chaque succès/erreur et le navigateur utilisé. Une confirmation de la
page login seule ne démontre pas l'authentification. Ces quatre étapes ne prouvent
pas à elles seules la rotation, les attributs des cookies ou tous les cas SameSite.
Ne pas transmettre de token/cookie de session dans le rapport.

## Aperçu de rendu quand la confiance HTTPS est indisponible

```powershell
$env:QUEUEATLAS_BROWSER_REVIEW = 'render'
go test ./internal/httpapi -run '^TestWebRenderedBrowserReview$' -count=1 -v -timeout=35m
```

Ce serveur HTTP ne sert que des snapshots HTML générés par les vrais handlers
protégés à partir de l'import synthétique. GET/HEAD seulement, POST405, routes
inconnues404 ; pas de login/session/proxy ni modification des gardes de production.
Recherche, détail, timeline et variantes prédéfinies raw/limit1 sont disponibles.
Changer le formulaire ne démontre pas une nouvelle requête SQL : les snapshots
restent prédéfinis. Cet aperçu permet de vérifier DOM/XSS/CSP, clavier et format
adaptatif ; **il ne valide pas le parcours authentifié HTTPS ou SameSite**.

## Arrêt et preuves

Dans un second terminal, créer un fichier vide au **chemin exact**
`BROWSER_REVIEW_STOP` affiché :

```powershell
New-Item -ItemType File -Path '<chemin affiché>'
```

Le test se termine normalement et ferme le serveur/SQLite ; les temporaires sont
nettoyés. Sans arrêt explicite, il échoue après30minutes et ferme le serveur.
Il est alors nécessaire de relancer le test avant de réutiliser une ancienne URL.

Pour redémarrer après une correction sur le port déjà utilisé par l'opérateur,
la fixture TLS accepte une option **de test seulement** :

```powershell
$env:QUEUEATLAS_BROWSER_REVIEW = '1'
$env:QUEUEATLAS_BROWSER_REVIEW_PORT = '50104'
go test ./internal/httpapi -run '^TestWebBrowserReview$' -count=1 -v -timeout=35m
```

Port numérique1–65535, écoute toujours `127.0.0.1` ; port occupé/invalide : refus.
Sans cette option, port temporaire aléatoire comme auparavant. Aucun changement
de certificat/trust-store/TLS ni garde. Recharger GET `/login` avant un nouveau
POST pour obtenir les en-têtes corrigés ; éviter de renvoyer l'ancien formulaire.
Le mot de passe synthétique reste celui indiqué au début du guide.

Après la session, retirer la variable dans le terminal de lancement :

```powershell
Remove-Item Env:QUEUEATLAS_BROWSER_REVIEW
Remove-Item Env:QUEUEATLAS_BROWSER_REVIEW_PORT -ErrorAction SilentlyContinue
```

Sans opt-in, les deux fixtures sont skipped et la régression automatisée
`TestWebReviewImportedHostileValuesAcrossViews` s'exécute normalement.
Consigner navigateur, dimensions, actions, résultats et limites dans la
[revue Web164](reviews/m4-web.md), avec captures synthétiques.
