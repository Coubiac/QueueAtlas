# Revue de la PR #11 — partie 5 : migration et transitions SQLite v2

Revue du 4 octobre 2026 sur la tête publiée
`a0c0d13319a27c38ffad8054cc6b081fbec76299`, basée sur main après fusion de #10.
Coordinateur et auditeur agent indépendant en lecture seule. Périmètre : delta v2
de migration.go, validate/Commit pour FollowTransitions, contrat source et tests
follow_state_test.go. Données synthétiques uniquement.

## Résultat et garanties relues

Aucun blocage concret identifié par les deux revues. Aucun correctif de code ou
nouveau test sans défaut concret à reproduire.

- Migration v2 : ajout d'une colonne INTEGER NOT NULL, DEFAULT 0 et CHECK limité
  aux états 0/1/2. Les origines v1 deviennent unknown, sans déduction par date ou
  checkpoint. Tables de provenance, observations et positions v1 conservées.
- Colonne, entrée d'historique v2 et user_version=2 dans la même transaction.
  Historique des versions requises vérifié avant migration ; version future ou
  négative refusée. Base neuve : v1 puis v2 dans la transaction. Échec du DDL,
  de l'historique ou du commit : aucun schéma/version partiellement acquitté.
- Trois transitions autorisées : unknown→following, following→retired et
  retired→following. Retour unknown, retrait direct unknown, transition vers
  soi-même, valeur invalide, origine vide ou doublon d'origine refusés. Une source
  non file ne peut pas porter une transition de suivi.
- UPDATE avec paramètres liés, filtré par source_id et ID d'origine. L'état doit
  être l'attendu ou déjà la cible pour un retry idempotent. Exactement une ligne
  requise ; origine absente/étrangère ou état divergent : ErrFollowStateConflict.
- Transitions dans la même transaction que origine, records/events et checkpoints.
  Un conflit ultérieur ou un checkpoint invalide annule tous les effets antérieurs
  du batch. Batch sans transition et réenregistrement des métadonnées ne réinitialisent
  pas l'état. Les seuls changements d'état n'altèrent pas provenance ou position.

## Vérifications

Coordinateur :

```powershell
go test ./internal/storage/sqlite -run '^TestFollow(Transitions|StateMigration)' -count=1
```

Auditeur : TestFollow* -count=1. Tests réussis sous Windows, checkout isolé propre
et git diff --check réussi. Tests existants : acquisition/retrait/réacquisition et
réessais, réouverture, deux lecteurs d'état, refus des origines étrangères/absentes,
contrainte de colonne, annulation, rollback état/record/event/checkpoints et batch
corrigé ; migration v1 avec données conservées, échec d'historique via trigger
synthétique avec DDL/version annulés, refus d'historique v2 manquant.

La [CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37202826203)
a réussi : tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64
sans CGO et job Windows ciblé chemins. Code inchangé dans ce lot ; consulter la
PR #11 pour la CI de publication du rapport.

## Limites et suite

Le Sink n'observe ni fichier, ancre, EOF ou grâce : l'appelant justifie chaque
transition. Les transitions n'ont pas d'époque de propriétaire/protection ABA ;
un retry ancien après un cycle de réacquisition ne peut pas être identifié par ces
seuls états. Sérialisation des écritures et réessais par source obligatoire,
comme accepté dans ADR-009. Aucun crash réel, concurrence ou corruption injecté
dans ce lot. Le contrôle d'historique n'est pas une validation exhaustive d'une
base modifiée extérieurement. DDL/version atomiques ne signifient pas que les
préparatifs Open, dont le choix WAL, font partie de la même transaction.

Acquisition/retrait par FileSource, rotation et reprise orchestrée/Run restent à
relire. PR #11 en brouillon. Prochain petit lot : acquisition d'une génération
avant consommation de sa première ligne, refus et propriété des descripteurs.
Audit assisté par agents, sans certification humaine externe.
