# Revue PR #11 — partie 17 : préparation orchestrée stricte

4 octobre 2026, référence `4e2c1fa890cc7778edb0303e66ac28412287c440`.
Coordinateur et auditeur indépendant : PrepareFollowResume strict et tests ; helpers
réutilisés des parties 10–14. Zéro explicite, détail de lacune et Run séparés.

Aucun blocage concret identifié. Source/chemin/dépendances/budgets validés avant
lecture d'état ; parcours complet borné puis localisation entière/réouverture stricte
avant observation. Absent seulement après absence complète de following ; unknown,
invalide, capacité, limite ou localisation non unique bloquent sans fallback.
Après ouverture, erreur/annulation ferme l'ensemble, rend un résultat vide et joint
les erreurs de cleanup. Ready remet le propriétaire complet sans lecture de lignes,
normalisation, Sink, mutation d'état ou transfert au scheduler.

Coordinateur et auditeur : deux TestPrepareFollowResume* -count=1 Windows réussis,
19 sous-cas ; diff propre. Intégrations Linux relues : 1–2 connus/nouveau vide/non vide,
positions/checkpoints conservés, aucun nouveau courant ouvert, refus filesystem et
observation/annulation/cleanup joints. Tests Linux exécutés par la
[CI de référence](https://github.com/Coubiac/mailtrace/actions/runs/37221985374) verte,
pas localement sous Windows. Code inchangé,
aucun nouveau test sans défaut concret. CI de publication du rapport sur #11.

Limites : namespace fourni par le lecteur, écritures sérialisées, pages/recherches
et preuves non atomiques, fenêtres d'empreintes bornées. Audit assisté par agents,
pas certification humaine externe. Prochain : zéro explicite et diagnostic de lacune.
