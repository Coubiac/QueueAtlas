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
réussis, root/Git inchangés. Revue documentaire finale favorable. Publication et CI117
restent à vérifier. Métadonnées sans expirationAPI et digest non anonymisant/non
clé documentés ; aucune qualification finorigine, purge ou sécurité d'effacement
présentée comme livrée. Le lot118 demeure suppression/invalidation transactionnelle.
