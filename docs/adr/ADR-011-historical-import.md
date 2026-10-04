# ADR-011 — import historique borné et identité de contenu

Statut : prévalidation normale développée au lot 69, contrat de suite retenu le
5 octobre 2026. Importeur et manifest non encore implémentés.

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

1. Inspecter gzip séparément : réutiliser la limite des octets décompressés, ajouter
   une limite de ratio, lire à EOF pour contrôler CRC/taille et tous les membres.
   Erreur de gzip, ratio, fermeture/annulation sans résultat complet ; pas d'ingestion.
2. Préparer un fichier régulier détenu, ordre/nombre de fichiers/durée globale bornés.
   Ne pas déduire une identité de contenu d'un chemin, inode ou en-tête gzip ;
   recompressions/renommages identiques doivent pouvoir retrouver la même origine.
3. Ajouter lecture/écriture source-scopée du manifest dans un lot stockage distinct,
   sans retoucher le schéma v1 publié. Lier provenance/offsets/digest décompressé,
   inscrire running/failed et n'autoriser complete qu'après validation finale.
4. Application du contenu validé à parser/Sink, reprise du checkpoint exact,
   vérification du contenu utilisé entre inspection et ingestion. Une inspection
   n'est pas un snapshot filesystem et ne prouve pas un second passage identique.
5. CLI import hors service actif, ordre fourni sans tri implicite, recalcul M3,
   contraintes globales et scénarios de chevauchement avec suivi continu.

La stratégie d'identité et l'évolution SQL seront précisées avant leur lot
d'implémentation. Pas de déduplication sur seul hash de ligne. Reconnaître un
chevauchement FileSource uniquement avec provenance/positions et preuves concordantes ;
sinon le signaler incertain. Conserver les hypothèses des timestamps sans année.

## Limites actuelles

Seule la brique d'inspection normale existe. Pas encore d'ouverture de fichiers,
gzip, import_run, ingestion, CLI, déduplication ni garantie de snapshot. Les tests
restent synthétiques ; aucun accès Web à des chemins locaux d'import. MIT conservée,
authentification AD/OIDC après MVP.
