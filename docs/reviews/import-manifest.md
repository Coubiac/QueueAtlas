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
Publié `8797183d65b6a2cfd79f85fc94df913c7d764ccd`,
[CI réussie](https://github.com/Coubiac/mailtrace/actions/runs/37246530486).
Audit assisté, sans certification externe.

## Lot 76 : migration v3 et lecture source-scopée

Base `8797183`, quatre tests nouveaux et suite/vet/diff Windows réussis.
Rebuild transactionnel import_runs conserve les huit colonnes v1 et chaque valeur
legacy ; source/origine/partial restent NULL. V1/V2 SQL inchangés. Contraintes des
lignes associées, FK composite et source, timestamps terminaux, offset/complete,
digest canonique, index et historique/version ajoutés dans la même transaction.

ImportRun lecteur exact source/ID, jointure unique ; legacy/étranger absent, zéro
distinct, kind import + originID/fingerprint/content contrôlés, aucune écriture.
Migration v1/v2, refus trigger3 et rollback DDL/version/history, réouverture,
statuts avec/sans contenu, source opaque NUL/caractères SQL, absence/cancel,
treize contraintes et corruption d'origine couverts. Revue indépendante interrompue
par quota, reprise ensuite : SHA canonique suivi NUL échappait au CHECK TEXT/GLOB.
Régression échouante, première correction BLOBseul laissait un NUL intérieur passer ;
seconde régression échouante. Correction finale exige les deux longueurs TEXT/BLOB
64 et GLOB canonique ; treize cas puis suite/vet/diff Windows repassent. Le lecteur
Go refusait déjà ces valeurs, sans faux succès de lookup ; contrainte SQL durcie.
Revue finale terminée sans autre blocage : auditeur quatre tests nouveaux Windows,
deux régressions NUL ciblées et diff propres. Null branches/FK/offsets/statuts
cohérents ; SQL v1/v2 inchangés. ID run global dans la base précisé dans le contrat.
Publication/CI à vérifier. Manifest-write/application et preuve EOF restent futurs.
