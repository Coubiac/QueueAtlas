# Relecture et clôture du compte local — lot147

Périmètre : [PR #34](https://github.com/Coubiac/QueueAtlas/pull/34), lots143–146,
tête relue `cfde74d52dfa8fd8a58864e3434e70e67d796c3c`, base main14219843d6.
Résultat attendu147 : revue, correction ciblée, bilan, CI finale, revue COMMENT
assistée sur tête finale puis fusion/CI main/nettoyage. Cette relecture n'est pas
une approbation humaine indépendante. M4 reste ouvert, aucun serveur/login livré.

## Contrats confrontés au code et aux preuves

| Point | Résultat | Preuve acquise |
| --- | --- | --- |
| Identité | ASCII1..64, sensible à la casse, pas de normalisation/default/lookup/rôle | LocalIdentity, tests143 et refus CLI146 |
| Coûts | Argon2id64Mio/3/4, limites64..256Mio/3..6/1..4/alignement, validation avant dérivation | Parameters, tests143/144 ; autres profils refusés sans allocation |
| Hash | crypto/rand sel16, IDKey32, v19 ; hash vide en erreur, comparaison32octets constante | Vecteur indépendant libargon2/argon2-cffi25.1.0 figé, sels publics frais, refus avant entropie et mismatch144 |
| Codec |128octets, entiers/coûts/base64 canoniques, tailles16/32, record zéro en erreur | Tables hostiles144, fuzz codec5s/633884exécutions Windows acquis144 sans dérivation coûteuse |
| Persistance | JSON version1/3champs exacts/un objet UTF-8,4096octets max, validation sans hash | Tests145 stricts/bornes/ownership et corruption préservée |
| Publication | Répertoire fiable existant, temporaire exclusif0600, Sync/Close puis lien dur sans remplacement | Test145 huit créateurs/un gagnant/lectures complètes, race auth CI146 |
| Lecture Linux | Lstat/Stat/SameFile/type/droits et taille/mtime, nofollow/nonblock | Tests Linux145 droits partagés/liens/FIFO exécutés CI145 corrective/146 |
| CLI | stdin borné1027octets/EOF, transport LF/CRLF, secret littéral, pas de console/option secret | Tests146 frames/Unicode4octets/limites/device/reader, binaire réel codes0/1/2/hash vérifiable |
| Refus/effets | Préflight stockage avant stdin, final intact si occupé/corrompu ou stdout échoué, diagnostics fixes | Tests146 sans lecture/écrasement/état/disclosure et branches privées relues |
| Politique de création | Valeurs complètes comparées sans casse/espaces autour ;27entrées/suffixes compte/service/répétitions | Test146 complet/littéral ; correction147 du cas constitué uniquement d'espaces |
| Intégration | Une dépendance officielle x/crypto v0.57.0 ajoutée, versions existantes inchangées | Diff go.mod/sum, CI tests/vet/build ; gates Windows auth et Linux race ajoutés143–146 |

## Correction147 et contrôles ciblés

Défaut concret : quinze espaces satisfaisaient ValidatePassword, puis la valeur
de comparaison vide n'était pas dans la liste d'enrôlement. Même problème pour
les espaces Unicode TrimSpace. ValidateNewPassword refuse maintenant la valeur
de comparaison vide avec ErrBlockedPassword, avant hash/création. Ce refus porte
sur le secret complet ; les espaces dans une passphrase restent acceptés et
hachés littéralement. ValidatePassword/VerifyPassword restent inchangés : aucune
désactivation implicite d'un compte déjà provisionné.

Tests existants enrichis, pas de test supplémentaire ni de nouveau sous-système :
espaces ASCII15/256, NBSP/EM SPACE15, absence de mutation et séparation des bornes ;
CLI sans état en erreur et code2 réel du binaire. Trois tests ciblés Windows passés :
TestNewPasswordPolicyCompleteValuesAndLiteralBytes,
TestAdminRefusalsLeaveNoAccountAndNoPrivateDiagnostics,
TestLinkedBinaryVersionAndProcessExitCodes. Vet packages auth/CLI, format/diff passés.
Autres tests/fuzz/mesures acquis réutilisés ; pas de rerun local des fondations
sans risque nouveau. CI finale couvrira tout et race auth après publication147.

## Liste de mots de passe et conditions avant login/release

La liste initiale finie est un contrôle local d'enrôlement, pas un corpus de
compromissions maintenu ou une preuve de force. La revue **ne la déclare pas
suffisante pour une release de login**. Avant cette livraison, évaluer/enrichir
les valeurs courantes/compromises et dérivées selon un corpus local documenté,
sa provenance/licence et les essais admis ; tests de valeurs représentatives et
guide opérateur à fournir dans le chantier de protections HTTP. Garder la
vérification des comptes existants indépendante de la politique d'enrôlement.
Pas de lookup réseau/secret transmis à un fournisseur dans ce chantier.

Le [NIST §3.1.1.2](https://pages.nist.gov/800-63-4/sp800-63b.html#passwordver)
prévoit une comparaison complète avec une liste adaptée aux tentatives permises
et des valeurs liées au compte/service. Cette référence ne valide pas les27valeurs
choisies ici. Aucune conformité NIST globale ou résistance au phishing revendiquée.

La livraison de login exige aussi admission/mémoire globale bornées et limitation
d'essais, traitement des comptes inconnus sans énumération, sessions opaques avec
expiration/révocation/capacité bornées, cookies appropriés/TLS, CSRF et routes de
données protégées, selon ADR-006. Le compte local initialisable et les primitives
de hash ne satisfont pas à eux seuls ces critères de M4.

## Limites du stockage et du transport conservées

os.Root initial suit les liens du répertoire ; propriétaire/espace de noms/ancêtres
doivent rester fiables. Les droits POSIX ne vérifient pas l'identité du propriétaire.
Sans verrou contre écritures hostiles en place ; montages/device non interdits par
Root seul. Windows ACL à configurer, aucune attestation automatique ou Sync
répertoire portable ; Linux demande Sync fichier/répertoire, pas de test de coupure
physique/matériel. Temporaire orphelin possible, ignoré sans purge/reset/migration.
Erreurs tardives nettoyage/Sync relues mais pas injectées, final conservé.

stdin opérateur fiable requis, EOF peut attendre sans deadline ; CR/LF internes
non transportables via cette CLI. Buffer clear au mieux, copies string/Argon2/
shell/pagination non garanties. Exemple interactif Bash documenté, non exécuté
sous Windows. Sauvegarde du fichier de compte/ACL et restauration restent M5.
MIT conservée, AD/OIDC/Keycloak après MVP, namespace provider/subject séparé.

## État avant publication147

[CI14637608644994](https://github.com/Coubiac/QueueAtlas/actions/runs/37608644994)
entière réussie/trois jobs Windows/Linux Go1.26.x/stable sur cfde74d exact revérifiés
REST à la reprise147.20CLI, auth Windows15/Linux17, race auth et builds statiques
amd64/arm64 réussis ; preuves144/145 reprises sans refaire vecteur/fuzz/concurrence.
Fetch effectué, checkout propre/local/origin/PR identiques, base main142 inchangée,
PR ouverte/brouillon/mergeable/clean, aucune revue/thread en attente.

Relecture assistée favorable à la clôture **du compte/CLI143–146**, après correction
147 et CI finale verte. Publication/CI/revue COMMENT/ready/fusion/main restent à
terminer au commit ; consigner preuves effectives dans PR puis reprise148.
Prochain148 : sessions en mémoire bornées, émission/expiration/révocation/tests,
sans transport HTTP ni login. API/Web/diagnostic élargi ensuite. M4 reste6–14lots
après clôture147, M5 10–18, total16–32, estimation incertaine.
