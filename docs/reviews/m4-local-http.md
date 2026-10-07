# Relecture M4 — sessions, login et protections HTTP, lot 153

## Périmètre et décision

Relecture assistée du chantier 148–152 dans #35 depuis main147
`9cd6cecaff00d58e43f5ca05b49271ed7ada7ae9`. Tête relue :
`e4aaa12e38892b9ad7f6fe18eb5becb4235704ca`. Il s'agit de la relecture du code et
des tests par l'agent du chantier, pas d'un audit indépendant.

**Avis favorable à la clôture de cette bibliothèque d'authentification.** Aucun
blocage fonctionnel identifié dans le périmètre examiné. Deux commentaires Go
obsolètes sont corrigés : le package et HTTPHandler proposent désormais le
transport HTTP et Protect. Aucun changement de comportement ou nouveau test
n'est nécessaire pour ces commentaires.

La demande de corpus/provenance/licence/tests/guide de la revue 147 est traitée
par 152 et évaluée ci-dessous. Les composants restent à monter dans l'application,
avec leurs limites réseau et des essais navigateur. M4 demeure en cours : aucun
serveur exécutable, API de messages ni interface de recherche livré par #35.

## Contrôles examinés

| Sujet | Contrat retenu | Preuves relues |
| --- | --- | --- |
| Sessions | Token CSPRNG 32 octets, codec canonique, digest seul en mémoire ; capacité stricte, pas d'éviction active | Tests148 émission/codec/collision trois essais/capacité/32 créateurs |
| Durées et révocation | Deadlines absolue et inactive exactes, activité seulement sur résolution admise, révocation idempotente, restart invalide tout | Tests148 frontières/horloge/revoke ;151 identité étrangère sans activité |
| Login | Compte immuable, même Argon2id pour noms bien formés inconnus, nom/secret littéraux, erreurs privées et session fraîche après succès | Tests149 public Argon2id/noms inconnus/metadata ;152 ancien compte toujours accepté |
| Admission | Budget unique tous noms, succès sans reset, plafond simultané sans file, slot conservé pendant calcul annulé | Tests149 fenêtre exacte/32 refus concurrents/annulation/horloge/saturation |
| Transport | TLS direct, Host exact, Origin unique exact sur login/logout et mutations ; metadata complémentaire, aucun proxy de confiance | Tests150/151 TLS absent, Host/Origin/Referer/Forwarded, metadata absente/étrangère/dupliquée |
| Entrées | Formulaire et cookies bornés, ambiguïtés/champs inconnus refusés avant hash ; octets du secret conservés | Tests150 corps4096/UTF-8/formes hostiles/cookies ; CLI152 code2 sans compte |
| Cookies et sortie | Secure/HttpOnly/SameSite Strict/préfixe Host/Path racine sans Domain ; remplacement après succès, logout serveur ; rollback d'un token si réponse échoue | Tests150 HTTPS réel/login/relogin/logout et écriture échouée/partielle/panique/annulation |
| Routes protégées | Aucun chemin exempté sous Protect, token actif du compte local exigé, identité en contexte copiée ; aucun bypass headers/query/ancien contexte | Tests151 protocole avant lecture/activité,401 privé commun, copie,32 requêtes puis revoke |
| Réponses | no-store/Pragma/nosniff/Vary, erreurs fixes sans valeur/cause, aucune redirection/CORS | Assertions150/151 sur succès/refus et diagnostics CLI152 |
| Corpus | Source publique figée sous MIT, import vérifié, empreintes embarquées, comparaison complète, aucune politique de création au login | Tests152 intégrité de toutes les entrées/fixtures synthétiques/ancien compte ; générateur sortie préservée si source fausse |

## Évaluation du corpus et des essais

Le corpus SecLists/Xato conserve 10 898 empreintes uniques de valeurs source
compatibles avec les bornes de création, issues du sous-ensemble public d'un
million de lignes. Révision, origine historique, checksum source/sortie, procédure
reproductible et licence complète sont dans [le contrat du corpus](../password-blocklist.md).
La notice MIT amont est conservée dans le dépôt et devra accompagner les paquets.
Les fixtures utilisent des motifs synthétiques ; aucun compte réel utilisé.

