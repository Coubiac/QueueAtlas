# Revue du chantier de continuité — lots 120–122

## Périmètre et résultat

Relecture locale du diff de la PR #28 au lot122 : deux API pures et onze tests
synthétiques, sans modification des parseurs, sources, projections ordinaires
ou du stockage. Aucun défaut bloquant identifié. Cette relecture assistée
n'est pas une approbation humaine indépendante.

Le lot120 contrôle des attestations explicites ; le lot121 versionne les clés
candidates sous leur contexte. Le lot122 documente les vérifications, les limites
et la clôture. Il n'ajoute pas un troisième comportement de corrélation.

## Points contrôlés

| Point | Résultat de la relecture |
| --- | --- |
| Provenance | Validation du snapshot entier avant plan ; références exactes, extrémités incluant inconnus/NOQUEUE, même source et instance configurée |
| Bornes | Limite de faits existante, au plus 256 frontières ; parcours itératif, sans récursion pilotée par les entrées |
| Contradictions | Doublons, branchements, jonctions et cycles refusés ; aucune sortie partielle |
| Fraîcheur | Révision de tous les faits obligatoire ; aucun plan arbitraire accepté comme jeton de validation |
| Identité | Faits et attestations cadrés, tri canonique, domaines distincts pour faits, plan et clés contextuelles |
| Propriété | Listes de sortie copiées, références/pointeurs des générations détenus ; observations et attestations immuables pendant l'appel |
| Réserves | Générations et ordinaux inchangés, dates incertaines/NOQUEUE/non résolu conservés, CrossStreamUncertain jamais effacé |
| Erreurs | Messages fixes, pas d'origine ou de contenu du journal dans les erreurs, résultat entièrement vide sur refus |

Le plan ne valide pas physiquement une rotation. Un appelant peut donner une
affirmation fausse mais cohérente. Aucune fin définitive de fichier, couverture
complète, absence de fichier intermédiaire, authentification du producteur ou
déduplication inter-source n'est établie. Les hashes versionnent les entrées.
Le contrat exclut les attestations entre sources différentes. La borne en nombre
de faits/frontières ne borne pas séparément les bytes de métadonnées.

## Vérifications déjà acquises

- Lot120 : six tests Windows, suite de corrélation, vet/format/diff réussis ;
  code16784eb, CI37537740519 entière réussie.
- Enregistrement120 ec56b35 : CI37537911013 entière réussie.
- Lot121 : cinq tests Windows, suite de corrélation, vet/format/diff réussis ;
  code720a054, CI37540913337 entière réussie.
- Enregistrement121 114357d46b2862d95f82287c0fb49a7431f7d6e0 :
  CI37541123250 entière réussie, trois jobs et SHA exact vérifiés à la reprise122.

Le runtime n'est pas changé au lot122. Les tests locaux déjà scellés ne sont pas
relancés ; contrôle du diff documentaire puis CI requise pour la nouvelle tête.
Clôture122 publiée016268366bfe309d684cad1387807181ef095384,
CI37543845063 entière réussie ; [PR #28](https://github.com/Coubiac/QueueAtlas/pull/28)
fusionnée sur7ec6dd737681f4af878b8cdc4c8b71de0deb4308.
CI push main37543982877 entière réussie sur le merge exact. Branche distante
supprimée automatiquement et branche locale supprimée après fast-forward propre.

## Limite de la clôture et suite M3

La clôture concerne le contrat et les clés sous attestations, pas une production
automatique de preuves de collecte. Aucun consommateur des claims n'est ajouté
à BuildProjection/SQLite et aucune fusion d'origines n'est autorisée. Cette limite
reste visible ; elle ne doit pas être comptée comme une continuité prouvée livrée.

Le cadrage MVP autorise les logs incomplets avec réserves et exige l'absence de
fausses associations/succès. La voie actuelle reste donc conservatrice. Une
intégration future de preuves de collecte fiables nécessitera producteur,
revalidation et règles de fusion propres, à estimer avant développement.

La [matrice123](../m3-exit-checklist.md) relie les critères aux vérifications.
Le premier contrôle d'intégration manquant124 concerne le conflit de résultats à
date égale après persistance et reopen. Ne pas répéter les mêmes contrôles du
corpus sans risque nouveau.
M3 demeure ouvert pour ce bilan, les vérifications/mesures d'ensemble et sa revue
finale. Les estimations sont révisées dans docs/avancement.md.
