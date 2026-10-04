# Revue PR #11 — partie 16 : nouveau courant au transfert

4 octobre 2026, référence `c5e928fb762e7445c67f16ea57557e5c5bc2c1ca`.
Coordinateur et auditeur indépendant : branche new de applyOpened,
prepareNewAndFollow et tests. Données synthétiques ; aucun blocage identifié.

Capacité avant ouverture, reobservation/source puis préfixe zéro éventuel,
taille/ancre ancien avant OpenLog. Identité du nouveau comparée à l'observation ;
refus/annulation ferme le temporaire et conserve l'ancien propriétaire/statut.
Après transfert, préparation/acquisition avant les lignes, cleanup de tous sur
échec et scheduler seul responsable après succès. Courant vide non enregistré
avant contenu ; ancien ingesteur réutilisé. Échec après registration sans acquisition
laisse unknown/zéro, sans faux retrait ou ingestion ; récupération encore différée.

Coordinateur et auditeur : TestFollowNew* -count=1 portable Windows réussi, huit
sous-cas ; diff propre. Intégrations Linux relues : courant vide/non vide, acquisition
avant ligne, écritures anciennes tardives, registration/acquisition/EOF/annulation
avant et après acquittement, checkpoint insuffisant et fermeture. Exécutées par la
[CI de référence](https://github.com/Coubiac/mailtrace/actions/runs/37221815044) verte,
pas localement. Code inchangé, aucun nouveau test sans défaut concret reproduit.
CI de publication à consulter sur #11.

Limites : observations non atomiques et preuves bornées, état/set sérialisés.
Acquisition séparée de registration, unknown reste bloquant au redémarrage.
Audit assisté par agents, pas certification humaine externe. PR en brouillon.
Prochain lot : orchestration PrepareFollowResume stricte ; zéro/lacune et Run séparés.
