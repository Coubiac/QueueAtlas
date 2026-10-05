# Contrat d'exploitation de l'ingestion

État du socle M2 : bibliothèque Go, pas encore de commande import, service,
doctor ou exporter de métriques. Ce guide décrit les décisions disponibles,
leurs preuves et les limites à préserver lors de l'intégration dans l'application.

## Avant d'exécuter une source

- Protéger les parents des journaux, de SQLite et des copies temporaires.
- Sérialiser les écritures d'une source entre objets/processus ; la garde de
  chaque objet ne protège pas les autres instances.
- Utiliser le même stockage autoritatif pour StateReader et Sink. Un nil du
  Sink acquitte tous les éléments du batch ; une erreur ne prouve pas rollback.
- Donner à chaque source un ID stable. Le host déclaré dans syslog reste une
  observation ; l'instance de confiance vient de TrustedHost ou, à défaut, de l'ID.
- Donner aux tentatives d'import des IDs positifs globalement uniques, stables
  pour les retries/redémarrages. Une nouvelle demande utilise un nouvel ID.

## Suivi continu FileSource

| Situation | Décision du socle | Limite à conserver |
| --- | --- | --- |
| Premier démarrage sans historique | Beginning ; end seulement si demandé et EOF à frontière LF complète | Vide attend puis consomme les futurs ajouts depuis zéro ; end n'est pas hérité par rotations/reprises |
| Checkpoint positif unique vérifié | Reprendre la génération prouvée | Identité, préfixe et fenêtre d'ancre bornés ; aucune preuve de l'intégralité des octets antérieurs |
| Checkpoint zéro existant | Refus strict ou replay explicitement autorisé dans le cas admis | Une ancre vide seule ne prouve pas le fichier ; pas de replay implicite d'ensembles multiples |
| Lifecycle unknown | Refus de Run ; récupération explicite du courant unique si preuves suffisantes | Transition seule, pas de données/CP avancés ; recovery et autorisation de replay zéro sont distinctes |
| Scan incomplet, ambigu ou limité | Décision requise ; aucune génération choisie par le plus grand offset ou la date | Ni gap confirmé ni réinitialisation automatique |
| Génération following introuvable ou différente après scan complet | Gap de continuité vérifiée refusée | Ne prouve ni suppression ni quantité d'octets perdus |
| Courant absent après transfert | Ancien descripteur encore suivi ; observation missing | Absence du path n'implique pas à elle seule perte ou arrêt de lecture |
| Troncature ou ancre changée | Arrêt et dégradation signalés | Pas de saut/reset/compensation implicite |
| Rotation et écritures tardives | Deux générations ouvertes au plus, grâce et retrait acquitté | Un ajout après le dernier contrôle/retrait peut être manqué ; retired ne prouve pas absence d'ajouts futurs |

Les preuves de Device/Inode utilisées par le suivi sont disponibles sur Linux.
Les tests Windows des chemins ne certifient pas le fonctionnement du suivi
continu sur Windows lorsque cette identité persistante est indisponible.

## Import historique

| Étape ou situation | Décision du socle | Limite à conserver |
| --- | --- | --- |
| Construction | Liste explicite et ordre fourni, encodage normal/gzip choisi, IDs distincts, limites positives | Pas de glob, tri implicite ou encodage déduit du nom |
| Run | Nombre <= MaxFiles <= 1000, une deadline pour toute la liste, une copie détenue au plus | Parent plus court respecté ; deadline coopérative, syscall bloqué non interrompu |
| Préparation | Lire un fichier régulier, valider tout le gzip jusqu'à EOF/CRC, copier et hasher les mêmes octets décompressés | Taille/ratio/compressed bornés ; échec ne produit aucune copie exploitable |
| Binding | Identité de contenu source-scopée et CP partagé vérifiés sur la copie privée avant association | Path/inode/en-tête gzip seuls ne prouvent pas le contenu ; copropriétaires/mutations interdits |
| Application | Record complet, CP et manifest dans le même batch | Ligne trop longue : préfixe borné unknown, offsets/ancre sur la ligne physique entière |
| Erreur Sink ou ACK perdu | Arrêt ; même batch en mémoire réessayé avant lookup/ouverture au Run suivant | Aucun nil/EOF du lecteur ne remplace l'ACK ; pas de compensation terminale |
| Perte de process | Recharger le manifest/CP durable et revalider le contenu entier | Pending mémoire perdu ; entrée changée ou CP non concordant refusés |
| Interruption pendant préparation | Running repris ultérieurement | Pas de faux complete ni terminalisation de l'interruption |
| Préparation non interrompue refusée | Failed tracé pour tentative sans contenu | Une erreur de cette trace conserve son propre batch de retry |
| Suffixe partiel | Failed au dernier LF acquitté | Suffixe ni normalisé ni compté comme ingéré |
| Même run complete | Succès sans rouvrir l'entrée | L'ID représente la tentative passée ; nouveau path/contenu demandé = nouvel ID |
| Nouveau run, contenu identique renommé/recompressé | Même origine de contenu et CP partagé prouvé | Pas de nouvelle interprétation des faits déjà durables sous d'autres hypothèses de date |
| Cleanup refusé | Erreur jointe, résidu signalé | Suppression de fichier/directory propre seulement, jamais récursive ; pas de compensation du complete durable |

