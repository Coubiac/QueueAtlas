# Revue du chantier de manifest d'import

## Lot 75 : identité durable de contenu

5 octobre 2026, base main `f8e58e30a8e85bac6d67fe1c30e5ce64ff2e4c7a`.
ImportOriginID et ses deux tests relus par coordinateur et auditeur indépendant.
Aucun blocage concret. Framing non ambigu avant SHA : domaine terminé NUL,
longueur source uint64 big-endian, octets source exacts, digest32 fixe canonique.
Source sans trim/casefold/transcodage/validation UTF-8, digest64hex minuscule,
erreur fixe et résultat vide sur refus. Golden .NET calculé/recalculé indépendamment.

Coordinateur : deux TestImportOriginID* -count=1, suite/vet/diff Windows réussis.
Auditeur : deux tests ciblés -count=1 -v et diff propres, golden identique.
Stabilité, différentes sources/espaces/casse/NUL/Unicode, différent digest et
refus de représentation non canonique couverts. Pas de changement SQL/FileSource.

ADR-011 précise le format durable et contrat du manifest suivant. L'identité
n'est pas une preuve de validation/EOF ni de chevauchement avec le suivi live ;
elle ne déduplique aucune ligne. SQL, manifest et ingestion restent futurs.
Publication/CI à vérifier. Audit assisté, sans certification externe.
