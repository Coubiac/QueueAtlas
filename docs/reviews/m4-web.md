# Revue assistée Web159–164 — PR #37

État au 7 octobre 2026 : **revue incomplète**, PR conservée en brouillon.
Les corrections du rendu sont publiées ef87ceb, CI37676596678 entière verte ; le parcours navigateur
HTTPS connexion/déconnexion reste à vérifier avant fusion. Aucun lot165 commencé.

## Périmètre et méthode

Recherche, détail, timeline, connexion et déconnexion locales des lots159–163.
Départ sur `33596e2f3a569fc47b30bf69fea0f22c2f942d02`, main158 `8c3aa85`.
[CI163](https://github.com/Coubiac/QueueAtlas/actions/runs/37675032968)
entière réussie, SHA exact et trois jobs revérifiés REST : Go1.26 112976207709,
stable112976208275, Windows112976208332. Auth/HTTPAPI Windows et race auth/HTTPAPI
Linux Go1.26 réussis ; stable race skipped comme prévu.

Trois lignes RFC5424 Postfix **synthétiques**, dont une adresse contenant une
balise image et U+202E, et une réponse SMTP contenant script/lien `javascript:`.
ImportFile réel → parseur Postfix → SQLite → reconstruction/handlers protégés →
HTML → DOM du navigateur Codex. Les pages sont générées avec une session de test
autorisée, puis seules ces pages statiques synthétiques sont servies sur loopback
HTTP pour examiner le rendu. Ce second serveur ne fournit aucune session ni
connexion et refuse POST ; il ne démontre ni authentification HTTPS navigateur,
ni exécution SQL d'une nouvelle recherche soumise depuis le navigateur.

[Guide reproductible](../web-browser-review.md). Le serveur TLS de test a été
refusé avec `ERR_CERT_AUTHORITY_INVALID`. La compétence Computer Use exige une
intervention humaine pour franchir cet avertissement ; aucune exception TLS ni
modification du magasin de confiance n'a été effectuée.

## Résultats et corrections164

| Contrôle | Résultat réellement observé |
| --- | --- |
| CSS/CSP sous Windows | Défaut trouvé : CRLF embarqués, normalisés en LF par le parseur HTML, invalidaient le hash CSP. Normalisation LF avant rendu et hash ; style appliqué après correction, politique inchangée. Test de hash adapté à la normalisation HTML. |
| Lien « Aller au contenu » | Défaut trouvé : focus restait sur BODY. `tabindex="-1"` ajouté aux quatre cibles main ; Tab puis Entrée donne le focus à MAIN sur la recherche dans le navigateur. |
| Valeurs de présentation | Contrôles de direction encore invisibles dans le détail. Notation visible appliquée aux valeurs natives/provenances affichées dans recherche et détail, comme la timeline. Champs de saisie/requêtes, identifiants canoniques et octets SQLite restent exacts. |
| Import et échappement | Un candidat retrouvé depuis le fichier synthétique ; adresse et réponse hostiles affichées comme texte dans détail/timeline/brut. DOM : zéro script/image/iframe/object/embed et zéro lien `javascript:` ; aucun contrôle de direction caché dans main. |
| Clavier timeline | Entrée sur le lien détail→timeline ; Tab du champ limite vers la case brute, Espace puis Entrée sur le bouton : `raw=1`, case cochée et trois blocs pre. Il s'agit de snapshots prédéfinis, pas d'un test de permission depuis le navigateur. |
| Rendu desktop/mobile | Détail desktop : largeur document/corps1265px. Recherche et timeline à375×812 ; timeline document/corps360px hors scrollbar, aucun débordement horizontal extérieur. Header/formulaire lisibles, contrôles accessibles ; pas d'audit exhaustif d'accessibilité ni de tous les navigateurs. |
| Auth HTTPS/SameSite | **À faire** : connexion/rotation, session protégée, déconnexion/replay et comportement SameSite dans un navigateur réel. Les tests Go HTTPS/cookiejar existants passent, sans remplacer ce contrôle. |

Une nouvelle régression automatisée couvre le fichier importé, la recherche
exacte de l'adresse hostile et l'échappement/notation visible dans détail et
timeline brute. Deux fixtures manuelles sont opt-in et skipped en CI normale.
Windows Go1.26 : `go test ./internal/auth ./internal/httpapi -count=1` réussit
(auth1.501s, HTTPAPI2.554s), `go vet` sur les deux packages réussit ; format et
diff vérifiés avant commit. Aucun Linux local ni test navigateur multiengine.

## Captures du navigateur après corrections

Les captures représentent uniquement les données synthétiques du serveur de
rendu statique ; le bouton de déconnexion visible n'y exécute pas l'auth réelle.

- [Recherche mobile](assets/web164-search-mobile.jpg)
- [Détail desktop complet](assets/web164-detail-desktop.jpg)
- [Timeline mobile, brut demandé](assets/web164-timeline-mobile.jpg)

## Reprise après intervention sur le certificat

L'utilisateur confirme « c'est fait. Page visible » pour la fixture TLS relancée
sur le code ef87ceb. Le contexte ambiant indique cette page dans le navigateur
de Codex. Cette confirmation atteste uniquement l'affichage du formulaire.
L'outil Computer Use refuse ensuite la lecture de l'onglet par politique de
sécurité ; aucun contournement ni autre surface n'est tenté.

Un rapport manuel est demandé pour connexion, recherche exacte de l'expéditeur
synthétique, détail ABC123→timeline trois faits, logout→login et accès protégé
refusé. Résultats en attente. Rotation, attributs des cookies et SameSite doivent
être qualifiés séparément ; ces étapes seules ne démontrent pas tous ces points.

[CI164](https://github.com/Coubiac/QueueAtlas/actions/runs/37676596678)
entière completed/success sur `ef87ceb6b24d5ad20538ecda5bdf23af75965083` :
Go1.26 112981583001, Windows112981583298, stable112981583404. Auth/HTTPAPI Windows
et race Linux1.26 réussis ; stable race skipped prévu. Preuves réutilisées de la
publication des corrections, sans relancer les tests du code inchangé.

## Décision et reprise

Corrections publiées/CI validée, même PR37 brouillon. Reprendre **le lot164**
sur le rapport manuel demandé après le refus d'accès de l'outil. Si la fixture
a expiré, relancer selon le guide. Documenter les résultats réels ; passer prêt/fusionner
seulement si les critères sont satisfaits, puis vérifier la CI main.
Montage serveur applicatif et filtres/diagnostics restent ensuite ; issue7/M4/MVP
ouverts. MIT et authentification AD/OIDC/Keycloak après MVP inchangés.
