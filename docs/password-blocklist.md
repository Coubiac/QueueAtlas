# Corpus local d'enrôlement — lot 152

`ValidateNewPassword` refuse désormais les valeurs complètes du corpus ci-dessous,
en plus des 27 exemples locaux et dérivés du nom du compte/service du lot 146.
La CLI applique ce contrôle avant Argon2id et publication du compte. Le rejet
conserve `ErrBlockedPassword`, le diagnostic fixe et le code de sortie 2.

## Provenance et licence

Source publique : [SecLists](https://github.com/danielmiessler/SecLists), fichier
`Passwords/Common-Credentials/xato-net-10-million-passwords-1000000.txt`, à la
révision figée `47cd752f4323f703e304104173633ee31462b9b3`.
[Lien immuable du fichier](https://raw.githubusercontent.com/danielmiessler/SecLists/47cd752f4323f703e304104173633ee31462b9b3/Passwords/Common-Credentials/xato-net-10-million-passwords-1000000.txt).

Le [README amont](https://github.com/danielmiessler/SecLists/blob/47cd752f4323f703e304104173633ee31462b9b3/Passwords/Common-Credentials/README.md)
attribue la famille Xato au jeu publié en 2015 par Xato/Mark Burnett ; la liste
complète est annoncée comme triée par fréquence décroissante. Nous utilisons
le sous-ensemble amont d'un million de lignes, sans recalculer ses fréquences.
Ce sont des données publiques de refus embarquées dans le produit, pas des
comptes, journaux ou identifiants de test provenant d'utilisateurs QueueAtlas.
Les fixtures de tests restent des constructions synthétiques.

SecLists publie ce dépôt sous [MIT](https://github.com/danielmiessler/SecLists/blob/47cd752f4323f703e304104173633ee31462b9b3/LICENSE),
copyright (c) 2018 Daniel Miessler. La licence complète est conservée dans
`internal/auth/password_blocklist.LICENSE`, référencée par
[THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md). Les futures distributions du
binaire doivent inclure ces notices ; l'assemblage des paquets reste M5.
QueueAtlas conserve MIT. Aucune dépendance Go supplémentaire.

## Import reproductible

Le générateur de maintenance `internal/auth/blocklistgen` lit un fichier local ;
il ne télécharge rien. Il exige la taille et l'empreinte exactes de la source,
puis un million de lignes. Il sépare les LF, retire un éventuel CR final par ligne,
et garde seulement les valeurs acceptées par `ValidatePassword` : UTF-8 valide,
15 à 256 points de code et au plus 1 024 octets, avant normalisation de comparaison.

Il applique exactement `strings.ToLower(strings.TrimSpace(value))` de Go,
calcule SHA-256 sur les octets UTF-8 obtenus, déduplique et trie les empreintes
hexadécimales minuscules. Une ligne de sortie contient 64 caractères puis LF.
Les valeurs sources en clair ne sont pas commitées. Les empreintes publiques
ne rendent pas ces valeurs secrètes et ne servent jamais au stockage du compte.
Le hash du compte demeure Argon2id salé ; SHA-256 sert seulement à cette liste.

| Mesure figée | Valeur |
| --- | --- |
| Source | 1 000 000 lignes, 8 557 632 octets |
| SHA-256 source | `424a3e03a17df0a2bc2b3ca749d81b04e79d59cb7aeec8876a5a3f308d0caf51` |
| Valeurs source dans les bornes | 10 908 |
| Empreintes uniques après comparaison normalisée | 10 898 |
| Sortie embarquée | 708 370 octets |
| SHA-256 sortie | `a5b8b74b66c0ed54d90098285cb0dbe02217ae41d7c91f0c57cf4a761e14521a` |

Depuis la racine du dépôt, avec Go 1.26 et la source téléchargée séparément depuis
le lien immuable, générer un fichier temporaire et comparer son empreinte avant
de remplacer la copie revue :

```text
go run ./internal/auth/blocklistgen <source-locale> <sortie-temporaire>
go test ./internal/auth -run TestPasswordCorpusIntegrityAndEnrollment -count=1
```

Le test d'intégrité fige taille, cardinalité et SHA-256 du corpus **embarqué** et
vérifie le format, l'ordre strict et l'unicité de toutes les entrées. Il ne
télécharge pas la source. Une modification volontaire nécessite une revue du diff,
de la provenance/licence, de la transformation et des nouvelles empreintes,
puis mise à jour de la documentation et des références du générateur/test.
Deux générations locales du lot 152 ont produit la même empreinte.

## Comparaison et limites

La donnée immuable est intégrée par `go:embed` ; aucun fichier ni réseau n'est lu
lors de l'enrôlement. Après validation des bornes, la valeur complète normalisée
est hashée une fois et recherchée par dichotomie dans les lignes de taille fixe.
Le volume est fixe, environ 692 Kio embarqués ; aucun chargement de table/map ou
scan d'un million de valeurs à chaque tentative. Les comparaisons ne sont pas
revendiquées constantes en temps. Une collision SHA-256 pourrait refuser une
valeur supplémentaire ; elle ne permettrait pas d'accepter une entrée connue.

La normalisation sert au refus : casse et espaces autour ne contournent pas une
entrée connue. Les octets d'une valeur acceptée restent littéraux pour le hash
et le login. Une passphrase contenant un mot listé n'est pas refusée pour cette
seule raison. Aucune recherche de sous-chaîne, règle de composition ou distance
de similarité supplémentaire. Les contrôles contextuels existants sont conservés.

Choix de couverture : le minimum de 15 caractères écarte déjà la majorité des
secrets courts de la source ; la sélection figée ajoute 10 898 valeurs longues
connues, au-delà des 27 exemples initiaux. Le login a par défaut un budget global
de 5 admissions par minute et un seul calcul simultané. Ce budget se renouvelle,
peut doubler autour d'une frontière et redémarre avec le processus : il n'arrête
pas définitivement les guesses. La liste reste historique et finie, ne couvre
pas toutes les compromissions, tous les dérivés ni les attaques hors ligne.
L'acceptation ne prouve pas la force d'un secret.

Le [NIST §3.1.1.2 et annexe A](https://pages.nist.gov/800-63-4/sp800-63b.html#passwordver)
préconise une comparaison complète, une liste adaptée aux essais permis et un
guide après rejet. Cette source motive le choix ; aucune conformité NIST globale
ni preuve statistique de couverture n'est revendiquée. Réexaminer le corpus avant
release/pilote, lors d'une modification du budget, des bornes ou des providers,
et si une information de compromission concrète l'exige. Pas d'actualisation
automatique ni de transmission du secret à un fournisseur.

Après rejet, créer une nouvelle valeur avec un gestionnaire de mots de passe ou
une longue passphrase originale ; éviter une simple modification d'un exemple
public. Les comptes déjà provisionnés restent vérifiés par leur hash littéral :
ni `VerifyPassword` ni `LocalLogin` ne réapplique la liste d'enrôlement. Aucun
reset/expiration/changement automatique du compte n'est ajouté.

## Vérifications du lot

Deux nouveaux tests auth : intégrité du corpus, motifs synthétiques de chiffres/
clavier, casse/espaces et absence de mutation, refus complet sans sous-chaînes ;
hash Argon2id et login réels d'un ancien secret désormais refusé à la création,
avec refus de sa variante normalisée au login. Tests CLI existants enrichis :
rejets sans compte/secret dans les diagnostics et code 2 du binaire compilé.
Un test du générateur refuse une source synthétique trop courte ou de taille
exacte mais avec une mauvaise empreinte, sans modifier la sortie précédente.
Tests auth/CLI, vet, format et diff Windows passés ; CI Linux/race/builds à
vérifier après publication. Aucun test de données personnelles ou de fuite réelle.

Le lot 153 reste la revue/clôture des sessions/login/protections HTTP et de ce
corpus dans #35. Aucun serveur, API de messages ou interface Web livré dans 152.
