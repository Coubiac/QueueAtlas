# Revue du chantier rétention

## Lot116 : preview readonly

Résultat attendu : aperçu borné et cohérent, checkpoint exact source/origine, aucune
suppression ou mutation implicite. PreviewRetention développé en un SELECT paramétré,
cutoff événementUTC exclusif/limit1..256, refs/dates/qualités+More. Positions non
couvertes/ancrevide et datesNULL exclues ; pas de preuve de fin d'origine/ACK pending.

Quatre tests Windows pass, vet/diff pass : scope/date/limite/More/repeat/UTCqualité,
query_onlyON/inventaires24/manifeste16faits/CurrentProjection et CP réelfound inchangés ;
CPmissing/end-1/anchorvide/covered et cutoffégal/+1ns ; requêtes invalides/cancel/
instanceSQLlittérale ; qualitémalforméeaprèslignevalide et conversionCPoffset privées
refusées avec erreurfixe/sortiezéro. Nom d'origine de fixture corrigé pour vérifier
un CP réellement présent, aucun défaut runtime. Schéma/writer/ingestion inchangés.

Revue indépendante favorable, quatre tests via overlay Windows isolé pass, aucun
root/Git modifié ou fondations relancées. Revue documentaire finale favorable ; mention obsolète de clôture115 corrigée.
Publié50047e670984b41840d2e879fec56ef8f9a7bcec dans PR #27 ;
CI37498495532 entièrement réussie, vérifiée REST sur la tête exacte. Pas de
mesure de charge/Linux local ni comportement de suppression présenté comme livré.
Lot117 protège le rejeu après purge ; suppression/invalidation118 et clôture119
séparées, conformément au découpage court demandé.

## Lot117 : garde de provenance persistant

Résultat attendu : futurs faits purgés reconnus par identité physique/empreinte,
sans resurrection ni ACK aveugle. V7 table vide initiale, PKsource/origin/start,
end/digest32 types contraints, FKorigin, UPDATEimmutable. Digest versionné raw/error
encadrés par longueurs ; helper interne avant DELETE, CP exact couvert/ancre nonvide
et conversions validés. Commit garde avant INSERT, retry identique skip ; collision
fixe ou state malformé fixe et rollback complet. Aucun DELETE/API purge117.

Six tests nouveaux Windows réussis : reopen/retries/observationsReadAt ignorés et
origines distinctes ; raw/error/end collisions et rollback du premier record,
source/CP ; checkpoint absent/incomplet/ancrevide/type privé, remember rollback,
contraintes/immutable ; complete import perduACK/retry/collision ; digestmalformé
et cancel ; migrationv6 préserve projection/facts/CP/reopen, injectionhistory7
rollback DDL/version/history. Suite SQLite et vet/diff Windows réussis. Fixtures
legacy créent temporairement les tables nécessaires au writer courant puis retirent
celles-ci ; assertions de version courante suivent schemaVersion, newer8 refusé.
Smoke benchmark migration v5→current1x (1k/10k) réussi après mise à jour du reset,
sans refaire les mesures115 ni les présenter comme mesure de performance117.

Revue indépendante code/tests favorable, six tests via overlay Windows isolé
réussis, root/Git inchangés. Revue documentaire finale favorable. Publié d82ae70c7d2ee4be603b98b164afa8c8e9ac4861 ; CI37499111912
entièrement réussie vérifiée REST sur tête exacte. Métadonnées sans expirationAPI et digest non anonymisant/non
clé documentés ; aucune qualification finorigine, purge ou sécurité d'effacement
présentée comme livrée. Le lot118 demeure suppression/invalidation transactionnelle.

## Lot118 : suppression/invalidation sous réservation writer

PurgeRetention développé : reselect transactionnel (pas preview token), instance/
cutoff/limit116 + origine file retired ou toutes tentatives import complete,
finalCP/taille/fingerprint cohérents. Markers117 avant invalidation de tous les
manifests référents/currents puis DELETEraw→events/domaines cascade. CP/imports/
origins/scopes intacts, résultat Deleted/InvalidatedRevisions/More après commit,
zero result sur toute erreur. Bornefaits256 sans borne indépendante de révisions,
SQL/WAL/durée ; pas preuve de couverture ou effacement sécurisé.

Cinq nouveaux tests Windows réussis et suite SQLite/vet/diff réussis : 24faits,
deux lots2+2, trois révisions concernées invalidées (dont historique explicitement
seedée et scope composite), foreignscope intact/reopen, non datés/horscutoff/foreign
restent ; unknown/following exclus, preview périmé revalidé, cutoff exclusif et retry
après vraie purge ; running import/sharedrunning bloquent, touscomplete éligibles,
deux manifests/CP conservés ; trigger sur dernierDELETE force rollback marker/
invalidation/facts puis retry ; requête invalide257/malformedqualityfixe/cancelzéro.
La fixture initiale supposait qu'InstallProjection gardait l'ancienne révision ;
il la remplace déjà. Fixture corrigée par seed d'une révision historique autorisée
par le schéma, aucune correction runtime associée. Pas de changement116/117.

Revue indépendante code favorable, cinq tests overlay Windows isolé pass, root/Git
inchangés. Revue documentaire finale favorable ; action obsolète117 corrigée.
Publication et CI118 en attente. Intégration WAL,
recherche/reconstruction et clôture119 restent à vérifier ; aucun CLI/service livré.
