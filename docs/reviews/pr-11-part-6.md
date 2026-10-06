# Revue de la PR #11 — partie 6 : acquisition avant première ligne

Revue du 4 octobre 2026 sur la tête publiée
`55c820b0ae31387506a1754e97ec8d0edfa40762`, basée sur main après fusion de #10.
Coordinateur et auditeur agent indépendant en lecture seule. Périmètre :
runOpened, prepareGeneration et acquireGeneration de file_source.go, tests
acquisition*.go et propriété des successeurs lors de leur préparation. Données
synthétiques uniquement ; pas de revue globale du scheduler dans ce lot.

## Résultat et garanties relues

Aucun blocage concret identifié par les deux revues. Code inchangé et aucun nouveau
test sans défaut concret à reproduire.

- prepareGeneration applique la décision de génération/politique zéro. Un fichier
  neuf vide attend sans registration ni acquisition ; une décision inapplicable
  retourne un diagnostic typé, sans ingestor utilisable.
- NewIngestor vérifie identité/préfixe et checkpoint avant acquisition. Il prépare
  la lecture au checkpoint, mais aucune normalisation ni consommation de ligne
  n'a lieu avant acquittement de la transition. Les ReadAt bornés de vérification
  précèdent cet acquittement ; ce n'est pas une promesse d'absence de lecture physique.
- unknown/retired passent explicitement à following. following réutilise l'état
  durable ; toute autre valeur est refusée. Le helper d'acquisition dépend de la
  vérification et de la décision de son appelant, ce n'est pas un vérificateur isolé.
- Le batch d'acquisition porte seulement l'identité configurée et une transition.
  Origine, offset, ancre, provenance et observations ne sont ni remplacés ni remis
  à zéro. Une génération retirée est revérifiée avant réacquisition explicite.
- Un retour d'erreur du Sink, y compris EOF, arrête la préparation ; aucun retry ou
  premier record automatique. Annulation avant acquittement : arrêt. Annulation
  après acquittement : arrêt avec état following durable conservé, sans faux retrait.
  Une erreur avec réponse perdue ne prouve pas que la transition n'est pas durable.
- runOpened possède et ferme son descripteur sur erreur ou attente. Après succès,
  la propriété passe au suivi. Un successeur est ajouté à la collection possédée
  par le scheduler avant préparation ; le nettoyage couvre son erreur d'acquisition.
  Les erreurs de fermeture sont jointes à l'erreur initiale.

## Vérifications

Coordinateur : TestAcquisitionRejectsInvalidStateAndHonorsCancellation -count=1
réussi sous Windows, git diff --check réussi. Auditeur : même test et
TestRunOpenedClosesOwnedDescriptorOnCancellation réussis sous Windows ; checkout
isolé propre et référence confirmée.

Tests Linux d'intégration relus : acquisition avant premier record pour neuf,
checkpoint positif, following, retired et zéro explicitement permis ; provenance
conservée ; erreur/EOF/conflit/annulation avant et après acquittement sans
normalisation ; préfixe changé après registration refusé avant acquisition ;
fichier vide fermé sans commit ; échec du successeur sans réessai ni fuite.
Ces tests Linux ne s'exécutent pas localement sous Windows.

La [CI de la référence revue](https://github.com/Coubiac/QueueAtlas/actions/runs/37203133730)
a réussi : tests/vet Linux Go 1.26.x/stable, race FileSource, builds Linux amd64/arm64
sans CGO et job Windows ciblé chemins. Les intégrations Linux y sont exécutées.
Ce lot modifie uniquement la documentation ; consulter la PR #11 pour la CI de
publication du rapport.

## Limites et suite

Sink durable et écritures/réessais sérialisés par source requis. Vérifications
filesystem et transaction d'acquisition non atomiques. Registration et acquisition
restent deux transactions : arrêt entre elles peut laisser unknown ; après
acquisition avant première ligne, checkpoint zéro exige sa politique explicite.
Ce helper ne valide pas à lui seul la reprise globale de tels états dans Run.

Pas de crash réel ou nouveau scénario concurrent injecté dans ce lot. Retrait/grâce,
rotation et reprise orchestrée/Run restent à relire ; PR #11 en brouillon. Prochain
petit lot : retrait durable après EOF stable/grâce, protections des partiels/pending,
acquittement avant fermeture et nettoyage sur erreur. Audit assisté par agents,
sans certification humaine externe.
