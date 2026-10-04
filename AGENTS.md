# QueueAtlas — travail par petits lots

Le propriétaire demande des lots courts pour faciliter la reprise après une
interruption de quota Codex. Cette préférence s'applique à la suite du projet.

- Lire `docs/reprise.md` au début d'une reprise, puis vérifier l'état Git.
- Par défaut, une demande « continue » correspond à un seul petit lot : un
  comportement précis, ses tests utiles et sa documentation immédiate.
- Découper un jalon en plusieurs lots ; éviter de réunir stockage, source,
  corrélation et interface dans le même lot.
- Définir le résultat attendu avant les modifications. Si le périmètre grossit,
  reporter les comportements indépendants dans les lots suivants.
- Terminer le lot par les vérifications adaptées, un commit et la mise à jour
  de `docs/reprise.md`. Publier le commit sur la branche de travail lorsque
  l'accès GitHub est disponible, puis vérifier la CI déclenchée.
- Réutiliser la PR du chantier tant que son périmètre reste cohérent ; ne pas
  créer une PR pour chaque petit commit. Respecter la dépendance des PR empilées.
- Le point de reprise indique ce qui est terminé, les vérifications réellement
  faites, les limites et la prochaine action concrète. Si un lot est interrompu,
  distinguer explicitement le travail incomplet du dernier état validé.
- Réutiliser les décisions, lectures et résultats déjà consignés. Relancer un
  contrôle lorsqu'une modification ou un risque précis le justifie.

Le cadrage technique est dans `docs/phase-0-proposal.md` et `docs/adr/`.
Les données de test doivent rester synthétiques. QueueAtlas conserve la licence
MIT ; l'authentification Active Directory et OIDC (notamment Keycloak) est prévue
après le MVP.
