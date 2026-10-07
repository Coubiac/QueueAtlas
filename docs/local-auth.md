# Authentification locale — chantier M4

## Lot143 : identité locale et paramètres Argon2id

Contrat pur dans [internal/auth](../internal/auth/local.go), selon
[ADR-006](adr/ADR-006-authentication.md). Il prépare le compte administrateur local
initialisé ultérieurement par CLI ; aucune authentification ou autorisation livrée.

`LocalIdentity{Username}` / `NewLocalIdentity` : identifiant ASCII de 1 à 64octets,
premier caractère alphanumérique, suivants alphanumériques ou point/tiret/underscore.
Sensible à la casse, sans trim/case folding ; aucun compte ou nom implicite.
Validate ne modifie rien, ne recherche pas le compte et ne vérifie pas son unicité.
New renvoie une valeur indépendante validée ou la valeur zéro en erreur.
ErrInvalidLocalIdentity est fixe, sans identifiant fourni. La valeur ne contient
ni mot de passe, ni hash, ni rôle ; elle n'est pas une preuve d'identité authentifiée.
Les futurs fournisseurs AD/OIDC garderont un espace provider/subject distinct,
selon ADR-008, sans assimiler email/nom d'affichage et clé d'autorisation.

`Parameters{MemoryKiB, Iterations, Parallelism}` / `DefaultParameters` :

| Paramètre | Défaut | Valeurs admises |
| --- | ---: | --- |
| Mémoire | 65536Kio (64Mio) | 65536..262144Kio (64..256Mio) |
| Passes | 3 | 3..6 |
| Voies | 4 | 1..4 |
| Version Argon2id | 19 | Fixée pour le futur codec |
| Sel | 16octets | Longueur fixée pour le futur codec |
| Hash | 32octets | Longueur fixée pour le futur codec |

