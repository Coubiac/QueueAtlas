# Revue de la PR #11 — partie 2 : décision de génération

Revue du 4 octobre 2026 sur la tête publiée
`c8a5b46a03289819f4b5d53208b94dd8377ee0a5`, basée sur main après fusion de #10.
Coordinateur et auditeur agent indépendant en lecture seule dans le checkout
géré détaché sur cette référence. Données synthétiques uniquement.

## Périmètre et résultat

selection.go, generation.go et leurs tests, y compris fichier vide et politique
explicite de relecture à zéro. Aucun blocage concret identifié par les deux
revues. Aucun changement de code ni nouveau test sans défaut concret à reproduire.

Garanties relues :

- Requête dans le namespace SourceID configuré, avec device/inode du descripteur.
  Page limitée à 100 états et au budget restant ; au plus 1 000 candidats examinés.
  IDs strictement croissants, identité physique cohérente et curseur égal au dernier
  ID fourni. Une continuation vide ou une page répétée est une erreur fixe.
- Une sélection applicable exige un parcours terminé, une seule correspondance et
  aucune preuve insuffisante. Deux correspondances suffisent à refuser l'unicité.
  Limite, insuffisance, ambiguïté, erreur ou annulation ne fournissent aucun candidat.
  Le checkpoint retenu est copié ; aucune préférence par date, ID ou offset.
- Un candidat positif et un candidat zéro explicitement autorisé sont concurrents.
  La politique zéro ne transforme pas les preuves absentes ou invalides en preuve
  suffisante. Une reprise existante retourne son état sans écriture.
- La registration exige une sélection complète absent/different et un préfixe
  non vide. Origine avec ID aléatoire et checkpoint zéro canonique sont soumis au
  même Commit, sans record. Created et State sont fournis après acquittement seulement.
  Erreur du Sink : aucun état retourné comme acquitté.
- Un préfixe capturé vide diffère la registration sans écriture. Après append,
  l'appelant refait toute la décision ; une ancienne origine aux preuves insuffisantes
  n'est pas remplacée. Les opérations ne déplacent pas la position de lecture.

## Vérifications

Coordinateur et auditeur : tests portables ciblés sous Windows avec `-count=1`
réussis ; auditeur : checkout et diff restés propres. Commande du coordinateur :

```powershell
go test ./internal/source/file -run 'Test(ResumeSelection|SelectResume|EnsureGeneration|ZeroReplayPolicy)' -count=1
```

Ces tests Windows vérifient notamment pagination, limites, copies, erreurs,
annulation et refus sur plateforme sans identité persistante. Les tests marqués
Linux de registration SQLite et de preuves réelles ne s'exécutent pas sous Windows.
La [CI Linux de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37187511562)
a réussi : Go 1.26.x/stable, tests/vet, détecteur de courses FileSource et builds
Linux amd64/arm64 sans CGO. Ce lot modifie uniquement la documentation ; consulter
la PR #11 pour le résultat de la CI déclenchée par la publication du rapport.

## Limites et suite

Les pages ne forment pas un instantané global ; l'appelant doit sérialiser les
écritures de la source pendant la sélection et son application. Les captures
filesystem restent bornées et non atomiques. Un lecteur doit respecter le
namespace et la fin de pagination annoncés par son contrat.

EnsureGeneration ne garde pas un batch pending de registration pour un retry
automatique. Après un commit durable dont l'acquittement est perdu, la nouvelle
décision stricte peut être insuffisante au checkpoint zéro. L'appelant doit s'arrêter
et appliquer la politique de reprise prévue ; cette revue ne valide pas une
récupération automatique de cet état. Registration et acquisition restent deux
transactions, comme décrit dans ADR-009.

Chemins/ouverture, rotation, orchestration Run et lifecycle SQLite v2 restent hors
périmètre. Aucun jugement global sur la PR #11, qui reste en brouillon. Prochain
petit lot : ouverture et observation du chemin (open*.go, path.go, source_path.go).
Audit assisté par agents, sans certification humaine externe.
