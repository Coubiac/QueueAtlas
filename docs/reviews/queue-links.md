# Revue du chantier indices et liens de file

## Lot100 : indices natifs, sans arc

Résultat attendu : conserver des indices typés SMTP/local/bounce avec preuve native,
jamais créer une file depuis un ID distant ni fusionner sur Message-ID ou relay.
BuildQueueHints réutilise bornes/refus/index de PartitionFacts. IsQueueID expose
uniquement la grammaire existante du parser. Premier statut/réponse exacts, formats
limités et identifiant borné exigés ; champs historiques/HTML/chevrons/suffixes
trompeurs ne deviennent pas preuves. SMTP n'a aucune instance cible de confiance,
même loopback ou file du même nom présente. Local/bounce restent des indices.

Quatre tests QueueHints et suites correlation/parser Postfix, vet/diff Windows
réussis : corpus11/12/21/09, preuves/permutations/copies, quinze formats non admis
et quatre valeurs historiques refusés, deux origines, dates inconnues/hôte déclaré,
REMOTE02 non projeté et quatre refus de snapshot sans résultat partiel. Toutes
les observations restent indexées, aucun résultat de remise ou arc changé.
Revue code/docs indépendante sans blocage : quatre tests QueueHints via overlay
isolé Windows pass, root inchangé ; documentation alignée, aucun rerun. Publication/
CI100 à vérifier. Données synthétiques ; aucune configuration du serveur déduite
des manuels ou du relay.
