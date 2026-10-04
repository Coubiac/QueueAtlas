# Premières issues à publier après validation

Ces fiches décrivent les [issues #1 à #8 publiées sur GitHub](https://github.com/Coubiac/mailtrace/issues). Le [cadrage de phase 0](phase-0-proposal.md) fixe les contrats communs. Chaque PR doit indiquer la fixture utilisée, le résultat des tests et les limites restantes.

## 1. [Corpus synthétique de référence](https://github.com/Coubiac/mailtrace/issues/1)

**Label :** `area:qa`, `priority:P0` — **Jalon :** M0 — **Dépendance :** validation des identifiants et du format de fixture.

Créer sous `testdata/` des extraits Postfix synthétiques couvrant les 25 scénarios du cahier des charges : entrant/sortant/interne, plusieurs destinataires, rejet RCPT et NOQUEUE, deferred puis sent, plusieurs retries, bounce/expired, réinjection et changement d'ID, Rspamd, Dovecot, logs partiels/hors ordre, deux hôtes, rotation/restart/import/gzip et données hostiles. Chaque scénario décrit les faits attendus **et les conclusions interdites**. Utiliser `example.org` et les plages IP de documentation ; aucun log de production.

**Acceptation :** chaque fixture a un nom, un court manifeste de provenance et un résultat attendu par destinataire ; deux files avec même Queue ID sur hôtes distincts restent distinctes ; deux messages avec le même Message-ID restent distincts ; les payloads hostiles sont présents mais non exécutables.

## 2. [Enveloppes syslog et parseurs Postfix](https://github.com/Coubiac/mailtrace/issues/2)

**Labels :** `area:parser`, `priority:P0` — **Jalon :** M1 — **Dépendance :** issue 1.

Implémenter la séparation enveloppe syslog / message Postfix. Les parseurs purs couvrent `smtpd`, `pickup`, `cleanup`, `qmgr`, `smtp`, `lmtp`, `local`, `virtual`, `pipe`, `bounce`. Conserver hôte déclaré, identité de source, date brute/normalisée et qualité de datation, service, PID, Queue ID optionnel, `to`, `orig_to`, DSN, relay, délais, réponse SMTP intacte et ligne originale. Une ligne non reconnue devient `unknown` sans panique.

**Acceptation :** tests table-driven sur fixtures, passage d'année et date sans fuseau, réponse SMTP contenant virgules/`=`/parenthèses, champ malformé et ligne très longue ; fuzz du parseur sans crash ni allocations non bornées.

## 3. [Migration SQLite v1 et contrat de stockage](https://github.com/Coubiac/mailtrace/issues/3)

**Labels :** `area:storage`, `priority:P0` — **Jalon :** M1/M2 — **Dépendance :** ADR-004 revu.

Créer migration versionnée pour sources, origines de fichiers, checkpoints, observations, instances de file, liens, destinataires, tentatives et rejets pré-file. L'événement brut garde une identité stable de provenance ; les projections sont recalculables. Indexer les champs réellement proposés par l'API. Configurer WAL sur disque local, `synchronous=FULL`, `foreign_keys=ON` et `trusted_schema=OFF` par connexion. Refuser une version de base inconnue supérieure.

**Acceptation :** tests migration fraîche et reprise, FK effectives, `PRAGMA integrity_check`, refus version supérieure, transaction rollbackée sans checkpoint avancé, build `CGO_ENABLED=0` Linux amd64 et arm64.

## 4. [Sink transactionnel et FileSource](https://github.com/Coubiac/mailtrace/issues/4)

**Labels :** `area:source`, `priority:P0` — **Jalon :** M2 — **Dépendances :** issues 2 et 3.

Implémenter `Source.Run(ctx, Sink)` et `Sink.Commit(ctx, Batch)`. Le lot porte lignes complètes et curseurs ; l'accusé arrive après commit des observations, projections et checkpoints. FileSource utilise identité de fichier, empreinte et ancre d'offset ; suit un fichier actif et son ancien descripteur après rename/create ; signale missing/gap/degraded. Le premier démarrage part du début sauf `start_at: end` explicite. Limiter lignes, mémoire, lots et descripteurs.

**Acceptation :** crash avant/après commit sans ligne perdue ni doublon persistant, ligne partielle reprise, écriture tardive sur ancien fichier, rotation double pendant arrêt, troncature/copytruncate diagnostiquée, inode réutilisé détecté ou signalé ambigu, permission retirée puis rétablie, métriques de lacune sans PII.

## 5. [Import historique normal et gzip](https://github.com/Coubiac/mailtrace/issues/5)

**Labels :** `area:source`, `priority:P0` — **Jalon :** M2 — **Dépendances :** issues 3 et 4.

Ajouter `queueatlas import <files...>` hors service actif pour le MVP. Accepter uniquement les fichiers réguliers et gzip en flux, gérer manifest `import_run`, empreinte du contenu décompressé et offsets de reprise. Limiter taille décompressée, ratio, temps et longueur de ligne. Vérifier le checksum gzip à EOF avant succès. Réutiliser parser et Sink ; reconnaître les chevauchements prouvés avec FileSource et déclarer ceux qui restent incertains.

**Acceptation :** import répété identique sans doublon, renommage/recompression reconnue, reprise après interruption, gzip corrompu en erreur explicite, bombe gzip stoppée par les bornes, ordre d'import différent donnant la même projection canonique.

## 6. [Corrélation Postfix révisable](https://github.com/Coubiac/mailtrace/issues/6)

**Labels :** `area:correlation`, `priority:P0` — **Jalon :** M3 — **Dépendances :** issues 1–5.

Produire QueueInstances par `(instance, queue_id, génération)`, tentatives et état par destinataire, NOQUEUE hors file et graphe de relations typées. Une réinjection ou un transfert interhôte n'est confirmé que sur preuve corroborée ; un `queued as` sans log cible reste candidat. Un import tardif déclenche recalcul déterministe. Préserver événements et tentatives, même si le résumé change.

**Acceptation :** fixtures multi-recipient partiel, deferred→sent, ID réutilisé, Message-ID dupliqué, deux hôtes, bounce, NOQUEUE et logs manquants ; jamais de « livré en boîte » sur simple `smtp sent`, jamais de faux succès global sur résultat partiel.

## 7. [Authentification, API et rendu sûr](https://github.com/Coubiac/mailtrace/issues/7)

**Labels :** `area:security`, `area:web`, `priority:P0` — **Jalon :** M4 — **Dépendances :** issues 3 et 6, ADR-006 validé.

Ajouter compte administrateur local créé par CLI, sessions révocables, limitation des essais, CSRF des mutations et protection commune API/UI. Écoute loopback par défaut ; refuser une exposition distante non sécurisée. API de recherche bornée et paramétrée avec pagination stable ; vues de recherche, timeline, tentatives et logs bruts via `html/template`. Ne jamais interpréter un champ SMTP comme HTML/URL active.

**Acceptation :** 401 sur chaque route sensible non authentifiée, test d'en-tête proxy forgé, expiration/logout, recherche SQL hostile, limites de période/taille, XSS testé dans un navigateur réel de la source jusqu'au rendu, en-têtes CSP/`no-store`/anti-framing et accessibilité clavier de base.

## 8. [Authentification Active Directory et OIDC (release future)](https://github.com/Coubiac/mailtrace/issues/8)

**Labels :** `area:security`, `area:identity`, `priority:P1` — **Jalon :** après M5 — **Dépendances :** auth locale MVP et ADR-008 détaillée avec l'environnement AD cible.

Ajouter un fournisseur OIDC générique testé avec Keycloak et un mode de connexion Active Directory protégé par TLS, direct LDAP(S) ou via fédération OIDC selon l'environnement retenu. Conserver le compte local de secours. Le modèle de compte distingue `provider_id` et identifiant stable du sujet ; l'adresse email ne sert pas de clé ni de preuve de rôle. Les groupes/claims sont mappés explicitement aux rôles `viewer` et `admin`, avec refus par défaut.

**Acceptation :** login et logout OIDC avec découverte, code + PKCE, vérifications `state`/`nonce`/issuer/audience/signature ; rotation de clés et fournisseur indisponible testés ; AD sans liaison en clair et avec validation du certificat ; groupe retiré provoquant perte d'accès ; collision de deux comptes ayant le même email sans fusion ; secrets absents des logs et de l'API.