La copie est éphémère et détenue par Attempt jusqu'à Close ; son parent protégé
et ses octets immuables restent requis. Les protections de bits Unix sont testées ;
les ACL Windows ne sont pas vérifiées. L'ordre des lectures n'est pas une
projection canonique des messages : cette reconstruction relève de M3.

## Dates, doublons et chevauchements

Le parser conserve les faits et hypothèses de date fournis. Sans année/zone
connues, aucun instant UTC n'est inventé. Une reprise sous un contexte différent
n'écrase pas les observations déjà commitées. Deux lignes identiques à des offsets
distincts restent deux records ; le hash d'une ligne n'est pas une identité.

L'idempotence est établie pour une provenance source/génération/offset vérifiée,
et pour le contenu entier réimporté au sein d'une même source import. Des sources
différentes conservent des origines et observations indépendantes, même quand
TrustedHost, raw et offsets concordent. L'import et FileSource ne disposent
actuellement d'aucun protocole pour certifier un lien de provenance entre eux.

**Leur chevauchement reste incertain.** Ne pas supprimer ou fusionner des faits
inter-source sur la seule égalité textuelle, Queue ID, Message-ID, chemin ou date.
La future application devra exposer cette incertitude ; aucune déduplication
inter-source prouvée, étiquette Web ou résolution automatique n'est livrée ici.
Ces critères de l'issue #5 restent ouverts avec la CLI et la reconstruction M3.

## Diagnostics sans données sensibles

Diagnose(LastPathStatus, erreur) retourne un PathStatus validé, une cause fixe et
les rapports Missing/Gap/Degraded, sans chemin, ID, offset ou chaîne d'erreur.
Une observation peut être ancienne et la composition n'est pas un snapshot
atomique. Les tags des dépendances sont des rapports, pas des autorisations de
reprise ni des certificats de perte. Un contexte pur n'est pas dégradé ; un vrai
échec joint au contexte reste signalé. Un arbre d'erreur incomplet/malformé
reçoit Failure error ; les rapports déjà rencontrés restent séparés.

Ce vocabulaire pourra alimenter doctor/métriques à valeurs fixes. Aucun compteur
ou endpoint de métriques n'existe encore. Ne pas mettre adresses, Queue IDs,
Message-IDs, chemins ou chaînes d'erreur dans les labels.

## Preuves et critères encore ouverts

- FileSource/rotation/recovery/end : rapports des PR #11–13, tests Linux et CI
  scellés ; limite retired zéro et fenêtres non atomiques dans ADR-009.
- Copie/manifest/import ordonné : rapports #14–17, reprise après ACK/interruption,
  CRC et budgets, propriété et cleanup ; CI main après #17 réussie.
- Diagnostic : quatre tests Windows et correctif de repli reproduit/relu au lot86.
- Intégration parser/import/SQLite du lot87 : test portable normal/gzip réussi sous
  Windows ; intégration FileSource/inter-source réservée Linux, CI à vérifier.
- CLI hors service actif, métriques exportées, projection canonique indépendante
  de l'ordre, preuve/affichage des chevauchements et paquet pilote : non livrés.
  Les issues #4/#5 restent ouvertes tant que ces critères ne sont pas intégrés.

Voir le [point de reprise](reprise.md) pour les commits/CI réellement vérifiés
et [l'avancement](avancement.md) pour les fourchettes de travail restant.
