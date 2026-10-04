# Revue de la PR #11 — partie 10 : classification des états de reprise

Revue du 4 octobre 2026 sur la tête publiée
`848aa0694591e735ff9153d43b68e404aa8c8582`. Coordinateur et auditeur agent
indépendant en lecture seule. Périmètre : LoadFollowOrigins, ses tests et dépendance
LoadPathOrigins pour le parcours complet et les copies. Recherche de rotations,
localisation, réouverture et Run exclus. Données synthétiques uniquement.

## Résultat

Aucun blocage concret identifié par les deux revues. Code inchangé et aucun nouveau
test sans défaut concret à reproduire. Comportement conforme à ADR-009.

- La classification commence après le parcours complet d'une source et d'un chemin
  enregistré exacts. Le budget de 1 à 1000 états inclut les retirés ; pages bornées,
  ordre et curseurs contrôlés. Un parcours limité ne propose pas un sous-ensemble.
- Les retirés sont exclus. Sur un parcours complet, priorité invalide, inconnu,
  capacité, absence, ensemble complet. Au plus deux following sont exposés sur
  complete seulement ; ID, date et offset ne choisissent pas le courant.
- Limite, page invalide, erreur du lecteur ou annulation n'exposent aucun candidat
  partiel. Les erreurs conservent leur cause et annulent tout le résultat.
- Métadonnées copiées, checkpoints nil et zéro conservés sans inférence ni preuve
  inventée. Empreintes et ancres sont encore non vérifiées. Le résultat ne prouve
  ni présence des fichiers, ni capacité finale avec un nouveau courant, ni reprise sûre.
- Aucun journal ouvert et aucun état écrit. L'intégration SQLite utilise un chemin
  de journal inexistant et vérifie les lectures répétées sans mutation.

## Vérifications

Coordinateur : `go test ./internal/source/file -run '^TestLoadFollowOrigins' -count=1`
réussi sous Windows. Auditeur : TestLoadFollowOrigins* et TestLoadPathOrigins*
réussis, intégration SQLite incluse. git diff --check réussi ; checkout isolé propre
et référence confirmée.

Couverture relue/exécutée : vide/retirés/1–2 following/capacité, invalides et unknown
avec priorités, nil/zéro et copies indépendantes, dernière page inconnue au-delà
d'un budget réduit, erreur/page incohérente/annulation. SQLite : 101 états dont
99 retirés et deux following, limite 100 refusée, isolation source et état inchangé.

La [CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37204843549)
a réussi : tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64
sans CGO et job Windows chemins. Ce lot modifie uniquement la documentation ;
consulter la PR #11 pour la CI de publication du rapport.

## Limites et suite

Filtrage par source assuré par le lecteur, OriginState ne contenant pas de source ID.
Pages sans snapshot global : écritures à sérialiser pendant parcours et application.
Nombre d'états borné, pas taille globale de leurs chaînes ni coût SQL mesuré.
Ces garanties reprennent le contrat déjà revu des lecteurs d'état (partie 4).

PR #11 en brouillon. Prochain petit lot : SelectRotation, scan du répertoire,
exclusions, budgets, preuves et fermeture des descripteurs temporaires. Localisation
de l'ensemble, réouverture/transfert, préparation orchestrée et Run seront relus
séparément. Audit assisté par agents, sans certification humaine externe.
