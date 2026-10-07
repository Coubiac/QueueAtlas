# Relecture du chantier API — lot 158

Périmètre : [PR #36](https://github.com/Coubiac/QueueAtlas/pull/36), lots154–157,
tête de code `277d981e33f93ae4c055ae3a89df711910d67fb7`, base main153
`81f9f79b2c0a444d017095516e9c2e3976ba0caa`. Résultat attendu158 : revue du code,
documentation de clôture, CI finale, revue COMMENT sur la tête finale, fusion
préservant les petits commits, CI main puis nettoyage. Aucun nouveau comportement.

Relecture assistée favorable : aucun défaut bloquant identifié dans ce périmètre.
Elle ne constitue pas une approbation humaine indépendante, ni une sortie de M4.

## Contrats confrontés au code et aux preuves

| Point | Résultat | Preuve utilisée |
| --- | --- | --- |
| Frontière d'authentification | Constructeur public retourne seulement Protect ; trois routes GET/HEAD sous même garde, aucune exemption de chemin ; compte local sans RBAC par instance | search_handler, auth/http_guard ; protocoles search/detail/timeline avant base, HTTPS réel et révocation155 |
| Entrées | Query fermée/unique/bornée, dates UTC ns et fenêtres exactes, corps non lu, chemins et IDs canoniques ; six critères seulement | search_request, candidate_id, timeline_request ; tables hostiles/limites et golden indépendants154/156/157 |
| SQL | Validateur pur réutilise eventSearchSelection ; valeurs en paramètres et enum de colonne fermé ; diff SQLite limité à Validate et tests | search.go avant/après ; régressions SQL/binary/index111–115 réutilisées, CI complète157 |
| Pagination recherche | Position publique liée au sélecteur/fenêtre, taille variable, nouveaux snapshots ; imports anciens nécessitent une nouvelle recherche | search_cursor et vecteur domaine normalisé ; tests154/155 et contrat http-search |
| Faits complets | Une lecture agrégée par scope exact avec budget global, toutes origines/dates ; chaque file reconstruite séparément | searchScope/searchMatches, CorrelationFacts ; SQLite réel155/156/157, revision stable entre pages/scopes |
| Détail/timeline | Même lecture complète avant révision puis ordinal/position ; pas d'ancienne génération réassignée après import tardif | candidateSnapshot ; tests SQLite409156/157 avant position hors génération ; compteurs/faits hors fenêtre |
| Sémantique | Pas d'identité globale ; NOQUEUE/conflits non assignés, réserves conservées, sent SMTP distinct de delivered ; tie-break d'affichage sans faux verdict | BuildProjection/DeliveryFrom réutilisés ; tests warning/unresolved155, conflits156/157 |
| DTO | Allowlist, pas de sérialisation Observation/Fields/Message/credentials/chemin ; offsets int64 en chaînes, présence/vide et casse conservés | search_response/candidate_detail/candidate_timeline ; tests allowlist, >2^53, octets/presence/échappement |
| Lignes brutes | Option serveur false par défaut ET raw=1 ; refus403 avant base, UTF-8/base64 des octets du record | test SQLite binaire157 exact, permission seule sans exposition ; champs parsés binaires testés via seam, sans promesse d'import JSON |
| Admission/délai | Un handler partagé, slots jusqu'au retour, pas d'attente ; contexte SQL et refus des réussites tardives | tests slot commun trois routes, annulation et deadlines155–157 ; race HTTPAPI Linux1.26 |
| Réponses | no-store/nosniff/Vary hérités, pas de CORS/cookie/redirect ajouté ; JSON entièrement encodé avant200 et cap1MiB, HEAD sans corps | assertSearchHTTP/HTTPS155, HEAD détail/timeline, dépassements expansés155/157 ; erreurs privées fixes |
| Compatibilité | Aucun listener, route de mutation, schéma ou dépendance ajouté ; CI de forks sans secrets, actions épinglées | diff main153..277d981 ; CI Go1.26/stable/Windows, builds statiques et races157 |

Lecture du périmètre et des points d'appel de Protect, du validateur SQL, de la
partition de générations et du hash de faits/options. La révision inclut Raw et
les champs/présences/date ; une page d'événements ne remplace jamais cet ensemble.
Pas de demande de reviewer externe ni de déclaration d'approbation indépendante.

## Limites conservées et travail applicatif restant

- Partager une instance auth/login/sessions et un handler de données, Store déjà
  ouvert. Configurer le montage, listener HTTPS, certificats, headers, timeouts
  réseau et shutdown ; proxy TLS vers HTTP non pris en charge par la garde actuelle.
- Le compte local a accès aux instances de cette bibliothèque. AllowRawLogs est
  une permission de configuration de ce handler, sans rôles par compte/provider.
  Relay/DSN/réponse restent privés et visibles dans la timeline sans ligne brute.
- Les budgets portent sur faits/pages/requêtes et réponse encodée. Ils ne donnent
  pas un plafond de RAM1MiB ni une interruption des calculs purs. Les seuls octets
  bruts de1024records valides peuvent représenter64MiB (4096=>256MiB), auxquels
  s'ajoutent champs, reconstruction, copies et encodage JSON. La concurrence
  multiplie ces ressources ; mesurer/ajuster au pilote avant une prétendue limite
  de mémoire. La désactivation de sortie raw ne supprime pas sa lecture interne
  nécessaire à la révision actuelle. Aucune nouvelle mesure de charge revendiquée.
- Une file dépassant FactLimit échoue entièrement même avec limit1. Une réponse
  de gros records peut nécessiter limit1. Pas de pages partielles de reconstruction
  pour obtenir artificiellement un résumé. Délais d'écriture réseau à régler au serveur.
- La recherche paginate des événements, avec candidats éventuellement répétés,
  pas une liste de messages globaux. Snapshots de recherche/faits distincts et
  imports tardifs : nouvelle recherche si nécessaire, pas de gel entre pages.
- Les champs parsés SQLite utilisent déjà JSON : impossible de promettre des
  octets non UTF-8 antérieurs déjà remplacés. Raw SQLite est bien conservé/testé.
- JSON échappé n'est pas une preuve de rendu Web inoffensif. Le futur Web doit
  utiliser du texte/html-template, CSP, navigation/formulaires accessibles et
  vérification du corpus hostile dans un navigateur réel avec auth/cache.
- Filtres statut/direction/IP/SASL/hôte, état sources/lacunes, health/ready,
  liens entre files et raccordements CLI/YAML/ingestion restent au backlog du
  cadrage. La clôture ne déclare ni ces critères ni M4/MVP terminés.

## Vérifications et clôture

[CI15737650350084](https://github.com/Coubiac/QueueAtlas/actions/runs/37650350084)
entière completed/success sur277d981 exact, trois jobs : Windows112891756168,
stable112891756499, Go1.26 112891756754. Étapes HTTPAPI Windows et race HTTPAPI
Linux1.26 réussies, revérifiées REST158 avec tête PR/base et absence de reviews/
commentaires inline existants. Tests/vet/format Linux complets, builds statiques
amd64/arm64 et régressions des fondations réutilisés. Pas de Linux local revendiqué.

Les21tests HTTPAPI relancés Windows Go1.26 au lot158 passent (`-count=1`), pour
vérifier ensemble les contrats clos ; format/diff passent. Aucun changement de
code/tests requis par la revue, pas de rerun optionnel des fondations/fuzz/perf.

Au commit158 : CI finale sur la nouvelle tête documentaire, revue COMMENT,
passage ready, fusion avec SHA attendu, CI main et nettoyage encore à terminer.
La preuve post-publication est consignée dans #36 puis à la reprise159.
Prochain159 : premier lot Web, page de recherche protégée exploitant les six
critères existants, résultats échappés et réserves visibles ; détail/timeline Web
dans les lots suivants. MIT conservée, AD/OIDC/Keycloak après MVP.

## Clôture effective158, consignée159

Tête finale `e8bde7189be8ae45ff0e156cd7ed0924e9a0c1b2` :
[CI37653248114](https://github.com/Coubiac/QueueAtlas/actions/runs/37653248114)
entière completed/success, trois jobs Windows112901746659, Go1.26 112901747061,
stable112901747095 ; HTTPAPI Windows/race HTTPAPI Linux1.26 réussis.
Revue COMMENT5445385773 sur cette tête, puis #36 rendue prête et fusionnée par
merge avec SHA attendu, sans écraser les petits commits. Revue assistée uniquement.

Main `8c3aa859eb9489f0e969cb47b087053448126ac4` :
[CI37653544179](https://github.com/Coubiac/QueueAtlas/actions/runs/37653544179)
entière completed/success, trois jobs Windows112902777063, Go1.26 112902777243,
stable112902777253 ; HTTPAPI Windows/race HTTPAPI Linux1.26 réussis.
Métadonnées/SHA/étapes revérifiés REST159. Branche codex/m4-search-api supprimée
GitHub/local après vérification d'ascendance ; main propre avant codex/m4-web.
Ce résultat clôt l'API bibliothèque, pas l'issue #7 ni M4.
