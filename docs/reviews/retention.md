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
Publication/CI116 à vérifier. Pas de
mesure de charge/Linux local ni comportement de suppression présenté comme livré.
Lot117 protège le rejeu après purge ; suppression/invalidation118 et clôture119
séparées, conformément au découpage court demandé.
