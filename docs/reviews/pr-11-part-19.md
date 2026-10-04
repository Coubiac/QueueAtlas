# Revue PR #11 — partie 19 : configuration et démarrage Run

4 octobre 2026, référence `c1782f7fab72555ddf3e0d490ce1f92e2ff0bf7c`.
Coordinateur et auditeur indépendant : New/Run/runPreparedResume/runOpened et tests,
composants déjà revus réutilisés. Aucun blocage concret identifié.

Configuration copiée et chemin résolu une fois, intervalles/grâce/budgets bornés
et defaults validés sans I/O. PathStateReader requis avant journal. Garde unique
pendant préparation/application/attente ; erreurs libèrent la garde. Statut remis
à zéro seulement après acceptation Run, même si la préparation bloque.

Ready applique le cœur commun sous cette garde, ferme le propriétaire sur refus
avant transfert et laisse le scheduler nettoyer après transfert. Seul Absent
complet démarre le courant ; aucune décision bloquante convertie en fallback.
Courant vide fermé avant attente cancellable, rouvert/reconsidéré au passage
suivant. Toute erreur filesystem/selection/Sink, dont EOF, arrête sans retry.

Coordinateur et auditeur : tests New/RunOpened -count=1 Windows réussis ; test
concurrent explicitement ignoré sous Windows (identités persistantes Linux), pas
présenté comme exécuté. Intégrations Linux relues : 1–2 connus/nouveau vide/non vide,
checkpoint sans replay/acquisition répétée, blockers sans mutation/fallback, refus
avant transfert/cleanup, attente vide/cancel et garde concurrente. Exécutées par la
[CI de référence](https://github.com/Coubiac/mailtrace/actions/runs/37222118286) verte,
pas localement. Code inchangé ; diff propre, aucun nouveau test sans défaut concret.
CI de publication du rapport à consulter sur #11.

Limites : garde par objet, écritures à sérialiser par source entre objets,
preuves/pages non atomiques. Pour un courant retired à zéro via démarrage Absent,
le préfixe est vérifié avant acquisition, sans préfixe retenu aux polls. La protection
persistante de partie 18 concerne following zéro. Différence conforme au contrat
actuel, explicitée dans ADR-009 ; élargir cette garantie serait un comportement séparé.
Audit assisté par agents, pas certification humaine externe. Prochain : synthèse,
CI de tête, passage prêt et fusion #11 ; récupération unknown/import encore à faire.
