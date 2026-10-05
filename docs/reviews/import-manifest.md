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
Publié `91bfdaa717bc6eb30a8b222eaa7fe10cdd6ef097`,
[CI réussie](https://github.com/Coubiac/mailtrace/actions/runs/37261183931).
Manifest-write/application et preuve EOF restent futurs à cette étape historique.

## Lot 77 : trace transactionnelle de préparation

Portée réduite avant contenu connu, progression reportée au 78. ImportChange
Before/Target dans Batch, running sans contenu créé ou running->failed, identité
et datation immuables. Target exact déjà durable accepté, autres conflits/legacy/
étrangers/refus terminal sans adoption ni retry. Source et trace dans le même Tx.
Reader commun DB/Tx ; timestamps int64 sans wrapping. Aucun changement SQL/FileSource.

Quatre tests TestImportPreparation* -count=1, suite/vet/diff Windows réussis :
ACK perdu/retry/reopen, global ID/legacy/foreign/stale, trigger insert/update et
source rollback, cancel et dix-huit refus contenus/provenance/dates/immutabilité.
Revue indépendante terminée sans blocage : quatre tests ciblés Windows/diff propres,
contrat/rollback/immutabilité/retry relus, aucun edit. Publication/CI à vérifier. Mono-écrivain et ownership
du batch au caller, audit assisté sans certification externe.

## Lot 78 : association et progression atomiques

Contenu attaché une fois ou création préparée au checkpoint persisté, puis progression
records contigus/CP/manifest dans le même Tx. Source/ID/fingerprint/noPhysical et
contenu immuables, metadata concordante entre tentatives ; expectedstate/CPavant
contrôlés avant writes, CPaprès égal à Target et anchor proposé si présent.
Complete taille entière/noPartial, failed au dernier offset acquitté ; Sink ne
prouve pas EOF/CRC/anchor/fichier.

Cinq tests initiaux Windows/suite/vet/diff réussis : deux lignes identiques à des
offsets distincts, retry/reopen/réimport, finaltrigger rollback records/events/CP/source,
treize divergences et mauvaisCPavant/anchorégaloffsetdifférent, vide/partial, sept
refus d'association. Anciens Preparation/Run repassés après extension.

Revue indépendante : défaut ACK trouvé, Target déjà identique acquittait une ancre
différente au même offset ignorée par upsert. Régression deux sous-cas zéro/cinq
échouante, puis corrigée : CP durable au moins au Target, à égaloffset comparer
l'anchor proposé même lors des deux retours anticipés. Ancien retry exact après
progression partagée d'une autre tentative reste accepté, sans duplication.
Six tests ciblés/régression/suite/vet/diff Windows repassent. Revue finale sans blocage,
auditeur régression corrigée exécutée Windows et diff propre. Test supplémentaire
réutilisation contenu : taille/partial changés ou fingerprint/device stockés
incohérents refusés, quatre sous-cas Windows passent ; runtime inchangé après revue.
Publication/CI à vérifier. Mono-écrivain/preuves/batch possédé par caller requis.
