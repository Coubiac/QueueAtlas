# ADR-002 — Frontend serveur

Statut : accepté le 3 octobre 2026.

## Contexte
L'interface consiste surtout en recherche, tableaux et timeline ; elle affiche des champs SMTP hostiles.
## Options
`html/template` + HTMX local et JavaScript minimal ; frontend compilé intégré via `go:embed`.
## Décision
Pages `html/template` fonctionnelles sans JS ; HTMX local embarqué pour les fragments.
## Raisons
Chaîne de build courte et rendu contextuellement échappé, adapté aux écrans prévus.
## Conséquences
Tester tout le trajet log→DB→HTML dans un navigateur ; bannir `template.HTML` pour les données. Versionner les assets intégrés.
## Limites
Les interactions riches pourraient justifier un frontend compilé après mesure d'un besoin réel.
