# Login local borné — lot149

Moteur en bibliothèque dans [login.go](../internal/auth/login.go), raccordant le
compte143–147, VerifyPassword144 et les [sessions148](local-sessions.md).
Chantier [PR #35](https://github.com/Coubiac/QueueAtlas/pull/35) en brouillon.
Le [transport150](local-http-auth.md) ajoute login/logout/cookies ; la garde des
routes de données et la revue restent requises avant exposition applicative.

## Construction et résultat

NewLocalLogin(account, sessions, options) valide le compte, les options et le
magasin construit, sans lire de fichier, hacher, lire l'horloge ou tirer d'aléa.
Le compte et les options sont copiés, le pointeur du magasin partagé est conservé.
Charger le compte privé au démarrage avec LoadLocalAccount, puis construire **un
seul moteur** pour tous les appels du processus. Ne jamais copier son mutex ou
construire un moteur par requête/nom : cela réinitialiserait les limites.

Login(ctx, username, password) renvoie token frais, métadonnées Session et erreur.
Nom exact/casse sensible et mot de passe littéral doivent tous deux correspondre.
Le succès appelle Issue avec l'identité configurée et bénéficie des expirations,
révocations et capacité148. Aucun token fourni par le client n'est réutilisé.
Tous les échecs renvoient token vide et Session zéro ; jamais de session après
identifiants refusés. Le mot de passe appartient à l'appelant, n'est pas modifié
et ne doit pas être modifié simultanément. Aucun secret, token, hash, nom ou erreur
brute n'est journalisé/recopié dans les erreurs. La bibliothèque ne fait aucun log.

La validation syntaxique du nom et UTF-8/15..256runes/1024octets du mot de passe
précède Argon2id. Entrées malformées, mot de passe incorrect et nom inconnu
retournent ErrInvalidCredentials. Pour tout nom **bien formé**, même inconnu ou
de casse différente, VerifyPassword utilise le même hash configuré avec les mêmes
coûts. Aucune sortie anticipée selon l'existence du nom. Ceci ne revendique pas une
fonction complète constante en temps ou des délais observés identiques. La politique
d'enrôlement/liste de mots de passe n'est pas réappliquée au login d'un compte existant.

## Budget et admission

| Option | Défaut | Bornes inclusives |
| --- | ---: | --- |
| AttemptLimit | 5 | 1..100 |
| Window | 1min | 1s..1h |
| MaxConcurrent | 1 | 1..4 |

Options explicites, zéro refusé ; DefaultLoginOptions retourne une copie.
Choix applicatifs QueueAtlas, à réévaluer au pilote ; aucune option YAML livrée149.

Un budget unique couvre le compte local, tous les noms soumis et toutes les
origines. Pas de map par nom/IP, donc pas d'accumulation d'identifiants ni de
contournement en changeant le nom. Chaque tentative **admise** réserve un essai
avant toute validation/hachage : échec, entrée malformée, succès, erreur interne
ou annulation détectée après admission comptent. Un succès ne remet rien à zéro.

Fenêtre fixe démarrée à la première admission, réouverte si now >= début + Window.
Ce n'est pas une fenêtre glissante : deux budgets peuvent être proches de part et
d'autre de cette frontière. Un hachage traversant la frontière reste compté dans
sa fenêtre d'admission ; le plafond simultané reste applicable. Pas de bannissement
permanent, temporisation exponentielle, remboursement ou API de reset des essais.

Si budget épuisé ou MaxConcurrent occupé : ErrLoginLimited immédiate, sans file
d'attente applicative, hachage, session ou nouvel essai réservé. Un mutex bref
protège les compteurs ; il n'est retenu ni pendant Argon2id ni pendant Issue.
La place est conservée jusqu'au retour de la vérification/émission de session.
Avec les coûts144, au plus MaxConcurrent vérifications Argon2id de ce moteur
coexistent (mémoire nominale jusqu'à 4 × 256MiB). Ce n'est pas un plafond RSS,
des allocations encore à collecter, des requêtes HTTP ou des autres appels directs
à HashPassword/VerifyPassword/Issue. Tous les futurs chemins de login doivent
utiliser ce moteur partagé. Pas de coordination entre processus ; redémarrage ou
nouvelle instance perd le budget, comme les sessions mémoire.

Annulation/deadline déjà échue : erreur standard du contexte avant admission,
aucun essai réservé. Contexte nil/moteur nil ou zéro : ErrLoginUnavailable.
Après Argon2id, une annulation détectée refuse l'émission et libère la place.
Le hachage synchrone n'est **pas interruptible** ; annuler ne libère pas la place
avant sa fin. Aucun goroutine de calcul détaché ou délai garanti. Une annulation
peut courir avec la dernière vérification avant Issue : une fois Issue commencé,
la lecture d'entropie148 n'est pas annulable et une session peut être émise.
Le futur transport doit traiter l'échec d'envoi et les délais de requête.

Horloge time.Now, comparaisons monotones quand disponibles ; valeur nulle ou recul
depuis la dernière observation refuse l'admission/émission. Haut niveau conservé,
pas de reset sur succès ; refus jusqu'au rattrapage. Vérification après hachage
avant émission. Erreur de vérificateur, horloge ou magasin plein/entropie en échec
donne ErrLoginUnavailable fixe sans cause privée. Config invalide au constructeur
donne ErrInvalidLoginSetup ; LoginOptions.Validate donne ErrInvalidLoginOptions.

Les [recommandations OWASP d'authentification](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
motivent le refus générique, le coût de vérification pour noms inconnus et la
limitation des essais associée au compte. Le budget global de ce compte unique
peut être épuisé par un tiers et empêcher temporairement le propriétaire de se
connecter : limiter l'exposition et revoir ce compromis avec le transport/pilote.
Ces primitives isolées ne suffisent pas à valider la sécurité du login publié.

## Vérifications et suite

Huit nouveaux tests149 : configuration/valeurs zéro/copies/absence de dérivation
au constructeur ; vrai Argon2id succès/erreur/nom inconnu/casse et session fraîche ;
budget partagé/frontière exacte/succès sans reset ; entrées hostiles sans hachage
mais comptées ; deux vérifications bloquées et 32refus concurrents immédiats ;
annulations/erreurs privées/entropie avec place libérée sans session ; annulation
pendant calcul conservant la place ; horloge nulle/recul après calcul et capacité
de sessions sans éviction/reprise. Temps/verify injectés seulement via constructeur
privé ; production time.Now/VerifyPassword. Pas de sleep dans les tests.
Tests auth, vet, format et diff Windows réussis ; CI entière/Linux/race à vérifier
après publication149. Aucune exécution Linux locale revendiquée.

Prochain150 : transport HTTP login/logout et cookies. Contrôles transversaux
CSRF/TLS/routes, corpus local de mots de passe adapté/provenance/licence et revue
restent à réaliser avant clôture/authentification Web publiée. CLI/config/SQLite/
FileSource/modules/workflow inchangés149. MIT, AD/OIDC/Keycloak après MVP.

Validation149 effective : 94ef705 dans #35, CI37619138226 entière/trois jobs/SHA
exact/race auth réussis, REST revérifié150. Le [transport HTTP150](local-http-auth.md)
raccorde ce moteur partagé aux routes POST login/logout et cookies HTTPS ; vérifié
Windows, publication/CI à terminer au commit150. Garde des routes de données,
corpus d'enrôlement adapté et revue restent requis avant exposition applicative.