Le minimum de 15 caractères et cette sélection de valeurs longues, complétés par
les exemples/dérivés locaux, sont **retenus pour le compte local du MVP avec les
défauts actuels de limitation** : 5 admissions/minute, un calcul simultané.
C'est un choix d'ingénierie documenté, pas une preuve statistique de résistance.
La fenêtre renouvelle le budget, peut permettre deux budgets rapprochés et
redémarre avec le processus. Le corpus est historique, fini, sans mise à jour
automatique ; il ne couvre pas toutes les compromissions ou variantes.

Le [NIST §3.1.1.2](https://pages.nist.gov/800-63-4/sp800-63b.html#passwordver)
préconise comparaison complète, liste adaptée aux essais et guide après rejet.
Cela motive le choix sans certifier la sélection ni une conformité NIST globale.
Le guide propose une nouvelle valeur générée ou une longue passphrase originale.
VerifyPassword/LocalLogin ne réappliquent jamais le corpus aux comptes existants.
Réexaminer au pilote et si budget/bornes/providers ou information de compromission
changent ; aucun nouveau lot de corpus n'est requis pour clore #35 en l'état.

## Limites d'intégration conservées

- Construire et partager une seule instance de login/magasin/handler au démarrage,
  puis monter auth séparément et protéger le routeur entier des données.
- Le chemin livré exige TLS direct, y compris loopback. Proxy TLS vers HTTP,
  confiance Forwarded et authentification fédérée ne sont pas implémentés.
- Définir certificats, bind, bornes de headers et délais lecture/écriture/shutdown
  du futur serveur. Corps borné et budget Argon2id ne bornent pas les connexions,
  la durée du parsing ou toutes les ressources du processus.
- Les handlers de données doivent valider leurs entrées, être sans mutation sur
  GET/HEAD, conserver cache/CORS et appliquer les permissions applicatives.
  SessionFromContext fournit une identité, pas un rôle/RBAC ni une preuve portable.
- Révocation/expiration bloquent les nouvelles admissions ; une requête déjà
  admise peut finir. Argon2id/aléa synchrones ne deviennent pas interruptibles.
- Les tests HTTPS/cookiejar ne prouvent pas le comportement SameSite/préfixe dans
  un vrai navigateur. Vérifier montage et flux navigateur avec le Web/pilote,
  politique stricte des metadata inconnues, rendu échappé/XSS et cache.
- Choix d'origine/metadata cohérents avec la [recommandation OWASP CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html),
  avec refus volontaire des valeurs inconnues ; aucun contournement par Referer,
  sous-domaines ou CORS configuré. XSS n'est pas corrigé par ce middleware.

## Vérifications et clôture

[CI15237631390279](https://github.com/Coubiac/QueueAtlas/actions/runs/37631390279)
entière completed/success ; trois jobs Windows/Linux Go1.26.x/stable et leurs SHA
exacts revérifiés REST153. Race auth1.26, tests/vet/format, smoke/race source-file
et builds statiques Linux amd64/arm64 verts. Pas de Linux local revendiqué.

Dix tests ciblés auth et un test du générateur relancés Windows153 : cycles HTTPS
auth/données/logout, rollback de sortie, protocole/bypass/401 privés, concurrence
login/sessions/guard, intégrité et ancien compte Argon2id, import hostile. Tous
passés ; vet auth et format/diff réussis. Résultats complets152 réutilisés ; pas de
rerun optionnel des fondations/fuzz/mesures sans risque nouveau.

Au commit153 : publication/CI finale, revue COMMENT sur SHA final, PR prête,
fusion puis CI main et nettoyage encore à terminer. La preuve effective sera
consignée dans #35 puis au début de la reprise154. MIT conservée, AD/OIDC/Keycloak
après MVP. Prochain154 : contrat borné des requêtes de recherche HTTP, sans
combiner serveur, stockage ou interface dans le même lot.
