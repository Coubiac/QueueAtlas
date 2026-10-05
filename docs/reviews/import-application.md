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
Propriété/exclusivité jusqu'à ingestion et mono-écrivain requis. Publié e8e5469,
CI 37264777146 success ; pas de Run ou cleanup implémenté par ce lot.

## Lot 83 : Attempt.Run sur un fichier

Deux nouveaux fichiers attempt.go/tests : tentative explicite source/run/path,
deadline obligatoire, exclusivité d'objet et caller mono-écrivain, copie possédée
et cleanup joint. Pending exact réessayé avant lookup/open, copie fermée après
erreur Sink puis revalidation/reprise durable après ACK ; complete reconnu par
status. Failed préparation non interrompue tracé sans compensation de commit
ambigu ; interruption ou reprise input indisponible/changé restent running.
Terminal ID représente opération passée, aucun reopen/ranimation implicite.
Cinq tests ciblés Windows/suite/vet/diff réussis avec SQLite réelle,
revue indépendante sans blocage : cinq tests ciblés Windows passés par auditeur,
code/tests/documentation relus, aucun contrôle supplémentaire nécessaire identifié.
Publié 1e6bc8f, CI 37265252402 success ; audit assisté sans certification externe.
Liste/ordre/nombre/deadline globale encore exclus ; pas de CLI/service.

## Lot 84 : ImportSource liste ordonnée et borne globale

Deux nouveaux fichiers import_source.go/tests : liste explicite, IDs distincts
stables, encodings explicites/pathabs, paramètres copiés, count positif<=MaxFiles
<=1000 et durée positive, validation avant IO. Run séquentiel sous deadline commune,
parent plus court respecté, une copie au plus, premier échec stop, retries par
Attempt et complete sans reopen. Cinq tests ciblés/suite/vet/diff Windows réussis
avec SQLite réelle, deadline globale/parent sans write via lookup bloqué, ordre/
recompression/config/ACKretry/partial/limites/concurrence. Revue sans blocage : cinq
tests ciblés exécutés par auditeur, aucun contrôle supplémentaire nécessaire.
Publié 84f091b, CI 37265578493 success. Dépendances context-aware et mono-écrivain requis ;
pas CLI/corrélation. Audit assisté sans certification externe.

## Lot 85 : synthèse et référence isolée

Checkout géré isolé propre sur 84f091b86c26a9bf046595d1aa74574612bf8f81. Huit
fichiers runtime/tests comparés par auditeur identiques aux versions relues ; coordinateur
confirme les neuf fichiers Go/test du chantier identiques entre root et isolé. Aucun risque
nouveau justifiant rerun. Avis de clôture indépendant sans blocage : réutiliser
tests/revues ciblés Windows et suites/vet/CI publiées, ready/fusion après CI finale
exacte du commit documentaire85. Aucun changement runtime à cette clôture.
Limites : mono-écrivain et parents protégés, deadline coopérative, syscall/cleanup
potentiellement bloquant, résidu cleanup refusé signalé. Provenance/déduplication
source-scopées ; overlap avec FileSource incertain, pas de projection canonique
avant M3. Bibliothèque Go sans CLI/service. Audit assisté sans certification externe.
Clôture publiée 5b28ee7, CI37265820369 success ; #17 ready/fusion sur main fcb6ad9,
CI push37265917012 success vérifiée REST. Les contrôles ci-dessus ont précédé la fusion.
