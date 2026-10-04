# ADR-010 — départ initial explicite en fin de fichier

Statut : contrat retenu le 5 octobre 2026 ; capture implémentée au lot 66,
intégration configuration/registration/Run développée au lot 67, CI/fusion à vérifier.

## Décision

Le cadrage prévoit `start_at: end` en option explicite. La valeur par défaut reste
le début. Cette option s'appliquera seulement au bootstrap du chemin : parcours
durable complet sans aucune origine historique, puis sélection physique absente.
Un checkpoint existant, un état unknown, des retired ou un budget incomplet ne
peuvent jamais autoriser un saut à EOF. Les reprises et nouveaux fichiers de
rotation conservent leurs règles actuelles et démarrent une nouvelle origine à zéro.

Le départ choisi est la taille du descripteur lors de sa capture, sans déplacer sa
position. Au plus les 4096 derniers octets sont lus pour l'ancre. LF final exigé
(CRLF accepté) ; sinon `ErrInitialEndPartial` arrête le démarrage sans écriture.
Pas de saut d'une ligne partielle, d'interprétation de son suffixe ou de recherche
non bornée du LF précédent. Les appends ultérieurs ne déplacent pas la frontière.
Fichier vide : attendre comme aujourd'hui, sans enregistrer d'empreinte vide ;
le départ initial reste zéro. Après cette attente, les ajouts sont lus depuis zéro,
sans recapturer leur EOF pour les sauter. Le mode end ne s'applique qu'une fois.

L'origine et ce checkpoint positif initial sont enregistrés dans la même
transaction, sans observation. La transition d'acquisition restera distincte.
Le checkpoint rend explicite le préfixe volontairement ignoré ; aucune promesse
d'ingestion de l'historique précédant ce départ. Une preuve de reprise positive
sera vérifiée comme toute autre ancre. ACK perdu/erreur/cancel ne donnent pas lieu
à compensation ni retry automatique ; un unknown durable reste à récupérer.

## Découpage et vérification

1. Lot 66 : capture privée bornée, tests portables (frontière, position, bornes,
   troncature/erreur/cancel) et contrat. Aucune modification de Run.
2. Lot 67 : option copiée/validée, absence historique démontrée, registration
   initiale/Run, tests Linux avec SQLite (historique ignoré, append ingéré, reprise
   et rotation non sautées, refus partiel, erreur/ACK perdu).
3. Clôture : revue intégrée, CI verte de tête et fusion du chantier cohérent.

## Application en bibliothèque

`Config.StartAt` accepte `StartAtBeginning` (`beginning`) ou `StartAtEnd` (`end`),
valeur vide résolue en beginning par New ; valeur inconnue refusée avant ouverture.
PrepareFollowResume rend sur Absent le nombre Examined incluant les retired :
Run ne transmet end au bootstrap que si ce nombre vaut zéro. Un Absent avec
historique retired utilise le début, comme un nouveau fichier de rotation.

La sélection physique habituelle reste obligatoire : Unique/RestartZero rendent
le checkpoint vérifié existant ; insuffisant/ambigu/limite restent bloquants.
Même avec un chemin sans historique, Different conserve registration à zéro,
jamais un saut à EOF. Seule l'absence physique complète autorise la capture end.
L'attente après un bootstrap vide passe le mode local en beginning, sans modifier
la configuration ; une nouvelle invocation redécide à partir de l'état durable.

Tests lot 67 : config/défaut/copie et scan retired portables ; SQLite Linux pour
bootstrap/end, append et reprise positive, vide puis append, partial/unknown sans
Commit, historique retired/physique différent/rotation, erreurs/EOF/ACK perdu/cancel.
La récupération explicite du lot 64 permet de traiter un unknown après registration
end durable sans ACK ; Run conserve ensuite le checkpoint positif, sans le resauter.

## Limites

Observations de taille et contenu non atomiques ; empreintes bornées, écritures
sérialisées par source, preuves physiques persistantes Linux. CLI/configuration
du service et métriques viendront aux jalons suivants. Le refus du EOF partiel est
délibéré ; une future politique pour l'ignorer devra conserver une frontière
durable explicite. MIT et AD/OIDC après MVP inchangés.
