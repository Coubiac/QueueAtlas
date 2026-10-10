# Revue assistée Web159–164 — PR #37

État au 10 octobre 2026 : **revue incomplète**, PR conservée en brouillon.
Après correction b845d95 (CI37707149012 entière verte), l'utilisateur confirme
la connexion et ABC123 dans les résultats de recherche. Détail/timeline/logout
restent à vérifier. Retour sur la lisibilité et les filtres consigné ci-dessous.
Aucun lot165 commencé.

## Retour humain du 10 octobre : diagnostics incompréhensibles

Code0f58cfd publié, CI37812053841 entière success sur SHA exact/trois jobs,
preuve dans PR37/issue7. Nouvelle fixture42023, l’utilisateur confirme login,
puis capture des résultats ABC123/alice/bob ; aucune preuve du détail ni logout.
Il demande une présentation compréhensible des positions et dates.
Correction164 : suppression de la sixième colonne technique en recherche,
références gardées dans le diagnostic du détail et volets fermés de l’historique.
Offsets expliqués sans faux numéro de ligne. Dates UTC lisibles, provenance du
fuseau/année expliquée en phrases, sans notation « qualité de la date ».
Tests existants adaptés aux données sélectionnées, plafond HTML/XSS maintenus ;
suite HTTPAPI PASS3.958s et format/diff passés. Publication/CI du nouveau commit
à vérifier ; relancer la fixture pour la revue humaine. Détail/timeline/logout
restent à constater ; pas de nouvelle preuve DOM par outil navigateur.

## Second retour humain : informations mail et routage

L'utilisateur attend une fiche date/fuseau, expéditeurs SMTP et From:, RCPT TO
multiples, Message-ID/Queue ID, source SMTP/destination, événements/codes/motifs/
états et durée. L'allègement précédent9b97100 ne suffisait pas. CI de cet état
antérieur37809143458 entière success/SHA exact/trois jobs, consignée dans #37.
Correction dans164 : vue HTML de génération pour recherche et détail, champs
sélectionnés à partir des faits déjà lus ; API/JSON et stockage inchangés.
[Matrice détaillée](../message-tracking-view.md). Absence des en-têtes et état
final/global non déterminé restent explicitement visibles, aucun champ inventé.
Native delay par tentative et SMTP en préfixe de réponse, DSN distinct.

Deux tests utiles ajoutés : import Postfix réel/SQLite/vues, valeurs opérationnelles
et autre origine même Queue ID exclue, non-fuite JSON ; code SMTP distinct des
DSN/quotes/local/absence. Suite HTTPAPI Windows Go1.26 PASS2.520s/vet/diff ; tests
précédents de budget/HEAD/révocation/XSS/base64/vide/conflit conservés/adaptés à
la sélection élargie explicite du détail Web (Message-ID désormais affiché).
Nouveau DOM/rendu non encore constaté par l'utilisateur ; revue incomplète.

Fixture TLS humaine remplacée par cinq faits lisibles, alice transmis250 et bob
différé450 avec client/Message-ID/relais/delay. Fixture hostile inchangée pour
les preuves de sécurité ; ne pas confondre les deux jeux. Au commit publication/
CI exacte à terminer ; redémarrer serveur, nouveau login/recherche/détail,
timeline cinq faits/logout restant à confirmer. PR37 brouillon, aucun165.

## Retour humain du 8 octobre et correction de lisibilité

Fixture TLS relancée sur50104, code b845d95, session19343. L'utilisateur confirme
« la page de recherche apparait », puis « oui » pour ABC123 après recherche
expéditeur aux dates explicites. Capture des résultats fournie : preuve humaine
du login et de la soumission de recherche, pas de rotation/SameSite/logout.
L'outil navigateur reste inutilisé après son refus de sécurité.

Retour : provenance/hash/offsets/génération et compteurs zéro trop présents ;
dates de formulaire difficiles, critères expéditeur ET/OU destinataire ET/OU
sujet et correspondances début/contient/fin souhaités. Correction164 limitée
au rendu des résultats : date lisible UTC, comptes non nuls avec labels lisibles,
informations techniques accessibles via details/summary fermé. Avertissement
global sur couverture/remise finale conservé ; faits/API/SQL inchangés.
Suite HTTPAPI Windows Go1.26 -count=1 PASS2.474s, vet et diff-check passent.
Test SQLite existant adapté : pagination/réserves inchangées, provenance dans
le volet fermé et date canonique conservée. Rendu hostile HTTPS, budgets et
headers restent vérifiés par la suite ; aucune nouvelle preuve DOM revendiquée.

