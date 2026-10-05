# Revue du chantier d'application des imports

## Lot 80 : préparation d'ingestor et position prouvée

Base main 677a618 après manifest #16 et CI push réussie. Deux nouveaux fichiers
importfile/ingestor.go/tests ; NewIngestor vérifie source/run/content/copie/CP avant
Seek, digest entier pour zéro, ancre canonique et LF avant offset positif. Reader
privé déjà validé conservé ; pas de path original rouvert ni checksum recalculé.
Metadata pointers copiés, état acquitté exposé, pas de normalizer/Sink/manifest.

Trois tests ciblés Windows -count=1, suite/vet/diff réussis. Normal/gzip, vide/reprise0/
positive/EOF/partial/window4096+, original changé, position/getters, dix-neuf refus
et midline, cancel/nilNormalizer/copyclosed/sizechanged couverts. Revue indépendante
terminée sans blocage : trois tests Windows/diff propres, aucun test supplémentaire
nécessaire identifié. Publication/CI à vérifier, audit assisté sans certification externe.
Caller owns Close/exclusivité/bytes stables/répertoire protégé, cancel après Seek
peut déplacer position, aucun cleanup ou transfert implicite. Application future.