Le défaut reprend la seconde recommandation de la
[RFC9106 §4](https://www.rfc-editor.org/rfc/rfc9106.html#section-4), destinée à une
mémoire disponible réduite. La [documentation Go Argon2](https://pkg.go.dev/golang.org/x/crypto/argon2)
décrit les mêmes unités et le futur appel IDKey. Les plafonds et planchers admis
sont une **politique QueueAtlas**, pas l'ensemble du domaine de l'algorithme ni
une garantie de latence. Le profil de 2Gio de la première recommandation RFC est
hors du budget choisi ici. Mesures sur le matériel pilote requises avant release.

MemoryKiB doit être multiple de quatre fois Parallelism, pour éviter un arrondi
implicite de mémoire. Les zéros et débordements de bornes sont refusés, sans
sélection silencieuse de défauts. Validate retourne ErrInvalidParameters avec
champ/règle fixe, sans valeurs fournies ; aucune mutation ou allocation Argon2.
Les défauts sont des valeurs indépendantes, pas des paramètres globaux mutables.

Ces bornes limitent le coût d'un futur appel ; elles ne bornent pas plusieurs
appels simultanés, la mémoire du processus ou le temps écoulé. Une admission
bornée, une limitation des essais et la protection contre les comptes inconnus
restent à implémenter à la frontière de login. Les paramètres persistés devront
être validés avant tout hash, pas après son calcul.

Quatre tests Windows et vet/format/diff passés : identité littérale/casse/ownership,
refus/confidentialité/erreur zéro, défauts indépendants/profil fixe, bornes inclusives/
zéros/maxima numériques/alignement (y compris trois voies). Tests synthétiques,
sans données privées ou calcul cryptographique. Le workflow Linux existant teste
le nouveau package via go test/vet ./... ; workflow inchangé.

## Suite au lot143 (snapshot)

Lot144 : hachage/vérification Argon2id avec bibliothèque maintenue, sel aléatoire,
codec strict/borné des hashes et comparaison constante, tests/vecteurs et doc.
Politique de mot de passe à définir à cette étape avant toute création. Aucun
format de hash ni paramètre YAML d'auth accepté au143, aucune dépendance ajoutée.
Persistance atomique du compte, CLI d'initialisation, sessions/révocation,
protections HTTP/CSRF/essais et revue dans des lots suivants. Aucun serveur,
cookie, route login ou accès distant n'est activé. MIT conservée ; AD/OIDC après MVP.

Publication143 effective : e00573c dans [PR #34](https://github.com/Coubiac/QueueAtlas/pull/34),
[CI37598069906](https://github.com/Coubiac/QueueAtlas/actions/runs/37598069906)
entière réussie/trois jobs/SHA exact ; les attentes143 sont terminées.

## Lot144 : hash et vérification Argon2id

`HashPassword(password []byte, Parameters)` utilise `golang.org/x/crypto/argon2`
v0.57.0, version officielle épinglée et compatible Go1.26. Paramètres143 et secret
validés avant génération d'un sel frais de 16octets avec crypto/rand, puis IDKey.
Le résultat est un hash de 32octets encodé avec sa version, ses coûts et son sel :

```text
$argon2id$v=19$m=65536,t=3,p=4$<sel-base64-sans-padding>$<hash-base64-sans-padding>
```

`ValidatePasswordHash` valide sans dérivation/IO. Codec limité à128octets,
six segments exacts, argon2id/v19 uniquement, ordre m,t,p et entiers décimaux
canoniques sans signe/zéro initial/suffixe. Coûts validés avant IDKey, y compris
alignement ; base64 standard strict/canonique, sans padding/CRLF/espaces, longueurs
exactes16/32octets. Autres algorithmes, versions, champs, tailles et débordements
sont refusés avec ErrInvalidHash fixe, sans record partiel ni hash fourni.
La validation n'atteste pas l'origine du hash ou la qualité du secret qui l'a produit.

`VerifyPassword` revalide l'entrée et décode intégralement le record avant tout
calcul. Correspondance = true,nil ; mot de passe valide différent = false,nil ;
entrée/record invalide = false,erreur sûre. Comparaison des32octets via
crypto/subtle.ConstantTimeCompare ; le décodeur et l'ensemble du login ne sont pas
à temps constant. Les paramètres persistés sont utilisés tels que validés143.

Politique d'entrée144 : UTF-8 valide, 15..256points de code, au plus1024octets,
sans troncature/trim/case folding/normalisation Unicode. Espaces et caractères
Unicode conservés, pas de règle de composition. La longueur15 et le comptage par
point de code suivent les recommandations de longueur du
[NIST SP800-63B §3.1.1.2](https://pages.nist.gov/800-63-4/sp800-63b.html#passwordver).
Le choix applicatif conserve les octets : des formes Unicode visuellement identiques
peuvent être distinctes ; NFC n'est pas appliqué. Aucune conformité NIST globale
revendiquée. Liste de mots de passe courants/compromis et dérivés du compte reste
à raccorder lors de l'initialisation146 ; une longueur acceptée ne prouve pas la force.

L'appelant garde son buffer secret inchangé, sans le modifier pendant l'appel ;
aucune garantie d'effacement des copies mémoire ou de confidentialité d'un hash
imprimé par un appelant. Aucune journalisation ici. HashPassword retourne une
chaîne vide sur erreur de validation/lecture de sel ; l'erreur de lecture injectée
est ErrRandom fixe, sans détail de la dépendance. Source d'entropie injectée
uniquement dans un helper privé de test, API publique toujours crypto/rand.
Coûts bornés par appel, pas de deadline/annulation ni limite de concurrence :
admission bornée/essais et politique comptes inconnus restent à la frontière HTTP.

Cinq nouveaux tests144 (neuf tests auth au total) et vet/format/diff Windows passés :
limites UTF-8/points de code/bytes et ownership, vecteur externe et mismatches
casse/espaces/octet final, sel frais public, refus avant lecture du sel/erreurs sûres,
codec strict et valeurs maximales validées sans calcul coûteux. Vecteur synthétique
produit séparément avec argon2-cffi25.1.0/libargon2 : secret "synthetic secret phrase",
sel "0123456789abcdef", ID/v19, m65536,t3,p4,hash32 ; résultat figé dans le test,
pas recalculé avec la même implémentation pour établir l'attendu. Version Go de
l'algorithme confrontée à la version du record. Fuzzer du codec seul (aucun hash) :
5s,2workers,633884exécutions Windows réussies ; campagne bornée, pas preuve exhaustive.
CI Windows étendue à auth ; Linux existant teste/vet/build l'ensemble, dépendance
crypto seule ajoutée sans modification des versions existantes. CLI/SQLite/config/
FileSource inchangés ; publication/CI144 à vérifier après commit dans #34 réutilisée.

## Suite après144 (snapshot)

Lot145 : persistance atomique et lecture bornée du compte local, identité+hash
validés sans vérification de mot de passe lors du chargement, refus sans écrasement,
tests/doc. Aucun stockage créé par144 ; CLI d'initialisation146 puis revue de ce
chantier. Sessions/protections HTTP dans un chantier distinct. Aucun compte
utilisable, serveur, route ou cookie livré ; MIT, AD/OIDC après MVP.

Publication144 effective : 22f1694 dans #34,
[CI37601878573](https://github.com/Coubiac/QueueAtlas/actions/runs/37601878573)
entière réussie/trois jobs/SHA exact ; nouvelle étape Windows auth réussie,
Linux tests/vet/format/smoke/race source-file et builds statiques réussis.

## Lot145 : persistance du compte local

`LocalAccount{Identity, PasswordHash}` valide les contrats143/144 sans dérivation,
mot de passe clair, session ou rôle. Le fichier **local-admin.json** contient
exactement version1 (nombre JSON), username et password_hash (textes). Ce compte
est stocké séparément de SQLite : aucune migration ou dépendance de la base de
journaux pour initialiser l'identité administrative. C'est un choix de persistance
MVP, pas un magasin de comptes externes ou une gestion multi-utilisateurs.

`CreateLocalAccount(directory, account)` exige un répertoire existant, privé et
fiable : aucun parent créé/chmodé. Identité/hash validés avant IO ; toute destination
existante, même invalide/répertoire/lien, est conservée et renvoie ErrAccountExists.
Dans un [os.Root](https://pkg.go.dev/os#Root) ouvert, un temporaire à nom aléatoire
est créé exclusivement en0600, écrit complètement, Sync puis fermé. Publication par
lien dur vers le nom final, sans remplacement, puis retrait du temporaire et Sync
du répertoire sur Linux. Pas de fallback par copie ou rename qui écraserait un compte.
Filesystem sans liens durs : erreur sûre, aucun compte partiellement publié.
Créateurs concurrents : au plus un gagnant ; lecteurs voient absence ou record complet.

`LoadLocalAccount(directory)` refuse fichier non régulier/symlink, document dépassant
4Kio et droits partagés POSIX. Contrôle Lstat puis Stat/identité du fichier ouvert,
lecture de4097octets maximum, validation JSON/contrats et fermeture avant retour.
Taille/mtime contrôlés après lecture ; ce contrôle n'est pas un verrou contre une
modification en place par un propriétaire hostile. JSON UTF-8/un seul objet exact,
champs inconnus/dupliqués/casse différente, version inconnue/flottante/textuelle,
null/types imbriqués/champs absents/hash invalide et documents supplémentaires refusés.
Espaces JSON finaux permis dans la borne. Toute erreur renvoie LocalAccount zéro.

Erreurs fixes sans chemin/identifiant/hash/contenu/erreur brute OS :

| Erreur | Sens pour l'appelant |
| --- | --- |
| ErrAccountNotFound | Répertoire valide, aucun compte publié |
| ErrAccountExists | Destination occupée, aucune tentative de remplacement |
| ErrInvalidAccount | Document ou contrats invalides |
| ErrAccountIO | Répertoire/droits/type/lecture/écriture/publication indisponibles |
| ErrAccountPublished | Compte déjà publié ; nettoyage ou Sync du répertoire échoué, inspecter avant de réessayer |

Une création dont l'identité/hash est invalide conserve les erreurs143/144 et ne
fait aucune IO. Pas de suppression du compte final pour réparer une erreur tardive.
Nettoyage temporaire avant publication au mieux ; un arrêt brutal peut laisser un
temporaire complet privé, ignoré par le lecteur. Pas de purge automatique des
temporaires ni reset/rotation/migration du compte dans cette API.

Linux : répertoire ouvert et fichier sans droits groupe/autres ; fichier créé en0600 sous umask,
répertoire déjà privé (normalement0700). Ouverture finale O_NOFOLLOW|O_NONBLOCK,
pour refuser un symlink et éviter un blocage sur FIFO remplacée. Sync fichier puis
répertoire demandé ; garantie dépend du filesystem/matériel, pas de campagne
coupure électrique effectuée. Windows : liens durs testés sur le filesystem local,
Sync fichier demandé ; pas de Sync répertoire ni validation/modification ACL.
Les ACL restrictives relèvent du déploiement. Autres plateformes suivent le chemin
portable, sans garantie Linux de durabilité ou garde nofollow atomique.

Chemin du répertoire choisi par l'opérateur, pas via Web ; root initial suit les
symlinks du répertoire. Espace de noms/parents/propriétaire doivent rester fiables.
Root borne les opérations sous son répertoire ouvert, sans interdire montages ou
fichiers spéciaux par lui-même ; les contrôles de cette API portent sur le record.
Un hash syntaxiquement valide n'est pas une preuve de sa provenance ou de force.
Aucune authentification/autorisation accordée par la lecture seule.

Cinq nouveaux tests communs145, quatorze auth/vet/format/diff Windows passés :
roundtrip/ownership/format, refus sans création/écrasement/confidentialité, huit
créateurs concurrents/un gagnant/lectures complètes, JSON strict/bornes/read errors,
fichiers corrompus/surdimensionnés inchangés. Deux tests Linux supplémentaires à
exécuter en CI : droits privés/partagés, symlinks valides/pendants et FIFO, sans
remplacement ; seize tests Linux au total. Race auth ajouté au job Linux Go1.26.x.
Erreurs de Sync/nettoyage tardif relues, pas injectées ; aucune résistance à une
coupure physique revendiquée. Aucun changement CLI/config/SQLite/hash/modules.
Publication/CI145 à vérifier après commit dans #34 réutilisée.

Première CI14537605619494 échouée sur cc12df7 : fixtures testing.TempDir avec
droits0755 sous Linux, refus correct du stockage privé. Correction des fixtures
par chmod0700 POSIX explicite, aucun changement de production ; tests/vet Windows
repassés, CI corrective Linux/race à vérifier avant validation du lot.

## Suite après145 (snapshot)

Lot146 : CLI admin create, répertoire explicite existant/username/secret via stdin
borné, contrôle des mots de passe courants/dérivés, hash144 puis création145, codes
et diagnostics sans secret/chemin, tests du binaire/doc. Aucune commande admin,
route login ou session encore disponible. Revue/clôture du compte local ensuite ;
sessions/protections HTTP dans un chantier distinct. MIT, AD/OIDC après MVP.

Validation145 effective : tête corrective19415ba07989db2422f3ce1a51b10d6bf998e653,
[CI37605851032](https://github.com/Coubiac/QueueAtlas/actions/runs/37605851032)
entière réussie/trois jobs/SHA exact ; tests auth Windows14/Linux16, race auth
Linux Go1.26.x réussis. Fixtures POSIX0700 corrigées, production inchangée.
Les attentes145 et sa première CI échouée sont des snapshots historiques terminés.

## Lot146 : initialisation CLI

Commande disponible : `queueatlas admin create --directory <path> --username <name> --password-stdin`.
Syntaxe/ordre uniques, options et valeurs séparées, aucune config implicite ; aides
admin/create sans IO. Le répertoire existant doit remplir le contrat145, avec ACL
Windows administrées séparément. Le chemin et l'identifiant restent littéraux.
Stockage prévalidé avant lecture du secret ; compte existant/corrompu/IO refusé.
Cette vérification ne réserve pas la destination : Create arbitre toujours la
publication atomique sans remplacement et revalide le répertoire.

stdin doit fournir un seul record UTF-8 via pipe ou fichier. EOF requis, lecture
limitée à1027octets, mot de passe144 limité à1024octets/15..256points de code.
Un LF ou CRLF final est le séparateur de transport retiré ; CR/LF internes ou
supplémentaires refusés. Aucun trim/normalisation du secret haché, espaces initiaux/
finaux et Unicode conservés. Pas de mot de passe en argv, variable d'environnement
ou option par la CLI. Entrée fichier character-device/console refusée avant lecture,
pas de prompt interactif avec echo ; pipe local bloqué sans EOF peut attendre,
pas de deadline ajoutée. L'opérateur fournit une entrée fiable et privée.

`ValidateNewPassword(password, identity)` applique les bornes144 et une liste
initiale **finie**, autonome sans réseau :27valeurs complètes,15suffixes du nom du
compte/QueueAtlas, répétitions deux/trois fois avec quatre séparateurs. Casse et
espaces autour ignorés pour la comparaison uniquement ; aucun rejet par substring,
aucune composition imposée. Octets acceptés inchangés pour HashPassword. Liste dans
[password_policy.go](../internal/auth/password_policy.go), test synthétique/public ;
la phrase publique « correct horse battery staple » est refusée comme exemple connu.
Erreur fixe ErrBlockedPassword ; la CLI explique le rejet et propose une valeur
générée différente ou passphrase. Aucun mot de passe fourni recopié.

Cette liste initiale n'est pas un corpus exhaustif de compromissions, ni une preuve
de force. Taille/valeurs à réexaminer avant login/release avec la limitation d'essais.
Le [NIST §3.1.1.2](https://pages.nist.gov/800-63-4/sp800-63b.html#passwordver)
prévoit la comparaison des valeurs complètes, notamment liées au compte/service,
avec une liste adaptée aux essais permis. Aucune conformité NIST globale revendiquée.
La vérification d'un hash144 n'applique pas cette politique de création, afin de ne
désactiver aucun compte si la liste évolue.

En succès : HashPassword avec défaut143 et sel frais, Create145, sortie fixe/code0.
Codes2 pour syntaxe/identité/record/borne/liste refusés ; codes1 pour IO stdin ou
stockage, hash/création, publication tardive et sortie. ErrAccountPublished donne
un diagnostic explicite : compte créé, inspecter avant retry. Échec stdout après
publication garde le compte ; aucune suppression/reset. Erreurs/outputs sans chemin,
identifiant/hash/secret ni erreur brute de dépendance. Buffer secret effacé au mieux
via clear ; copies strings, mémoire Argon2/shell et pagination OS non attestées.

Cinq nouveaux tests CLI,20CLI et15auth/vet/format/diff Windows passés : aides/args
sans lecture, frames EOF/LF/CRLF/UTF-8/Unicode4octets/borne1024 et1027octets lus max,
character-device/erreur reader, compte vérifiable/littéral, seconde création sans
lecture/écrasement, refus sans état/diagnostic privé, compte corrompu conservé,
stdout échoué sans rollback. Test du binaire compilé existant enrichi, build réutilisé :
stdin/codes0/1/2 et hash vérifiable après reopen. Un nouveau test auth couvre valeurs
complètes/casse/espaces/comptes/dérivés et absence de mutation ; Linux17tests attendus
avec deux spécifiques145/race via workflow existant, à vérifier après publication146.
Pas de modification SQLite/config/FileSource/modules/workflow, pas de login/session.

Prochain lot147 : revue/clôture du compte local143–146 dans #34, contrôles ciblés,
évaluation des limites de la liste pour le futur login, corrections utiles/doc puis
CI finale/fusion/main. Sessions/protections HTTP dans un chantier suivant. MIT,
AD/OIDC/Keycloak après MVP.

## Clôture du compte local — lot147

CLI146 publiée sur cfde74d52dfa8fd8a58864e3434e70e67d796c3c,
[CI37608644994](https://github.com/Coubiac/QueueAtlas/actions/runs/37608644994)
entière réussie/trois jobs/SHA exact :20CLI/binaire, auth Windows15/Linux17,
race auth et builds statiques verts. Les attentes146 prépublication sont terminées.

[Relecture147](reviews/m4-local-account.md) favorable à la clôture143–146 après
correction ciblée : un secret constitué uniquement d'espaces ASCII/Unicode est
maintenant refusé par ValidateNewPassword avec ErrBlockedPassword. Secret complet
de comparaison vide bloqué avant hash/stockage ; passphrases avec espaces restent
littérales, ValidatePassword/VerifyPassword inchangés pour les comptes existants.
Tests auth/CLI/binaire enrichis, trois tests ciblés Windows et vet/format/diff passés.
CI finale/revue COMMENT/fusion/main à terminer au commit147.

La liste initiale finie n'est **pas validée comme suffisante pour la release de
login**. Dans le chantier de protections HTTP, évaluer/enrichir un corpus local
documenté/provenance/licence avec valeurs représentatives et essais admis ; aucun
secret envoyé à un service externe. Admission globale, essais/comptes inconnus,
sessions/cookies/CSRF/TLS et routes protégées restent requis selon ADR-006.
Stockage fiable/privé, ACL Windows, limites de durabilité/EOF/mémoire conservés.
Prochain148 : sessions en mémoire bornées, émission/expiration/révocation/tests,
sans login ni transport HTTP. MIT, AD/OIDC/Keycloak après MVP.

Clôture147 effective : #34 fusionnée sur9cd6cec, CI finale37611573008 et
main37611762238 entières réussies/trois jobs/SHA exact ; branche nettoyée.
Preuves dans [relecture du compte](reviews/m4-local-account.md), limites login
conservées. Les [sessions148 en mémoire](local-sessions.md) ajoutent émission/
expiration absolue et inactive/révocation/capacité bornées ; vérifiées Windows,
publication/CI à terminer au commit148, aucune route/login/cookie disponible.

Validation148 effective : bd862cd publié dans #35, CI37615586790 entière/trois jobs/
SHA exact/race auth réussis, revérifiés REST à la reprise149. Le [login149 borné](local-login.md)
raccorde le compte aux sessions après vérification Argon2id, budget partagé et
admission concurrente ; vérifié Windows, publication/CI à terminer au commit149.
Le transport/cookies150, protections transversales/liste adaptée et revue restent
requis avant exposition. Aucune route Web ou option YAML login livrée149.