[Découpage des extensions de recherche](../search-ux-follow-up.md) accepté mais
non réalisé par ce correctif. Au commit : publication/CI à vérifier. Reprendre
le parcours humain sur le serveur redémarré ; l'ancien processus ne recharge
pas les assets embarqués. Détail/timeline/logout non confirmés ; #37 brouillon.

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
refusé. L'utilisateur signale ensuite « Requête de connexion refusée » et fournit
une capture du formulaire en erreur. Le parcours n'a pas réussi ; diagnostic et
correction ci-dessous. Rotation, attributs des cookies et SameSite doivent
être qualifiés séparément ; ces étapes seules ne démontrent pas tous ces points.

[CI164](https://github.com/Coubiac/QueueAtlas/actions/runs/37676596678)
entière completed/success sur `ef87ceb6b24d5ad20538ecda5bdf23af75965083` :
Go1.26 112981583001, Windows112981583298, stable112981583404. Auth/HTTPAPI Windows
et race Linux1.26 réussis ; stable race skipped prévu. Preuves réutilisées de la
publication des corrections, sans relancer les tests du code inchangé.

Checkpoint documentaire140e50 publié, CI37682026040 entière completed/success
sur `140e50decddfa59f741fbca8087104a373258421` : stable113000216367,
Windows113000216586, Go1.26 113000216796. Auth/HTTPAPI Windows et race Linux1.26
réussis, stable race skipped prévu ; preuve post-publication dans #37.

## Refus de connexion et correction de Referrer-Policy

Le message exact utilisateur/capture correspond au refus403 rendu avant
vérification des credentials. L'Origin réellement reçu n'a pas été capturé :
l'accès de l'outil au navigateur reste refusé. Le compte de test reste valide
dans les tests de vérification Argon2id.

Défaut de compatibilité identifié : toutes les pages envoyaient
`Referrer-Policy: no-referrer`. Le
[standard Fetch](https://fetch.spec.whatwg.org/#append-a-request-origin-header)
spécifie Origin `null` sur un POST natif non-CORS avec cette politique ; la garde
exige l'origine exacte et refuse donc le formulaire. Ce comportement est aussi
[documenté par MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Referrer-Policy#effect_on_the_origin_header).
Le même défaut affectait les boutons logout des trois vues.

Politique Web remplacée par `strict-origin` : conserve Origin pour un formulaire
HTTPS→HTTPS ; Referer contient au plus l'origine, sans chemin ni critères privés,
et reste absent lors d'un downgrade HTTPS→HTTP. CSP, TLS direct, Host/Origin exact,
Fetch-Site, cookies Secure/HttpOnly/SameSiteStrict et refus d'Origin absent/null/
étranger inchangés. Aucun fallback Referer ni nouvelle confiance proxy.

Tests HTTPS login/logout renforcés : GET du formulaire/vues, puis Origin modélisé
selon la politique effectivement reçue au lieu de forcer un Origin réussi.
Régression reproduite avec l'ancienne politique : les deux tests échouent ;
source corrigée restaurée, les deux passent. Cette modélisation suit Fetch,
**sans démontrer un parcours navigateur réel réussi**.
La suite a révélé un test NOQUEUE utilisant la fenêtre de la date courante sur
des données du7octobre ; dates explicites fixées dans ce test, sans changement
du comportement serveur ou des données. Suite Windows Go1.26 auth1.349s/
HTTPAPI2.487s et vet passent ; format/diff vérifiés avant publication.

Fixture TLS : port optionnel borné1–65535, loopback127.0.0.1 forcée, pour reprendre
une URL déjà utilisée après redémarrage. Démarrage vérifié sur50104 puis arrêt
propre PASS17.145s ; aucune interaction navigateur revendiquée. L'ancien serveur
59518 a expiré après30minutes (FAIL attendu par timeout) ; aucun serveur de test
ne reste actif à ce checkpoint. Relancer avant un essai utilisateur.

## Décision et reprise

Corrections publiées/CI validée, même PR37 brouillon. Reprendre **le lot164**
sur le correctif Referrer-Policy et un **nouvel essai manuel**. Relancer la
fixture selon le guide, recharger GET login puis tester avec le même compte.
Documenter les résultats réels ; passer prêt/fusionner
seulement si les critères sont satisfaits, puis vérifier la CI main.
Montage serveur applicatif et filtres/diagnostics restent ensuite ; issue7/M4/MVP
ouverts. MIT et authentification AD/OIDC/Keycloak après MVP inchangés.
