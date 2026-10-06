# Revue PR #11 — partie 15 : transfert connu et courant absent

4 octobre 2026, référence `8bcc1a7bd137155248235cefbec3477b81bcd9cd`.
Coordinateur et auditeur agent indépendant, lecture seule des branches known/missing
de FollowOpened/applyOpened, tests portables et Linux. Données synthétiques.

Aucun blocage concret identifié. Garde partagée avec Run ; décision, chemin,
origine et identité de chaque source recontrôlés avant transfert. Refus conserve
propriétaire et statut. Missing canonique reobservé, diagnostic fixe, sans lecture
de données, attente, ouverture de journal ni écriture durable ; réapparition exige
une nouvelle décision. Observation Windows peut utiliser un handle metadata temporaire.

Known vide la collection avant le scheduler, qui prend immédiatement la propriété
et ferme les fichiers sur erreur/annulation ; contrôle taille/ancre avant consommation.
Ingesteurs et checkpoints conservés, pas de registration/acquisition répétée. Close
du propriétaire vidé est inoffensif. Causes Sink, dont EOF, conservées sans réessai.

Coordinateur et auditeur : cinq tests portables TestFollowOpened*/TestFollowMissing*
-count=1 réussis sous Windows ; diff propre. Intégrations Linux relues : ajout tardif
sur 1–2 fichiers, ordre du courant inversé, retrait du seul ancien, disparition/retour
et checkpoints SQLite conservés, fermeture sans fuite. Exécutées par la
[CI de référence](https://github.com/Coubiac/QueueAtlas/actions/runs/37221589000) verte,
pas localement. Aucun changement d'exécution ni nouveau test sans défaut reproduit.
CI de publication à consulter sur #11.

Limites : contrôles non atomiques, état/set sérialisés, preuves bornées. Les fenêtres
d'annulation juste avant/après transfert sont relues (ctx.Err avant ; cleanup détenu
avant ctx.Err après), sans injection ciblée dans ces tests. Audit assisté par agents,
pas certification humaine externe. PR en brouillon ; prochain lot : nouveau courant
et préparation après transfert. Préparation orchestrée, zéro/lacune et Run restent.
