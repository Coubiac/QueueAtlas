# Revue PR #11 — partie 14 : observation du courant

4 octobre 2026, référence `c7623d21c63e3c2ba0d21dd0ddbd84564bd3b8df`.
Coordinateur et auditeur agent indépendant en lecture seule ; ObserveCurrent et
tests, hors transfert/Run. Données synthétiques uniquement.

Aucun blocage d'exécution identifié. Tous les descripteurs sont inspectés, IDs et
identités distincts exigés avant comparaison au chemin absolu. known suit l'identité,
pas l'ordre ; missing n'invente aucun courant ; new exige une place et capacity
n'expose pas d'identité utilisable. Erreur/annulation : résultat vide, propriétaire
et positions/grâce inchangés. Aucun record, état durable ou transfert modifié.

Commentaire et ADR-009 précisés : Windows ouvre/ferme un handle temporaire de
métadonnées via ObservePath, sans lecture de données ni fermeture des fichiers détenus.
Aucun changement d'exécution ni nouveau test sans défaut concret à reproduire.

Coordinateur et auditeur : quatre TestObserveCurrent* -count=1 réussis sous Windows ;
git diff --check réussi, checkout isolé propre. Tests Linux réouverture réelle,
missing/capacité/retour du retenu, symlinks/FIFO et cleanup relus, pas exécutés localement.
[CI de référence](https://github.com/Coubiac/mailtrace/actions/runs/37221250736) verte :
tests/vet Linux Go 1.26.x/stable, race FileSource, builds amd64/arm64 sans CGO et
Windows chemins ; intégrations Linux exécutées. CI de publication à consulter sur #11.

Limites : snapshots non atomiques, known ne prouve pas la continuité du checkpoint,
écritures/set à sérialiser et décision à recontrôler avant application. PR en brouillon.
Prochain lot : transfert du courant connu et refus du courant absent, conservation
de propriété avant transfert, fermeture exclusive après. Nouveau courant, préparation
orchestrée et Run restent à relire. Audit assisté par agents, sans certification humaine externe.
