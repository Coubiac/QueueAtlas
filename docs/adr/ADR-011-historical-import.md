# ADR-011 — import historique borné et identité de contenu

Statut : prévalidation normale/gzip lots 69–71 fusionnée ; copie vers writer lot 72
et fichier privé détenu lot 73 publiés, relus sans blocage et CI vertes.
Lots 72–74 fusionnés dans #15, CI finale verte. Lot 75 identité source-scopée publié,
CI verte ; lot 76 migration/lecture du manifest développées. Écriture et importeur futurs.

## Décision et séparation des étapes

Le cadrage #5 exige ordre explicite, gzip validé jusqu'à EOF, SHA-256 décompressé,
reprise/idempotence et bornes. Commencer par une inspection en flux avant ingestion :
taille et SHA-256 du contenu entier, séparateurs et éventuel suffixe partiel inclus.
Pas de normalisation, observation, checkpoint ni écriture du manifest à cette étape.
La lecture ne déduplique pas les lignes identiques : elles restent des octets distincts.

`InspectPlain(ctx, reader, maxBytes)` utilise un buffer fixe de 32 Kio. Limite positive
obligatoire, au plus limite+1 octets consommés pour distinguer EOF exact d'un excès,
sans débordement int64. Hash hex minuscule seulement après EOF réussi. Une erreur,
un dépassement ou une annulation ne rend aucune métadonnée partielle exploitable.
Lectures vides répétées bornées ; lecteur et fermeture restent sous responsabilité
de l'appelant, qui fournira fichier régulier et deadline de l'import. Annulation
entre lectures bornées, pas interruption d'un syscall de lecture bloqué.

Le résultat indique si un suffixe ne se termine pas par LF. Ce suffixe doit être
traité explicitement par l'importeur : son SHA peut être connu mais cela ne signifie
ni que tous les records sont ingérés ni qu'un import_run peut devenir complete.
Le lecteur de lignes conserve ses bornes et ne normalise pas seulement un suffixe.

## Étape réalisée et lots suivants

1. Réalisé aux lots 72–73 : conserver les octets décompressés inspectés dans une
   copie privée bornée, SHA au même passage, writer fermé puis reader détenu.
   Entrée régulière ouverte avec propriété explicite ; échec/annulation refuse la
   copie et tente son cleanup, les erreurs de suppression étant signalées.
2. Suite : ordre/nombre de fichiers/durée globale bornés.
   Ne pas déduire une identité de contenu d'un chemin, inode ou en-tête gzip ;
   recompressions/renommages identiques doivent pouvoir retrouver la même origine.
3. Lot 76 : migration/lecture source-scopée du manifest développées, v1/v2 inchangés.
   Suite écriture : lier provenance/offsets/digest, inscrire running/failed et
   n'autoriser complete qu'après validation finale, dans la transaction du Sink.
4. Application du contenu validé à parser/Sink, reprise du checkpoint exact,
   depuis la copie privée validée. Une inspection n'est pas un snapshot filesystem
   et ne prouve pas un second passage identique de l'entrée originale.
5. CLI import hors service actif, ordre fourni sans tri implicite, recalcul M3,
   contraintes globales et scénarios de chevauchement avec suivi continu.

La stratégie d'identité ci-dessous précède le lot SQL. Pas de déduplication sur
seul hash de ligne. Reconnaître un
chevauchement FileSource uniquement avec provenance/positions et preuves concordantes ;
sinon le signaler incertain. Conserver les hypothèses des timestamps sans année.

## Limites actuelles

Les briques d'inspection/copie normale/gzip et préparation de fichier détenu existent.
Migration/lecture du manifest développées, aucune écriture publique, ingestion,
CLI, déduplication ni garantie de snapshot atomique. Les tests
restent synthétiques ; aucun accès Web à des chemins locaux d'import. MIT conservée,
authentification AD/OIDC après MVP.

## Prévalidation gzip du lot 70

