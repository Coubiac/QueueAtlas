# Détail de candidat authentifié — lot 156

Le handler de [recherche](http-search.md) dessert aussi
`GET /api/v1/messages/{id}` et `HEAD` via la même instance protégée par
`auth.Protect`. Les deux routes partagent admission, deadline, budget de faits,
encodage JSON plafonné à1MiB et erreurs privées. Aucun listener, SQL write,
installation de projection ou interface Web n'est créé dans ce lot.

## Identité révisable

Les candidats de recherche contiennent désormais `id`, encodé par
`EncodeCandidateID(correlation.QueueInstanceKey)`. Il représente une génération
candidate dans le périmètre complet d'une file exacte, toutes origines et dates.
Ce n'est pas une identité globale de message, un ordinal chronologique ou une
preuve de continuité. NOQUEUE et streams non résolus restent sans candidat/ID.

Le format version1 est du base64url canonique sans padding : version (1octet),
révision SHA-256 (32), génération (uint16 big-endian), taille instance (uint16),
taille Queue-ID (uint8), puis leurs octets UTF-8 exacts. Génération0..4095,
instance1..1024octets, Queue-ID1..32octets ; révision64hex minuscules.
Identifiant54..1459caractères. Version/longueurs/bornes/UTF-8/contrôles/bits inutilisés
et encodage canonique sont vérifiés avant tout accès base, sans erreur détaillée.
Pas de trim/casse ou autre normalisation de l'instance/file.

L'identifiant est non signé et décodable ; il n'est ni secret ni autorisation.
Le garde d'authentification est exécuté sur chaque requête. Le compte local possède
la lecture actuelle des instances configurées ; une future politique de rôles
devra vérifier aussi le scope décodé. L'infrastructure doit éviter de journaliser
ces URLs qui contiennent une référence à des métadonnées privées.

## Lecture et contrôle de révision

Le détail n'accepte aucun paramètre query, même un `?` vide, ni body/transfer
encoding. Les règles de TLS/Host/Origin/metadata, méthodes GET/HEAD et cache restent
celles du handler155. Chemin avec segment supplémentaire :404 ; ID malformé :400,
avant toute lecture de faits. Cette route ne dépend pas de l'horloge de défaut de
la recherche et ne lance pas une nouvelle recherche filtrée.

Une lecture complète `CorrelationFacts` pour l'instance/file exacte conserve
les faits hors fenêtre, toutes origines, cycles et faits sans date. Le dépassement
fait échouer le détail entier. `buildQueueProjection` est partagé avec la recherche :
mêmes faits et fenêtre de liens1minute sans binding SMTP donnent la même révision.
Aucune projection persistée ancienne n'est reprise ni aucune autre file parcourue.

La révision courante doit correspondre à l'ID avant de sélectionner la génération.
Import tardif/rétention/changement de faits :409 `stale_candidate`, sans révéler la
nouvelle révision. L'utilisateur doit refaire la recherche. File absente dans le
snapshot ou génération inexistante avec révision courante :404 `not_found` ; ce
statut ne prouve pas l'absence de logs ni une issue de livraison. Le même ID ne
devient jamais silencieusement une autre génération après changement de révision.
Budget de faits :422 `detail_too_broad`, autres erreurs/annulation/délai/volume ou
données invalides :503 `unavailable`. Pas de résultat partiel ni de détail SQL.

## Réponse de détail

| Champ | Sens |
| --- | --- |
| `candidate` | ID, révision, instance/file/génération, compteurs et réserves de recherche |
| `coverage_unproven` | Toujours vrai ; aucune preuve de couverture physique des logs |
| `first`, `removed` | Références physiques de l'ancre/removal observé ; removal absent = null |
| `receipt_observed` | Réception native observée, pas certificat de parcours global |
| `has_non_explicit_time`, `cross_stream_uncertain` | Réserves conservées du candidat |
| `recipients` | Tableau de résultats par adresse exacte observée |
| `unprojected_delivery_count` | Nombre de reports de livraison non projetés dans ces résultats |

Chaque destinataire expose son adresse, `observed_status`, `order_uncertain`,
`address_unspecified`, le nombre de tentatives et les références `latest`.
Deux résultats contradictoires à la dernière date restent unknown avec les deux
références ; le tie-break de provenance ne choisit pas un faux succès. `sent`
SMTP reste un report de transport distinct de `delivered`. Les cycles/origines
indépendants ne fusionnent pas leurs destinataires ni leurs compteurs.

L'adresse est un objet `{"encoding":"utf8","value":"…"}` pour des octets UTF-8
valides, ou `{"encoding":"base64","value":"…"}` en base64 standard paddé si le
lecteur fournit des octets non UTF-8. La conversion du DTO ne les remplace pas et
ne les normalise pas ; elle ne modifie pas le codec de champs existant du stockage.
Valeur explicitement vide conservée, casse conservée ; adresse au plus1024octets,
au-delà503 entier. Les tests binaires du DTO utilisent le seam de lecteur privé,
pas une revendication d'import SQLite de champs binaires. Provenance sous forme
de source/origine UTF-8 bornées et offsets décimaux int64 comme au lot155.

Le détail est une allowlist de métadonnées, pas une sérialisation de Projection ou
Observation. Pas de lignes/messages bruts, maps de champs, DSN/réponse/relay,
credential, cookie ou compte opérateur. Les tentatives détaillées, la timeline et
la politique de lignes brutes sont dans [http-timeline](http-timeline.md), lot157.
Les liens entre files restent au backlog applicatif.
Les tableaux vides restent `[]`. Encodage JSON standard échappé avant réponse200 ;
HEAD effectue la même vérification/lecture mais ne retourne pas de corps.

## Vérifications réalisées

Quatre nouveaux tests, seize HTTPAPI au total passent Windows Go1.26 : codec avec
vecteur struct/base64 indépendant, limites de longueurs/UTF-8/ordinal/canonicalité,
SQLite réel recherche→détail avec livraison/removal hors période et plusieurs
origines, génération distincte/inexistante, budget entier et import tardif409,
protocole/auth/query avant lecture, HEAD réussi/erreur, erreurs SQL privées,
destinataires littéraux/empty/binary/casse et résultats simultanés contradictoires,
deadline avec réponse tardive refusée. Le test d'admission155 vérifie maintenant
que recherche et détail partagent le slot. Vet/format/diff passent ; CI Windows et
race HTTPAPI Linux existantes incluront ces tests après publication.

Lot156 publié sur2298709 ; [CI37646414652](https://github.com/Coubiac/QueueAtlas/actions/runs/37646414652)
entière/trois jobs/SHA exact réussis, race HTTPAPI Linux1.26 passée, journal Windows
HTTPAPI réussi vérifié ; état et tête revérifiés REST157. MIT conservée,
AD/OIDC/Keycloak après MVP. Timeline157 ajoutée dans la même PR ; revue API158 ensuite.
