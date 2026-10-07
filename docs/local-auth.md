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

## Suite et limites

Lot144 : hachage/vérification Argon2id avec bibliothèque maintenue, sel aléatoire,
codec strict/borné des hashes et comparaison constante, tests/vecteurs et doc.
Politique de mot de passe à définir à cette étape avant toute création. Aucun
format de hash ni paramètre YAML d'auth accepté au143, aucune dépendance ajoutée.
Persistance atomique du compte, CLI d'initialisation, sessions/révocation,
protections HTTP/CSRF/essais et revue dans des lots suivants. Aucun serveur,
cookie, route login ou accès distant n'est activé. MIT conservée ; AD/OIDC après MVP.