InspectGzip utilise compress/gzip en mode multistream par défaut : chaque membre
et son trailer CRC/taille puis EOF final doivent réussir. Le digest porte seulement
sur les octets décompressés, quel que soit le nom/en-tête, niveau ou recompression.
Gzip vide valide rend le digest vide ; entrée compressée vide sans en-tête échoue.
Suffixe partiel signalé comme dans InspectPlain. Pas de succès d'import implicite.

GzipLimits exige trois limites entières positives : octets décompressés, octets
compressés (en-têtes compris) et ratio maximum. Deux budgets indépendants jusqu'à
limite+1 au niveau de leurs Reader, mémoire fixe ; les prélectures internes restent
distinctes des octets livrés. Contrôle du ratio après chaque lecture décompressée
sur les octets compressés consommés, prélecture comprise. Division/reste évitent
le débordement d'une multiplication. Un préfixe trop expansif est refusé même si
un membre ultérieur peu compressible aurait abaissé le ratio final.

Le lecteur compressé vérifie contexte/budget/n valide et borne 100 lectures vides :
la lecture d'en-tête gzip via io.ReadFull peut autrement boucler avant toute sortie.
Le décodeur est fermé et sa cause jointe sur retour ; l'input reste à l'appelant.
Erreurs/CRC/troncature/junk/limites/annulation ne rendent aucune métadonnée complète.
Tests synthétiques : identité normal/gzip/recompression, membres successifs,
second membre corrompu, CRC/taille/header/troncature/junk, budgets/ratio, contexte,
reader invalide et sans progrès. Exécution Linux vérifiée par CI 37244378495 verte
sur `8c48d1d7bdefa8e27aa21f0a8c3efd0a56ef4466`.

## Copie pendant inspection du lot 72

CopyPlain/CopyGzip écrivent vers un io.Writer fourni les mêmes octets utilisés pour
le digest, au même passage, y compris séparateurs/suffixe. Pour gzip, seuls les
octets décompressés sont copiés, tous les CRC restent requis. Budget contrôlé avant
Write ; aucune copie d'octets au-delà du budget. Pas de fermeture ou de seek.

Une sortie peut être partielle, voire contenir tout un payload CRC invalide, avant
l'erreur finale : le caller doit la jeter, sans ingestion. Writer nil refusé avant
lecture ; n invalide/short write/erreur/cancel échouent sans metadata et sans retry.
Si lecture et écriture échouent au même appel, leurs causes sont jointes, sauf EOF
exact qui reste un succès de lecture. Contrats io et usage exclusif requis.
Deadline entre lectures/écritures, sans interruption d'un syscall bloquant.

Ce mécanisme prépare la copie privée détenue du lot suivant ; une simple sortie
io.Writer n'est pas encore un fichier privé, ni un snapshot de l'entrée originale.

## Copie privée détenue du lot 73

PrepareRegular valide options/contexte, résout les chemins puis réutilise OpenLog
pour une entrée régulière possédée jusqu'à fermeture. Gzip est un choix explicite
d'appelant, pas inféré par le nom ou l'API Web. Plain utilise ContentBytes seulement,
gzip exige également CompressedBytes et MaxRatio positifs. Deadline globale au caller.

Une sous-directory MkdirTemp 0700 et un CreateTemp 0600 sont créés sous TempDir
(défaut os.TempDir), dont le parent doit être protégé/de confiance. Copie/digest au
même passage, writer fermé puis ouverture read-only à offset zéro, taille contrôlée.
Le résultat opaque PreparedContent fournit Info, Read, ReadAt, Seek et Close, un seul
consommateur. Les mutations ultérieures de l'entrée ne changent pas la copie validée.
Partiel reste metadata, sans déclarer l'import complet. La copie est éphémère, pas
un manifest durable et pas un snapshot atomique du fichier d'origine en mutation.

Les erreurs, annulations ou fermeture d'entrée en échec refusent le résultat et
ferment/nettoient la copie. Close libère la propriété avant fermeture/removal,
joint les causes et reste idempotent même sur échec : les removals échoués sont
signalés, sans retry silencieux. Remove de son fichier puis de son directory vide
seulement ; jamais de suppression récursive ou du parent TempDir/entrée d'origine.
Windows ACL non vérifiées ; protections de bits Unix exécutées en CI Linux verte
37245585832 sur `93cf83cec9a1a39dff5600f93ddd5a0bb93fc745`. La prochaine
application devra jeter la copie sur toute erreur et fermer le propriétaire final.

