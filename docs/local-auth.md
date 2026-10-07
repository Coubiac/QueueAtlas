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

## Suite après145

Lot146 : CLI admin create, répertoire explicite existant/username/secret via stdin
borné, contrôle des mots de passe courants/dérivés, hash144 puis création145, codes
et diagnostics sans secret/chemin, tests du binaire/doc. Aucune commande admin,
route login ou session encore disponible. Revue/clôture du compte local ensuite ;
sessions/protections HTTP dans un chantier distinct. MIT, AD/OIDC après MVP.
