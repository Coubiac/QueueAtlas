# ADR-003 — Source et acquittement durable

Statut : accepté le 3 octobre 2026.

## Contexte
Un simple envoi sur canal ne dit pas si ligne et checkpoint sont persistés.
## Options
Canal sans accusé ; canal avec accusé ; `Source.Run(ctx, Sink)` et `Sink.Commit(batch)`.
## Décision
Adopter `Source.Run` avec `Sink.Commit` acquitté après transaction contenant observations, projection et checkpoints.
## Raisons
Reprise après crash avec effet idempotent par identité de provenance `source/origine/offset`.
## Conséquences
Lots et files bornés, backpressure, curseurs opaques pour autres sources, tests avant/après commit. FileSource lit les checkpoints via une interface injectée.
## Limites
Impossible de récupérer des octets déjà effacés par rotation ; TCP seul ne garantit pas la reprise d'un futur SyslogSource.
