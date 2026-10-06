# Revue de la PR #11 — partie 1 : lecture et preuves de reprise

Revue du 4 octobre 2026 sur la tête publiée
`1968fc494c5191cf6ad8ea4cc9b11f792f2c85ea`, basée sur main après fusion de #10.
Coordinateur et auditeur agent indépendant en lecture seule, dans le checkout
géré détaché sur cette référence. Données synthétiques uniquement.

## Périmètre et résultat

Contrat Source/Sink et provenance de Record ; reader.go, ingestor.go,
identity*.go, anchor.go, resume.go et leurs tests. Aucun blocage concret identifié
par les deux revues. Les commentaires du contrat Sink précisent l'acquittement
atomique, le retry identique après erreur (y compris réponse perdue) et la propriété
des données. Record précise ses bornes et le comptage du suffixe écarté.
Aucun comportement d'exécution modifié ; pas de régression ajoutée sans défaut
concret à reproduire.

Garanties relues :

- Lecture avec mémoire bornée ; séparateurs LF/CRLF et offsets physiques conservés.
  Une ligne incomplète à EOF n'est pas émise ; ses fragments survivent aux erreurs
  et à l'annulation. Une ligne trop longue attend LF et conserve son offset total.
  Le dépassement int64 devient une erreur persistante du lecteur.
- Normalisation sur une copie, origine et SourceID imposés par l'ingestor.
  Pas de lecture de ligne suivante avant acquittement ; retry avec même Raw,
  ReadAt, observation et ancre. Position exposée limitée au dernier acquittement.
  Test SQLite du commit réussi avec réponse perdue : aucun doublon de record/event.
- L'ancre est construite avec les octets consommés, sans intégrer les octets lus
  d'avance par bufio ou relire un fichier réécrit pour préparer le checkpoint.
- Inspect utilise un descripteur régulier. Préfixe et ancre sont bornés à 4096 octets,
  les formats persistés à 128 caractères, et les encodages sont canoniques.
  ReadAt ne déplace pas la position de lecture.
- VerifyCandidate exige identité physique, préfixe non vide, checkpoint cohérent,
  ancre positive et frontière LF. État incomplet : insufficient ; preuves changées :
  different ; échec de lecture : erreur. Zéro est strict par défaut ; la politique
  explicite peut seulement produire restart_zero avec ses preuves requises.

## Vérifications

Auditeur : tests ciblés FileSource sous Windows avec `-count=1`, diff propre.
Coordinateur : tests FileSource et vet des sources sur la référence isolée réussis.
Vérifications finales locales : go test ./..., go vet ./... et git diff --check
réussis. [CI Linux du lot](https://github.com/Coubiac/QueueAtlas/actions/runs/37187398041)
réussie sur `3fe6a7063f88f2c396e18b8a386ebf95a2cd2cc2` : Go 1.26.x/stable,
tests/vet, détecteur de courses FileSource et builds Linux amd64/arm64 sans CGO.
Les tests d'identité Linux et du vérificateur positif y sont exécutés ; la
compilation seule sous Windows ne prouverait pas leur résultat.

## Limites et suite

Cette revue ne valide pas toute la PR #11. Sélection de génération, registration,
ouverture des chemins, rotation, reprise orchestrée/Run et lifecycle SQLite v2
restent à relire. La PR reste en brouillon.

Le lecteur suppose une position initiale à une frontière de ligne ; l'ingestor
suppose une décision de génération déjà justifiée par l'appelant et des accès
reads/seeks et écritures d'état sérialisés. NewIngestor ne constitue pas à lui seul
une politique de reprise. Les preuves bornées ne couvrent pas tout le fichier,
et les observations filesystem ne sont pas atomiques. La reprise persistante
avec device/inode est actuellement Linux seulement. Audit assisté par agents,
sans certification humaine externe ni preuve d'absence de toute erreur.
