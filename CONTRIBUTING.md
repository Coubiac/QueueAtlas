# Contribuer à QueueAtlas

QueueAtlas est au début de son développement. Avant une modification importante, consultez le [cadrage](docs/phase-0-proposal.md) et les [issues ouvertes](https://github.com/Coubiac/mailtrace/issues), puis décrivez dans l'issue le contrat ou le comportement visé.

Le développement avance par petits lots testables. Le [point de reprise](docs/reprise.md) indique l'état validé et le prochain lot ; les consignes pour les agents sont dans [AGENTS.md](AGENTS.md).

Pour une PR :

1. Gardez un périmètre limité et reliez l'issue concernée.
2. Ajoutez des fixtures **synthétiques** et des tests qui distinguent faits observés et conclusions inconnues.
3. Ne soumettez aucun log de production, adresse personnelle, secret ou base réelle.
4. Exécutez `go fmt ./...`, `go vet ./...` et `go test ./...`.
5. Décrivez les cas testés, les limites et les implications pour la sécurité ou la migration des données.

Le code est publié sous [licence MIT](LICENSE). Les contributions proposées pour inclusion suivent cette licence.
