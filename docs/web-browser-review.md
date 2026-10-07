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
Après la session, retirer la variable dans le terminal de lancement :

```powershell
Remove-Item Env:QUEUEATLAS_BROWSER_REVIEW
```

Sans opt-in, les deux fixtures sont skipped et la régression automatisée
`TestWebReviewImportedHostileValuesAcrossViews` s'exécute normalement.
Consigner navigateur, dimensions, actions, résultats et limites dans la
[revue Web164](reviews/m4-web.md), avec captures synthétiques.
