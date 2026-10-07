# Sessions locales en mémoire — lot148

Selon [ADR-006](adr/ADR-006-authentication.md), primitives dans
[internal/auth/sessions.go](../internal/auth/sessions.go). Le compte/CLI143–147 est
fusionné dans #34 sur9cd6cec ; CI finale37611573008/main37611762238 entièrement
réussies. Ce magasin ne vérifie pas un mot de passe et ne crée aucune route HTTP.

## Options et cycle de vie

| Option | Défaut | Bornes inclusives |
| --- | ---: | --- |
| Lifetime | 8h | 1min..24h |
| IdleTimeout | 30min | 1min..Lifetime |
| Capacity | 64 | 1..1024 |

Choix applicatifs explicites, à ajuster au pilote ; aucun défaut implicite pour
une option zéro. DefaultSessionOptions renvoie une copie. NewSessionStore valide
sans IO/aléa/lecture d'horloge et conserve les options par valeur. Partager le
pointeur construit, sans copier le magasin après usage ; valeur zéro/nil inutilisable.

Issue(identity) doit être appelé **seulement après authentification** par le futur
login : Validate de l'identité est syntaxique et ne prouve ni mot de passe ni rôle.
Retourne token, Session et erreur ; en erreur token vide/Session zéro. Token de
32octets crypto/rand, base64url sans padding canonique43caractères, aucune donnée
personnelle encodée. Le serveur indexe par SHA-256 des32octets, ne conserve pas le
bearer dans la map. Jamais de token fourni par le client à Issue.

Session contient Identity, CreatedAt, LastSeenAt et ExpiresAt (date absolue).
Métadonnées indépendantes par valeur, sans secret/credential/rôle. Le caller doit
protéger le bearer retourné, ne pas le journaliser. Un digest stocké fourni comme
token n'est pas accepté comme preuve : le token reçu est décodé puis haché.

Resolve(token) accepte seulement une session émise et active. Malformé/inconnu/
expiré/révoqué donnent la même ErrInvalidSession fixe et Session zéro. Validation
de43caractères/base64url strict/canonique/taille32 avant lookup. Une lecture valide
avance LastSeenAt, jamais ExpiresAt ; à now >= ExpiresAt ou now >= LastSeenAt +
IdleTimeout, refus et retrait. L'expiration exacte est exclusive, activité ne
permet pas de dépasser Lifetime. Pas de prolongation automatique absolue/rotation.

Revoke(token) retire une session, idempotent même pour token inconnu/malformé ;
RevokeAll retire toutes les sessions du magasin. Ni horloge ni aléa requis pour
révoquer. Nouveau magasin/redémarrage = aucune session précédente valide, pas de
persistance, réplication ou coordination entre processus.

## Bornes, erreurs et concurrence

Mutex sérialise Issue/Resolve/Revoke/RevokeAll, inclut dépendances privées du test.
Au plus Capacity entrées ; saturation refuse avant aléa avec ErrSessionsFull, aucune
éviction d'une session active. Expirées retirées au prochain Issue/Resolve bien
formé, scan au plus1024entrées ; aucun goroutine/timer de purge. Sans opération,
entrées expirées peuvent rester en mémoire mais ne deviennent pas acceptables.
Pas de mesure ni plafond exact du RSS/allocateur Go revendiqué.

Collision d'un digest actif : nouveau tirage, au plus trois essais ; jamais de
remplacement. Entropie partielle/erreur ou collisions répétées = ErrSessionUnavailable,
aucun token/session partiel, error IO brute non exposée. Une tentative échouée peut
avoir purgé des entrées expirées. Horodatage de création après entropie terminée.
Les32octets temporaires sont clear au mieux, pas de garantie d'effacement global.

Horloge réelle time.Now en production, avec comparaison monotone quand disponible.
Temps nul ou inférieur au dernier temps observé = ErrSessionUnavailable, refus
sans résoudre/installer de session ; attendre que le temps revienne au niveau
observé. La révocation reste possible. Dernier temps conservé après RevokeAll.
Il ne s'agit pas d'une horloge externe attestée ou d'un mécanisme contre un
processus/OS compromis. Mutex retenu durant crypto/rand : pas de timeout d'entropie.

Erreurs fixes : ErrInvalidSessionOptions, ErrInvalidSession, ErrSessionsFull,
ErrSessionUnavailable et ErrInvalidLocalIdentity pour Issue. Aucune valeur/token/
identité/heure/erreur brute recopiée. Lookup par map/digest et fonction complète
non revendiqués constants en temps. Les indices à sens unique ne protègent pas
une fuite du cookie, modification des métadonnées ou compromission du processus.

Les [recommandations OWASP de sessions](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
préconisent des secrets CSPRNG et des expirations serveur absolue/inactive. Le
choix256bits et les options ci-dessus sont QueueAtlas ; aucune conformité globale
du login n'est déduite de ce magasin isolé.

## Vérifications et suite

Six nouveaux tests148,21auth/vet/format/diff Windows passés : bornes/options/valeurs
zéro, tokens publics distincts et canonique/ownership/digest/revoke/restart,
inactivité et expiration absolue exactes/activité/capacité/reclamation, tokens
hostiles/digest non bearer et rejet privé unique, entropie partielle/erreurs/
trois collisions/absence d'aléa en entrée invalide ou saturation, horloge nulle/
recul avant et après entropie, horodatage après entropie, révocation malgré anomalies,
32créateurs/capacité8 et Resolve/Revoke concurrents. Horloge/entropie injectées par
constructeur privé test-only ; production toujours time.Now/crypto/rand. Pas de
sleep dans les tests. Tests auth Linux23/race via workflow existant à vérifier
après publication148. CLI/config/SQLite/FileSource/modules/workflow inchangés.

Prochain149 : login borné, admission/limitation d'essais et traitement des comptes
inconnus, raccordement VerifyPassword/SessionStore, sans HTTP. Ensuite transport
HTTP/cookies, contrôles transversaux CSRF/TLS/routes/liste de mots de passe adaptée
et revue/clôture. Aucun serveur/login/cookie/Web livré par148, aucune entrée YAML
sessions ou reset compte. Liste d'enrôlement non déclarée suffisante pour release
login selon [revue147](reviews/m4-local-account.md). MIT, AD/OIDC/Keycloak après MVP.

Validation148 effective : bd862cd publié dans #35, CI37615586790 entière/trois jobs/
SHA exact/race auth réussis, REST revérifié à la reprise149. Le [moteur de login149](local-login.md)
partage ce magasin et n'appelle Issue qu'après vérification réussie du compte,
avec budget d'essais/admission bornés. Le contrat d'Issue reste requis pour tous
ses appelants. Transport HTTP/cookies150, protections et revue restent à réaliser.
