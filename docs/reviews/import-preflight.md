# Revue du chantier de prévalidation d'import

## Lot 69 : contenu normal

5 octobre 2026, base main `df7e8a3076d27da4c28c3a8aa6c48e2cb7b4aa30`.
Coordinateur et auditeur indépendant : content.go/tests et ADR-011 relus, aucun
blocage concret ou couverture supplémentaire nécessaire identifié. Nouveau package
importfile, pas de changement FileSource, stockage, dépendance ou workflow.

InspectPlain calcule taille/SHA-256 du contenu entier, séparateurs/suffixe inclus,
sans déduplication de lignes, seek, parser, Sink ou manifest. Mémoire fixe 32Kio,
limite+1 maximum au niveau Reader ; contrôle avant addition pour éviter overflow.
Succès EOF exact seulement, y compris données+EOF ; EOF joint à une erreur échoue.
Erreurs/cancel sans métadonnées partielles, n invalides et lectures vides bornés.
TrailingPartial exige une décision de l'importeur, ne signifie pas complete.

Coordinateur/auditeur : trois TestInspectPlain* -count=1 Windows réussis. Cas vide,
LF/CRLF/partiel, lectures courtes/données+EOF, limite exacte/excès/read borné, maximum
int64, entrée invalide, erreur/cancel/deadline/no-progress/EOF joint couverts.
Coordinateur : suite go test ./..., go vet ./... et diff propres. Durcissement avant
revue : abandon du succès errors.Is(EOF) et régression EOF joint. CI à publication.
[CI du lot 69 réussie](https://github.com/Coubiac/mailtrace/actions/runs/37243851152)
sur `e5541f73d3738bdb5f48ded5c332f5193fb5d76c`, PR #14.

Limites : deadline et fichier régulier à fournir par appelant, syscall bloquant
non interruptible, éventuelle prélecture interne du Reader hors budget du scanner.
Pas de fermeture/fichier détenu/gzip/importeur, identité/provenance ou manifest.
Pas de preuve de snapshot entre inspection et ingestion. Audit assisté par agents,
sans certification externe. Gzip/ratio/checksum au lot distinct suivant.

## Lot 70 : gzip et budgets indépendants

5 octobre 2026, base `e5541f73d3738bdb5f48ded5c332f5193fb5d76c`.
Coordinateur et auditeur indépendant : gzip.go/tests et ADR-011 relus, aucun blocage
concret ni test supplémentaire nécessaire identifié. Pas de changement du scanner
normal, du stockage, des dépendances ou du workflow. Bibliothèque standard seulement.

InspectGzip conserve multistream : chaque CRC/taille et EOF final requis, hash du
contenu décompressé seul. Budgets compressé (headers inclus) et décompressé séparés,
limites positives, un octet témoin maximum en excès, compte avant addition. Ratio
division/reste sans overflow sur output/input consommé, prélecture comprise ; un
préfixe excessif n'est pas excusé par un membre ultérieur. Close/cancel causes jointes,
metadata zéro sur erreur ; input reste au caller, aucun parser/ingestion/manifest.

Coordinateur : quatre tests gzip et trois normal -count=1 Windows, suite/vet/diff
réussis. Auditeur : quatre TestInspectGzip* -count=1 -v réussis, sous-cas corruption,
CRC/taille/header/troncature, second membre corrompu, trailing junk/EOF joint,
normal/recompression/multimembre, limites exactes/excès/ratio, erreurs/ctx/n invalides.
Défaut concret avant correctif : Reader(0,nil) bloquait NewReader/io.ReadFull, test
timeout10s a échoué. CompressedReader borne maintenant 100 lectures vides : test
réussi après correctif, ancien processus bloqué arrêté via PID/commande vérifiés.
Source standard Go gunzip.go consultée pour EOF/CRC/multistream et rôle de Close.
Exécution Linux à vérifier en CI de publication.

Limites : ratio de consommation, prélectures internes distinctes des Reader
comptés, deadline du caller/Read bloquant non interruptible, pas de snapshot.
Gzip vide valide distinct d'une entrée compressée vide sans header. Suffixe partiel
signalé, ni importeur ni manifest livrés. Synthèse/fusion après CI exacte verte.
