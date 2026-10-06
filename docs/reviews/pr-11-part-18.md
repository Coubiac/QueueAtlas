# Revue PR #11 — partie 18 : politique zéro et lacunes

4 octobre 2026, référence `4e2c1fa890cc7778edb0303e66ac28412287c440`.
Coordinateur et auditeur indépendant : branche zéro de préparation, préfixe retenu,
diagnostic de lacune et tests. Exécution inchangée au lot documentaire 57.

Aucun blocage concret identifié. Replay zéro seulement opt-in pour une unique
génération following au courant configuré, identité physique et préfixe non vide
concordants, checkpoint/ancre zéro canoniques. Nil, multiple, unknown, archives
distinctes restent bloqués. Cleanup sur erreur/annulation, observation finale connue,
préfixe ReadAt revérifié sans seek avant transfert et aux polls jusqu'au premier
checkpoint positif acquitté ; aucune registration/acquisition répétée.

Gap seulement après localisation terminée absent/different ; insufficient,
ambiguous, limite et erreurs filesystem distincts. Diagnostic fixe sans chemin,
contenu ou offsets, errors.Is/As et code de sélection conservés. Aucune preuve de
suppression ou quantification de perte, aucun fallback/reset/retrait. Restauration
revérifiée sur la reprise suivante avec provenance/checkpoint conservés.

Coordinateur et auditeur : TestZeroReplayPrefix* et TestFollowLocationError*,
-count=1 Windows réussis ; diff propre. Intégrations Linux relues : acquisition
avant premier record puis arrêt/replay explicite/positif/ajout strict, preuves
incomplètes/modifiées, annulation/remplacement, lacune 1–2 fichiers/restauration,
diagnostic distinct du courant absent et absence d'historique, cleanup sans fuite.
Exécutées par la [CI de référence](https://github.com/Coubiac/QueueAtlas/actions/runs/37221985374)
verte, pas localement. Code inchangé ; aucun nouveau test sans défaut concret.
CI de publication à consulter sur #11.

Limites : fenêtres 4096 octets et observations non atomiques, écritures sérialisées.
Audit assisté par agents, pas certification humaine externe. PR en brouillon.
Prochain : configuration/démarrage Run, puis validation finale et fusion.
