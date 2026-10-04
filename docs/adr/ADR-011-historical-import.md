# ADR-011 — import historique borné et identité de contenu

Statut : prévalidation normale/gzip lots 69–71 fusionnée, copie vers writer lot 72
développée/relue. Fichier privé détenu, importeur et manifest non implémentés.

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

## Lots suivants et invariants à conserver

1. Conserver les octets décompressés inspectés dans une copie privée bornée avant
   ingestion, pour ne pas relire un fichier d'origine modifié sous le même digest.
   Copier au même passage que le SHA, fermer le writer puis rendre un reader détenu.
   Échec/annulation : copie partielle supprimée, aucune ingestion de son contenu.
2. Ouvrir l'entrée régulière avec propriété explicite ; ordre/nombre de fichiers/durée globale bornés.
   Ne pas déduire une identité de contenu d'un chemin, inode ou en-tête gzip ;
   recompressions/renommages identiques doivent pouvoir retrouver la même origine.
3. Ajouter lecture/écriture source-scopée du manifest dans un lot stockage distinct,
   sans retoucher le schéma v1 publié. Lier provenance/offsets/digest décompressé,
   inscrire running/failed et n'autoriser complete qu'après validation finale.
4. Application du contenu validé à parser/Sink, reprise du checkpoint exact,
   depuis la copie privée validée. Une inspection n'est pas un snapshot filesystem
   et ne prouve pas un second passage identique de l'entrée originale.
5. CLI import hors service actif, ordre fourni sans tri implicite, recalcul M3,
   contraintes globales et scénarios de chevauchement avec suivi continu.

La stratégie d'identité et l'évolution SQL seront précisées avant leur lot
d'implémentation. Pas de déduplication sur seul hash de ligne. Reconnaître un
chevauchement FileSource uniquement avec provenance/positions et preuves concordantes ;
sinon le signaler incertain. Conserver les hypothèses des timestamps sans année.

## Limites actuelles

Les briques d'inspection/copie normale/gzip existent. Pas encore d'ouverture de fichiers,
import_run, ingestion, CLI, déduplication ni garantie de snapshot. Les tests
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

Ce mécanisme prépare la copie privée détenue du prochain lot ; une simple sortie
io.Writer n'est pas encore un fichier privé, ni un snapshot de l'entrée originale.
