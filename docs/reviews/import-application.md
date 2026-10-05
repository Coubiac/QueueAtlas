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
nécessaire identifié. Publié 357bde7, CI 37263556572 success ; audit assisté sans certification externe.
Caller owns Close/exclusivité/bytes stables/répertoire protégé, cancel après Seek
peut déplacer position, aucun cleanup ou transfert implicite. Application future.

## Lot 81 : CommitNext et fin de copie

Un record, CP et progression du run dans le même batch ; record staged avant
CaptureAnchor et normalizer, pending exact conservé sur erreur/cancel/ACK perdu.
EOF commit terminal sans records/CP ; complete si copie entière sans partial,
failed au dernier LF sinon. Erreur Sink ambiguë ne déclenche aucune compensation.
Quatre tests ciblés Windows/suite/vet/diff réussis, dont SQLite réelle/reopen/doublons,
plain/gzip/EOF/partial/oversize/cancel et proof-failure. Revue runtime terminée sans
blocage ; trois tests ciblés exécutés par auditeur. Nuance signalée : sentinelle EOF
peut venir du Sink sans ACK ; qualification RunState terminal ajoutée au contrat et
régression douze cas passée puis exécutée par auditeur. Dernier delta relu sans
blocage ; publié 2a80042, CI 37264301733 success. Audit assisté, sans certification externe.
Orchestration/Run/cleanup global exclus du périmètre. Propriété exclusive, copie
privée immuable, erreurs de lecture non compensées, pas de retry interne.

## Lot 82 : Binding de contenu au checkpoint partagé prouvé

Deux nouveaux fichiers binding.go/tests : PrepareBinding lit CP scope source et
ID dérivé du contenu, NewIngestor prouve copie/CP avant association. Nouveau zéro
seulement si run non préparé et CP absent, run associé exige son offset exact.
Binding.Commit garde batch exact et rend ingestor après ACK ; resume déjà associé
sans write, aucun parser ou propriété transférée. Trois tests ciblés Windows avec
SQLite réelle réussis : ACK retry, rename/recompression/CP partagé, scope source,
refus dix cas/state-error/cancel. Suite/vet/diff Windows réussis, revue indépendante
sans blocage : trois tests ciblés/diff réussis, aucun test supplémentaire nécessaire.
Propriété/exclusivité jusqu'à ingestion et mono-écrivain requis. Publication/CI82
à vérifier ; pas de Run ou cleanup implémenté par ce lot.
