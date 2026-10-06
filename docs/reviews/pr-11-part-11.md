# Revue de la PR #11 — partie 11 : recherche de rotation

Revue du 4 octobre 2026 sur la tête publiée
`dcc1455c1978ebd3431366f908599748e685de7d`, reprise après interruption de quota.
Coordinateur et auditeur agent indépendant en lecture seule. Périmètre :
SelectRotation, scanRotation, checkRotationEntry et tests. VerifyCandidate et OpenLog
réutilisent leurs revues précédentes. Localisation de l'ensemble et Run exclus.
Données synthétiques uniquement.

## Défauts Windows corrigés

Le contrôle du répertoire utilisait encore os.Stat : ses IDs Windows pouvaient être
chargés seulement lors du SameFile suivant l'ouverture. Une substitution pouvait
donc faire attribuer au snapshot précédent l'identité du nouveau répertoire.
Une entrée de répertoire avec IDs différés présentait le même risque avant OpenLog.

Deux régressions déterministes échouent avant correction : SelectRotation accepte
le répertoire remplacé comme absent sans erreur ; checkRotationEntry accepte le
fichier remplacé comme candidat insufficient. Aucun faux unique Windows démontré,
car les identifiants persistants ne sont actuellement disponibles que sur Linux.

- Le snapshot du répertoire utilise désormais statPath, capturant ses IDs avant
  ouverture. La substitution observée est refusée avec ErrPathChanged.
- Le helper Windows fige les IDs de entry.Info avant ouverture de données,
  conserve les IDs déjà fournis par NTFS, puis compare un snapshot statPath cohérent.
  Mode régulier et identité sont revérifiés ; ce snapshot est comparé à OpenLog.
  Le helper non-Windows conserve entry.Info sans coût supplémentaire.
- Les deux régressions passent après correction. Un test nominal démontre qu'une
  entrée différée inchangée reste éligible, avec diagnostic physical_identity_unavailable
  Windows attendu. Les handles sont fermés avant retour, y compris sur refus.
- Le job Windows chemins exécute désormais SelectRotation et RotationEntry,
  incluant ces trois tests. Patch relu indépendamment, sans blocage restant identifié.

## Garanties du scan relues

Scan non récursif, pages d'au plus 32 entrées et budget de 1 à 1000 incluant les
exclusions. Une entrée supplémentaire permet de distinguer fin exacte et limite,
sans vérifier de candidat au-delà du budget. Seule la fin du scan avec un match
unique et sans preuve insuffisante fournit un chemin. Hard links ambigus,
insufficient/limit et erreurs n'exposent pas de chemin partiel.

Liens et types non réguliers observés, noms .gz insensibles à la casse et signature
gzip exclus ; aucune décompression. Preuves strictes d'identité/préfixe/ancre/LF,
sans replay zéro implicite. Au plus un descripteur candidat avec celui du répertoire,
plus handles temporaires de métadonnées Windows. Fermetures jointes aux erreurs et
résultat annulé sur erreur ; ni checkpoint ni état d'ingestion modifié.

## Vérifications

Coordinateur : deux régressions Windows échouant avant puis passant après ; tests
RotationScan/SelectRotation/RotationEntry -count=1 réussis, cas nominal ajouté et
réussi. go test ./..., go vet ./..., git diff --check et compilation des tests
FileSource Linux amd64 sans CGO réussis localement. Auditeur : tests ciblés avec les
deux régressions et diff réussis, patch relu ; cas nominal demandé par sa revue et
vérifié ensuite par le coordinateur.

Tests Linux relus : renommage/append, absence/différence/zéro, hard links, budget,
gzip nom/signature, sous-répertoire/lien, disparition/substitution d'entrée et
fermeture des fichiers/répertoire. Pas d'exécution locale Linux sous Windows.
La [CI de la référence initiale](https://github.com/Coubiac/QueueAtlas/actions/runs/37205110566)
était verte ; elle ne valide pas le correctif. Le correctif publié
`de81c4e86fc3deab0692bf471ae5ffeb6aa8f6b9` a sa
[CI réussie](https://github.com/Coubiac/QueueAtlas/actions/runs/37220202856) :
tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64 sans CGO
et job Windows étendu exécutant les trois nouveaux tests. Lot clos sur cette référence.
Consulter la PR #11 pour la CI de publication de cette clôture documentaire.

## Limites et suite

Le fallback Windows réel sur un autre filesystem n'est pas testé ; le test utilise
FileInfoToDirEntry(os.Stat) pour exercer les IDs différés. Dans ce fallback, identité
acquise au chargement, pas rétroactivement à l'énumération. SameFile masque les
erreurs natives et peut échouer sur un fichier ouvert par un tiers : refus sûr,
sans preuve certaine de substitution. Pas de prise en charge nouvelle de reprise
persistante Windows. Syscalls non interruptibles et observations non atomiques ;
réouverture/revérification nécessaires avant usage du chemin sélectionné.

PR #11 en brouillon. Prochain petit lot : LocateFollowOrigins, budget partagé,
ensemble complet et absence de chemins partiels. Réouverture/transfert, préparation
orchestrée et Run seront relus séparément. Audit assisté par agents, sans
certification humaine externe.