## Identité durable du lot 75 et contrat du manifest suivant

ImportOriginID prend l'ID exact et non vide de la source et le SHA-256 entier
décompressé, exactement 64 caractères hexadécimaux minuscules. Il refuse toute
autre représentation, sans trim ni casse implicite, et ne prouve aucune lecture
ou validation de fichier. Le caller fournit le digest d'une préparation réussie.

Contrat stable : `import-v1:` suivi du SHA-256 hex minuscule de la concaténation
des octets `queueatlas/import-origin/v1` puis NUL, de la longueur en octets de
l'ID source sur huit octets big-endian, de l'ID source, puis des 32 octets du digest.
Préfixe versionné et longueur évitent les ambiguïtés de concaténation. IDs source
opaques, octets Go exacts sans transcodage ou validation UTF-8 ; espaces/casse/NUL/
Unicode conservés. Un test golden fixe le format durable.
Renommage ou recompression du même contenu retrouve cette origine dans la même
source ; un digest ou une source différents donnent une origine différente.
La provenance garde chaque offset : deux lignes identiques ne sont pas supprimées.

Le lot 76 ajoute une migration v3 sans modifier les SQL v1/v2.
Un run sera une tentative explicite avec ID positif fourni par l'appelant,
globalement unique dans la base, y compris parmi les lignes legacy,
source de kind import distincte du suivi live, chemin original, état, digest/taille
du contenu validé, origine associée et offsets. Les lignes v1 sans source restent
héritées, non attribuées : aucune adoption ou reprise implicite. Le lien doit être
source-scopé, l'identité de contenu immuable ; les lecteurs n'interprètent pas
le chemin comme preuve. Préparation échouée sans digest valide ne doit pas produire
une origine ni des records ; la tentative peut être tracée failed sans contenu.

L'écriture du manifest sera ensuite intégrée au même Commit que records/checkpoint,
avec réessai identique après ACK perdu, sans avance séparée. Complete exigera EOF
validé, absence de suffixe partiel et offset égal à la taille ; cette validation
ne se déduit pas du SHA seul. Ces comportements SQL et l'application ne sont pas
encore implémentés au lot 76. Mono-écrivain/service arrêté au MVP. Aucune déduplication
inter-source prouvée par ImportOriginID, aucun chemin fourni par l'API Web.

## Migration et lecteur du lot 76

V3 reconstruit seulement import_runs avec les huit colonnes v1 conservées et
source_id/generation_id/trailing_partial nullable. Les anciennes valeurs, même
sans digest canonique ou sans fin renseignée, sont copiées sans adoption. Le DDL,
l'historique et user_version=3 sont dans la même transaction ; refus d'historique
manquant ou version future conservé. Index digest recréé, index source/origine ajouté.

Pour une ligne associée : ID positif, source/chemin non vides, FK source et FK
composite source/origine, SHA canonique et taille/partial présents ensemble ; longueur
TEXT et BLOB égales à 64 avec GLOB hex minuscule pour exclure aussi les NUL. Sans
contenu, offset zéro et état running/failed seulement. Running sans fin, états
terminaux avec fin >= début. Offset <= taille, complete seulement à taille exacte
sans partial. SQL ne prouve ni la lecture du fichier ni le checksum : caller requis.

ImportStateReader.ImportRun effectue une lookup exacte source/ID positif et une
jointure unique vers source/origine. Legacy ou autre source : absence, distincte
de zéro. Kind import, ID dérivé, fingerprint sha256 et absence d'identité physique
contrôlés avant exposition ; état incohérent refuse avec résultat vide. Dates UTC,
pointeurs rendus possédés par caller. Aucun write/adoption, décision de reprise,
checkpoint ou page globale implicite. Une lecture de checkpoint ultérieure est
un autre snapshot ; sérialiser les écritures/réessais reste à l'application.
