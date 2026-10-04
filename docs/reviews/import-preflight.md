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

Limites : deadline et fichier régulier à fournir par appelant, syscall bloquant
non interruptible, éventuelle prélecture interne du Reader hors budget du scanner.
Pas de fermeture/fichier détenu/gzip/importeur, identité/provenance ou manifest.
Pas de preuve de snapshot entre inspection et ingestion. Audit assisté par agents,
sans certification externe. Gzip/ratio/checksum au lot distinct suivant.
