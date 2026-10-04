# Revue de la PR #11 — partie 4 : lecteurs d'état SQLite

Revue du 4 octobre 2026 sur la tête publiée
`9007404264ba9b4a1a99a8563ce0fb9ab2155f40`, basée sur main après fusion de #10.
Coordinateur et auditeur agent indépendant en lecture seule. Périmètre : state.go,
contrats OriginQuery/OriginPathQuery/OriginPage et tests de lecture correspondants.
Données synthétiques uniquement.

## Résultat

Aucun blocage concret d'exécution identifié par les deux revues. Le commentaire
de Store.Checkpoint proposait toutefois de commencer à zéro lorsqu'aucune position
n'existait. Il précise maintenant que l'absence est distincte d'un zéro acquitté et
n'autorise pas une reprise automatique ; la décision appartient à la source.
Aucun changement de comportement ni nouveau test sans défaut d'exécution à reproduire.

Garanties relues :

- SourceID et identité physique ou chemin requis ; Limit validée dans [1, 100].
  SELECT avec paramètres liés, ordre croissant par ID et curseur exclusif. Le chemin
  reste littéral, sensible à la casse, sans normalisation ni interprétation de %/_.
  Un curseur n'a pas besoin d'identifier une ligne existante.
- Chaque requête joint l'origine et son checkpoint optionnel sur source_id et
  generation_id, avec filtre de source conservé. Une seule instruction SQL fournit
  les métadonnées et la position d'une page ; pas de lecture séparée de checkpoint
  susceptible de mélanger deux commits dans cette page.
- LIMIT est fixé à Limit+1 après validation : au plus 101 lignes lues et 100 états
  retournés. La ligne supplémentaire indique la continuation ; NextID est le dernier
  ID retourné, et non celui de cette ligne supplémentaire. Page exactement complète
  sans ligne suivante : curseur vide. Les checkpoints retournés sont distincts.
- LEFT JOIN et NullInt64 conservent checkpoint absent, zéro et positif. Horodatage
  FirstSeen restauré en UTC ; empreinte, ancre, chemin et identité restent des
  métadonnées persistées, sans prétendre les vérifier sur le filesystem.
- Query, Scan ou Rows.Err en erreur retournent une page zéro, sans états partiels.
  Rows est fermé ; requête avec contexte déjà annulé refusée. Les lectures n'écrivent
  pas d'état et n'infèrent aucune génération ou décision de reprise.

## Vérifications

Coordinateur et auditeur :

```powershell
go test ./internal/storage/sqlite -run '^TestFileOrigins' -count=1
```

Réussi sous Windows : réouverture, pagination, isolation source/device/inode/chemin,
nil/zéro/positif, ordre différent des dates, limites, annulation et paramètres
ressemblant à du SQL conservés comme valeurs littérales. Coordinateur : test
TestLoadPathOriginsWithSQLiteAndReducedFinalPage -count=1 également réussi, pour
101 origines et plafond 100 sans résultat partiel. git diff --check réussi.

La [CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37202460211)
a réussi : tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64
sans CGO et job Windows ciblé sur les chemins. Ce lot change uniquement des
commentaires et la documentation ; consulter la PR #11 pour la CI de publication.

## Limites et suite

Les pages successives ne constituent pas un instantané global. Les écritures de
la source doivent être sérialisées pendant une sélection et son application.
Le nombre de résultats est borné, pas la taille totale des chaînes persistées ou
le coût du parcours SQL ; mesures de charge et taille restent à faire avant
distribution. Aucun scénario de corruption ou d'écriture concurrente injecté dans
ce lot : l'atomicité de page est établie par la requête unique et son contrat SQLite.
Audit assisté par agents, sans certification humaine externe.

Migration et transitions lifecycle v2, rotation et reprise orchestrée/Run restent
hors périmètre. PR #11 toujours en brouillon. Prochain petit lot : migration v2 et
transitions durables acquittées, avec refus des conflits et rollback de batch.
