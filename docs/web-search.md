# Page Web de recherche protégée

Lot 159, bibliothèque M4. `httpapi.NewConsultationHandler(guard, store, options)`
retourne un seul routeur protégé pour `/messages` et les routes API existantes.
Partager cette instance : admission, Store et délais sont communs. Monter les
routes d'authentification séparément. `NewSearchHandler` reste limité à l'API.
Aucun listener, commande serve ou formulaire de connexion n'est créé ici.

## Recherche et pagination

GET/HEAD `/messages` sans paramètres montre le formulaire sans lire SQLite.
Défauts : dernières 24 heures UTC, 50 événements, critère expéditeur. L'instance
est saisie explicitement ; aucun inventaire de configuration n'est exposé.

Une requête utilise le [validateur API](http-search.md) : instance, un des six
critères exacts, valeur, dates UTC début inclusif/fin exclusive et limite1..200.
Même plafond de query, période maximale31jours, paramètres fermés et curseur
canonique. Une adresse vide reste recherchable ; les domaines sont normalisés.
Le lien suivant conserve le critère, la taille et les dates effectives, y compris
si la première requête omettait les dates. Un nouveau formulaire commence une
nouvelle recherche, sans conserver le curseur précédent.

Les lignes sont des événements correspondants : un candidat peut apparaître
plusieurs fois. Ses comptes/réserves proviennent des faits complets de la file,
pas de la seule page. Date/qualité, provenance et offsets décimaux sont affichés.
`sent` décrit le transport, pas une livraison finale. Les avertissements NOQUEUE
ne deviennent pas des rejets ; les faits sans génération restent non attribués.
Un résultat vide ne prouve pas l'absence dans les journaux.

## Protection et limites

- Même garde que l'API : HTTPS direct, Host/Origin vérifiés, session résolue à
  chaque requête, expiration/révocation, `no-store`, `nosniff`, aucune confiance
  ajoutée dans les en-têtes proxy. Pas de redirection de connexion automatique.
- Rendu `html/template`, données SMTP/provenance/champs toujours en texte ou
  attributs échappés. Aucun HTML/URL actif issu des journaux. Seule la CSS immuable
  embarquée est déclarée CSS de confiance ; ses octets exacts déterminent le hash
  CSP. Aucun JavaScript, police externe ou dépendance réseau.
- CSP : `default-src 'none'`, style embarqué autorisé par SHA-256,
  `form-action 'self'`, `base-uri 'none'`, `frame-ancestors 'none'` ;
  `X-Frame-Options: DENY` et `Referrer-Policy: no-referrer`.
- Budgets identiques à l'API, partagés entre Web/recherche/détail/timeline :
  délai5s,1024faits,2requêtes par défaut. Dépassement de faits422 ; admission429 ;
  erreur interne, annulation ou réponse HTML >1MiB :503. Paramètres invalides400.
  HTML entièrement tamponné avant succès : aucun résumé partiel. HEAD effectue
  les mêmes contrôles/lectures sans corps. Le plafond encodé n'est pas une limite
  de RAM totale et les délais ne préemptent pas les calculs purs.
- Les erreurs de recherche sont des pages fixes sans contenu privé. Les refus
  antérieurs de la garde/protocole gardent leur réponse texte/JSON existante.
  La recherche GET apparaît dans l'URL/historique ; le futur serveur devra éviter
  de journaliser les valeurs privées dans ses logs d'accès.

## Vérifications et suite

Quatre nouveaux tests,25tests HTTPAPI au total, Windows Go1.26 `-count=1` passent.
SQLite réel : pagination, reconstruction hors période, réserves, vide, NOQUEUE,
refus complet d'un budget insuffisant. Client HTTPS réel : texte hostile échappé,
offset >2^53 exact, HEAD, session révoquée. Admission commune API/Web, annulation
et expansion HTML >1MiB refusées sans résultats partiels ; CSP vérifiée contre
les octets de la CSS rendue. Format/diff et vet HTTPAPI passent.

Labels, aide, lien d'évitement, focus visible et tableau défilant sont présents.
Le parcours clavier, le rendu adaptatif et XSS/CSP de la source jusqu'au rendu
dans un navigateur réel restent à vérifier lors de la revue Web. Le test HTTPS
Go ne remplace pas cette validation. Aucun test navigateur revendiqué au lot159.

Depuis160, un candidat attribué possède un lien canonique local vers le
[détail Web protégé](web-detail.md). NOQUEUE/streams non attribués restent sans lien.
Depuis161, le détail mène à la [timeline Web](web-timeline.md) avec brut sous
permission explicite. Depuis162, [formulaire de connexion local](web-login.md)
et redirection fixe vers la recherche, même PR #37. Depuis163, bouton de
[déconnexion Web](web-logout.md). Au lot164, CSS/CSP normalisées en LF pour le
parseur HTML Windows et main reçoit `tabindex="-1"` pour le lien d'évitement.
Valeurs natives/provenances affichées en notation visible ; champs de recherche
conservés littéralement. [Revue navigateur partielle](reviews/m4-web.md) : rendu
statique synthétique contrôlé, parcours HTTPS authentifié encore à vérifier.
Montage serveur et fin de revue restent à réaliser.
L'issue #7 reste ouverte ; MIT conservée, AD/OIDC/Keycloak après MVP.
