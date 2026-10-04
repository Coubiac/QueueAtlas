# Point de reprise QueueAtlas

Mis à jour le 5 octobre 2026. Ce fichier décrit le dernier état connu ; vérifier
Git et GitHub avant de modifier une branche ou de fusionner une PR.

Vue des jalons et estimation des lots restants : [avancement](avancement.md).
Prévision après le lot 65 : quatre jalons à clôturer (M2 en cours, M3–M5), environ
50–90 petits lots jusqu'au MVP, dont 8–16 pour M2. PR #12 fusionnée après cinq lots
de récupération explicite ; chantier suivant start_at:end : environ trois lots,
inclus dans M2. Fourchettes de planification incertaines ; AD/OIDC reste après MVP.

## Avancement du jalon M2

M1 (parseurs), socle SQLite M2 et FileSource sont fusionnés dans main. PR #11 clôturée
au lot 60, synthèse de 19 parties et CI de tête verte. Chantier suivant : récupération
explicite d'un unknown courant unique, lots 61–65, PR #12 fusionnée et CI main verte.
Opération de bibliothèque seule, sans CLI/service. Lot 66 : capture bornée du
départ initial à EOF et ADR-010 ; configuration/registration/Run au prochain lot.
Critère de fin : refus sûrs, preuves, transition seule et réessai/ACK vérifiés,
CI verte et PR du chantier fusionnée. M2 inclut les autres décisions encore ouvertes
et l'import historique ; corrélation, API/Web et distribution sont des jalons suivants.

## État validé

- Lot 66 : capture privée d'EOF complet implémentée, trois tests portables Windows,
  suite/vet/diff et revue indépendante réussis ; publication/CI à vérifier.
  Run inchangé, aucun saut à EOF encore activable. Prochain : intégration initiale.
- Lot 65 terminé : [PR #12](https://github.com/Coubiac/mailtrace/pull/12) fusionnée
  avec tête `ca5db281d776e9e5c27874386212d78f520eedbc`, CI 37242452694 verte.
  Main `69dfe6bb056e4de326f2d77fc8aaf3433c2bf8b3` vérifié ;
  [CI push main réussie](https://github.com/Coubiac/mailtrace/actions/runs/37242571458),
  vérifiée via API REST publique. Branche suivante `codex/m2-start-at-end` depuis main.
- Lot 65 : synthèse indépendante sur `31520075b7fc9ad18a30c23ace1a7c3dd356d96b`,
  aucun blocage identifié ; anciens composants/dépendances/CI inchangés, diff propre.
  CI 37242113816 verte, Linux réellement exécuté ; clôture documentaire prête,
  publication/CI finale et fusion attendues, pas encore constatées dans ce commit.
- Lot 64 : RecoverUnknownCurrent développé/relu, test portable onze cas, suite/vet/diff
  et compilation Linux réussis. Application seule sans ingestion, ACK/réessai couvert
  par tests Linux exécutés en CI 37242113816 verte. Prochaine : validation/fusion #12.
- Lot 63 : préparation privée du courant/propriétaire/preuves implémentée et relue,
  deux tests ciblés Windows, suite/vet/diff/compilation Linux réussis ; CI à vérifier.
  Aucune application ou opération publique. Prochaine : RecoverUnknownCurrent.
- Lot 62 : LoadRecoveryOrigin implémenté/relu indépendamment, trois tests ciblés
  Windows, go test ./..., go vet ./... et diff réussis. Rapport PR #12 ; aucune
  ouverture/preuve/transition encore. Prochaine : ouverture et preuves du courant.
- Lot 60 terminé : [PR #11](https://github.com/Coubiac/mailtrace/pull/11) fusionnée
  dans main sur `27b9d98bba1f51749f10e8b9930e9c4da0302068`, après CI finale
  37222534218 verte sur `12eba5a`. Résultat GitHub et origin/main vérifiés.
  [CI main réussie](https://github.com/Coubiac/mailtrace/actions/runs/37222669898),
  vérifiée via l'API REST publique (le connecteur ne listait pas le run push).
- Lot 61 terminé : contrat standalone RecoverUnknownCurrent cadré et relu
  indépendamment, aucun comportement encore implémenté. Branche
  `codex/m2-unknown-recovery` depuis ce main ; documentation/estimation mises à jour.
  Diff propre ; [CI de publication réussie](https://github.com/Coubiac/mailtrace/actions/runs/37241041999),
  commit `51933e0d03f00ff7b3896f993cd7eaf22486c403`, PR #12 en brouillon.
- Lot 59 : New/Run/configuration relus sur `c1782f7`, sans blocage concret.
  Deux tests portables Windows réussis, concurrence réservée Linux/CI verte.
  Rapport partie 19 ; code inchangé. Limite retired zéro explicitée dans ADR-009.
  Prochaine étape : synthèse finale, CI de tête, prêt et fusion #11.
- Lot 58 : zéro following courant unique et diagnostic gap relus sur `4e2c1fa`,
  exécution inchangée au lot 57. Deux tests ciblés Windows réussis, CI Linux verte.
  Rapport partie 18 ; code inchangé. Prochaine revue : New/Run/configuration.
- Lot 57 : PrepareFollowResume strict relu sur `4e2c1fa`, sans blocage concret.
  Deux tests portables Windows (19 sous-cas) réussis ; intégrations Linux/CI verte.
  Rapport partie 17, code inchangé. Prochaine revue : politique zéro et lacune.
- Lot 56 : nouveau courant et préparation après transfert relus sur `c5e928f`, sans
  blocage identifié. Test portable Windows (huit sous-cas) réussi ; intégrations
  Linux relues/CI verte. Rapport partie 16, code inchangé. Prochaine : PrepareFollowResume.
- Lot 55 : transfert known et refus missing relus sur `8bcc1a7`, sans blocage concret.
  Cinq tests portables Windows réussis, intégrations Linux relues/CI de référence
  verte. Rapport partie 15 ; code inchangé. Prochaine revue : nouveau courant.
- Lot 54 : ObserveCurrent relu sur `c7623d2`, sans blocage d'exécution ; commentaire
  et ADR précisés pour les handles metadata Windows. Rapport partie 14, quatre tests
  ciblés Windows réussis. Prochaine revue : transfert connu et refus missing.
- Lot 53 : réouverture/propriété de l'ensemble localisé relues sur
  `a04afbb42fa93a678a4316017ba8b019f0a70894`, sans blocage concret identifié.
  Rapport `docs/reviews/pr-11-part-13.md`, code inchangé et tests portables Windows
  réussis. Prochaine revue : observation du courant parmi les descripteurs.
- Lot 52 : localisation de l'ensemble en suivi relue sur
  `e95f84ecc53096bd8af11d67356bf78075f9a103`, sans blocage concret identifié.
  Rapport `docs/reviews/pr-11-part-12.md`, code inchangé et tests ciblés Windows
  réussis. Prochaine revue : réouverture/propriété de l'ensemble localisé.
- Lot 51 terminé : substitutions Windows du répertoire et des entrées
  aux IDs différés corrigées dans SelectRotation/checkRotationEntry. Deux
  régressions échouant avant puis passant après, cas inchangé réussi, patch relu
  indépendamment, tests/vet Windows et compilation Linux réussis. Correctif
  `de81c4e` publié, CI 37220202856 verte avec les trois nouveaux tests Windows.
  Rapport `docs/reviews/pr-11-part-11.md`. Prochaine revue : LocateFollowOrigins.
- Lot 50 : classification des états durables de reprise relue sur
  `848aa0694591e735ff9153d43b68e404aa8c8582`, sans blocage concret identifié.
  Rapport `docs/reviews/pr-11-part-10.md`, code inchangé. Tests ciblés et intégration
  SQLite réussis sous Windows. Prochaine revue : recherche de rotation SelectRotation.
- Lot 49 terminé : starvation sur ligne partielle très longue corrigée
  dans le scheduler (budget 64 Kio par génération/passage). Régression échouant
  avant puis passant après ; relecture indépendante, tests/vet Windows et
  compilation des tests Linux réussis. Correctif `cd931e0` publié, CI 37204731135
  verte avec nouveau test conjoint Linux et race. Rapport `docs/reviews/pr-11-part-9.md`.
  Prochaine revue : classement des états durables de reprise.
- Lot 48 : courant et successeurs aux polls relus sur
  `82635d7e27fa3da5447a6e23fd27925a82f417aa`, sans blocage concret identifié.
  Rapport `docs/reviews/pr-11-part-8.md`, code inchangé et tests portables réussis.
  Prochaine revue : ordonnancement des lectures et polls sous flux continu.
- Lot 47 : retrait durable après EOF et délai de grâce relu sur
  `f468a7ad6e2327be58ce0c074b29b1491a03fed3`, sans blocage concret identifié.
  Rapport `docs/reviews/pr-11-part-7.md`, code inchangé et tests portables réussis.
  Prochaine revue : sélection du courant et ouverture des successeurs aux polls.
- Lot 46 : acquisition par FileSource avant première ligne relue sur
  `55c820b0ae31387506a1754e97ec8d0edfa40762`, sans blocage concret identifié.
  Rapport `docs/reviews/pr-11-part-6.md`, code inchangé. Prochaine revue : retrait/grâce.
- Lot 45 : migration SQLite v2 et transitions durables relues sur
  `a0c0d13319a27c38ffad8054cc6b081fbec76299`, sans blocage concret identifié.
  Rapport `docs/reviews/pr-11-part-5.md`, tests ciblés Windows réussis, code inchangé.
  Prochaine revue : acquisition par FileSource avant première ligne.
- Lot 44 : lecteurs d'état SQLite relus sur
  `9007404264ba9b4a1a99a8563ce0fb9ab2155f40`, sans blocage d'exécution identifié.
  Commentaire Checkpoint corrigé (absence distincte de zéro, sans reprise implicite).
  Rapport `docs/reviews/pr-11-part-4.md`. Prochaine revue : migration/transitions v2.
- Lot 43 : ouverture/observation relues et substitution Windows corrigée sur
  `cded0a87fd22f96220d3f519cb70b85f55f2cf3b`, CI Linux et nouveau job Windows verts.
  Rapport `docs/reviews/pr-11-part-3.md`. Prochaine revue : lecteurs d'état SQLite.
- Lot 42 : sélection et registration initiale relues, sans blocage identifié,
  sur `c8a5b46a03289819f4b5d53208b94dd8377ee0a5`. Rapport
  `docs/reviews/pr-11-part-2.md`, code inchangé. Tests ciblés Windows réussis et
  CI Linux de cette référence verte. Prochaine revue : ouverture/observation du chemin.
- Lot 41 : revue partielle FileSource (contrats, lecteur, ingestor et preuves de
  reprise) terminée, sans blocage identifié. Rapport `docs/reviews/pr-11-part-1.md`.
  Commit `3fe6a7063f88f2c396e18b8a386ebf95a2cd2cc2` publié, CI Linux réussie.
  PR #11 reste en brouillon ; prochaine revue : sélection et registration.
- Nom QueueAtlas, licence MIT et conception de phase 0 approuvés.
- M1 : enveloppes syslog, parseurs Postfix, corpus synthétique et tests/fuzz.
  [PR #9](https://github.com/Coubiac/mailtrace/pull/9), branche
  `m1-parser-foundation`, revue terminée et correctif des senders entre guillemets
  `0c224a213d3148cef866f72f48e28fd0ced26a78`. Fusionnée dans main le 4 octobre,
  commit `b52e7fb78022049146d87eb69de002fc239e0f12`.
- Socle M2 : migration SQLite v1, contrat `Source`/`Sink`, insertion idempotente
  des observations et commit atomique des checkpoints.
  [PR #10](https://github.com/Coubiac/mailtrace/pull/10), branche
  `codex/m2-sqlite-storage`, relue et corrigée sur
  `463d767418cb366b87aaf983530a42da6bba2f35`, tête finale
  `44e54f7b3b551ec5d073a616b18a67842026ad76`. Fusionnée dans main le 4 octobre,
  commit `69ab91e6a830914102ac96f4b9434be89f5702e6`.
- Premier lot FileSource : lecteur de lignes borné dans
  `internal/source/file/reader.go`, commit
  `dfacfa20575529b421d7a6f0dedef8e7f476ddcc`.
  [PR #11](https://github.com/Coubiac/mailtrace/pull/11), branche
  `codex/m2-file-source`, désormais basée sur main après fusion de la PR #10.
- Deuxième lot FileSource : identité physique et empreinte de début bornée dans
  `internal/source/file/identity*.go`, sur la même branche et dans la PR #11.
- Troisième lot FileSource : interface de lecture `source.StateReader` et
  implémentation SQLite de la recherche paginée des origines et checkpoints.
  Toujours dans la PR #11, sans migration ni dépendance ajoutée.
- Quatrième lot FileSource : ancres bornées de checkpoint et lecture stricte
  des formats persistés, dans `internal/source/file/anchor.go`.
- Cinquième lot FileSource : vérification d'un candidat de reprise dans
  `internal/source/file/resume.go`, commit
  `489c2dc5a2f067d7d7a1ca8271b32fb88f8a0ff4`, toujours dans la PR #11.
- Sixième lot FileSource : sélection bornée d'un candidat unique dans
  `internal/source/file/selection.go`, commit
  `362817ad349a3bd0f438e8e7fd84cb7040ac2721`, toujours dans la PR #11.
- Septième lot FileSource : enregistrement initial et retour d'une reprise
  vérifiée dans `internal/source/file/generation.go`, commit
  `9902546d1a1b342b9f834a96afe5842863203fe2`, toujours dans la PR #11.
- Huitième lot FileSource : ingestion d'une ligne et acquittement de son
  checkpoint dans `internal/source/file/ingestor.go`, commit
  `2d3c7d7ab72f07b0bbfccef2932cceb7c2531e07`, toujours dans la PR #11.
- Neuvième lot FileSource : attente annulable des ajouts sur une génération
  ouverte, dans `internal/source/file/follow.go`, commit
  `719d5139c345e4eede483a4646eabc172e162833`, toujours dans la PR #11.
- Dixième lot FileSource : décision d'attente sans écriture pour une génération
  neuve vide, dans `generation.go`, commit
  `90e4efcb6a0a8bc63f1b02f911618c5e2a01ade5`, toujours dans la PR #11.
- Onzième lot FileSource : politique explicite de relecture à zéro dans les
  vérificateur/sélecteur/décision de génération, commit
  `a879283d0de1b632dc7d9960662eb1ef6b1ca591`, toujours dans la PR #11.
- Douzième lot FileSource : ouverture en lecture seule et vérification du
  chemin dans `internal/source/file/open*.go`, commit
  `6d5f79471a63f03437787ae24559c075a86a09b5`, toujours dans la PR #11.
- Treizième lot FileSource : démarrage de `source.Source.Run`, décision de
  génération, attente initiale et suivi du descripteur choisi dans
  `internal/source/file/file_source.go`, commit
  `100669637fbfe219bcc5b1e5ce8dd9dcb7cb0a90`, toujours dans la PR #11.
- Quatorzième lot FileSource : observation du chemin courant par rapport au
  descripteur conservé dans `internal/source/file/path.go`, commit
  `b72c9f2fa2a3209fed2ce5368fa3cc499b7a0a11`, toujours dans la PR #11.
- Quinzième lot FileSource : observation au polling de `Run`, statut consultable
  et maintien du descripteur pendant une absence temporaire du chemin, dans
  `internal/source/file/source_path.go`, commit
  `d1e4a1257f11e85ebec200d8217a4d79b24cda8b`, toujours dans la PR #11.
- Seizième lot FileSource : bascule vers un remplacement régulier en conservant
  l'ancien descripteur/ingesteur, avec limite de deux fichiers dans
  `internal/source/file/rotation.go`, commit
  `bc1b12db4478633b206908b9db1027ddc3eb3ef2`, toujours dans la PR #11.
- Dix-septième lot FileSource : suivi conjoint des deux générations, une ligne
  par ingesteur et par passage, avec commits sérialisés et récupération des
  écritures tardives dans `rotation.go`, commit
  `a389cc8895fefdbea70fb4059a7ecd98ed361b11`, toujours dans la PR #11.
- Dix-huitième lot FileSource : période de grâce configurable après EOF stable,
  protection des lignes partielles/batches pending et libération des descripteurs
  dans `internal/source/file/grace.go`, commit
  `84cfae72db4ce4642588563d2f3cad57e25a2dc3`, toujours dans la PR #11.
- Dix-neuvième lot FileSource : polling périodique entre les passages de lecture,
  y compris sous flux continu, et attente limitée au temps restant avant contrôle,
  dans `rotation.go`, commit
  `311fe756a5dfd25b14ad4a78dfe4a19f2a809ab8`, toujours dans la PR #11.
- Vingtième lot FileSource : diagnostic fixe de diminution observée sous l'offset
  consommé, incluant les lignes partielles et les fichiers conservés, dans
  `internal/source/file/truncation.go`, commit
  `765e0b13e2483631208fc3ff9de613a72e9c2afb`, toujours dans la PR #11.
- Vingt-et-unième lot FileSource : contrôle au polling de l'ancre du dernier
  checkpoint positif acquitté, diagnostic fixe de non-correspondance et arrêt,
  dans `internal/source/file/live_anchor.go`, commit
  `5fee37fd45ab9848d64e5e3f333505ff7f1acc29`, toujours dans la PR #11.
- Vingt-deuxième lot FileSource : sélection bornée d'une rotation accessible pour
  une origine/checkpoint fournis, dans `internal/source/file/rotation_search.go`,
  commit `f5e50894b4c9fe1e0efb9757f3545baaed9a2756`, toujours dans la PR #11.
- Vingt-troisième lot FileSource : contrat `source.PathStateReader` et lecture
  SQLite des origines/checkpoints filtrés par source et chemin exact dans
  `internal/storage/sqlite/state.go`, commit
  `61c59f763f9499119d56285e3af299b6f4c99de3`, toujours dans la PR #11.
- Vingt-quatrième lot FileSource : parcours complet et borné des états par chemin,
  validation des pages et copies de checkpoints dans
  `internal/source/file/path_origins.go`, commit
  `7b8f9d12d583228c7595674897a81572f364c046`, toujours dans la PR #11.
- Vingt-cinquième lot FileSource (stockage) : ADR-009, état durable inconnu/en
  suivi/retiré, migration SQLite v2 et transitions attendues/idempotentes dans
  `Sink.Commit`, commit `b30b8748a66eabbc14cc9f2cfe271615bef9dbbc`, toujours dans la PR #11.
- Vingt-sixième lot FileSource : acquisition durable après vérification et avant
  consommation de ligne, reprise d'un acquittement existant/réacquisition explicite,
  arrêt sur erreur sans déplacement de checkpoint dans `file_source.go`, commit
  `f1794ab23aaa42aae2ba4fefdbd2884cb908abda`. Fixtures de date corrigées dans
  `f10ea5e0b16d6b9d5a65ee16b35cd3795e1fb2db`, toujours dans la PR #11.
- Validation du lot acquisition : contrôles locaux et
  [CI](https://github.com/Coubiac/mailtrace/actions/runs/37173188977) réussis sur
  `f10ea5e0b16d6b9d5a65ee16b35cd3795e1fb2db`, incluant tests Linux, détecteur de
  courses FileSource et builds Linux amd64/arm64 sans CGO.
- Vingt-septième lot FileSource : retrait durable après EOF stable/grâce et avant
  fermeture/libération de capacité dans `grace.go`, commit
  `baf30a13353d9a16fa6216670529a6789132cdd2`, toujours dans la PR #11.
  [CI retrait](https://github.com/Coubiac/mailtrace/actions/runs/37173662429)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource, builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Vingt-huitième lot FileSource : `LoadFollowOrigins`, préparation bornée des
  candidats durables en suivi à partir des états par chemin, commit
  `addd572898b0095c71952b232d1f5d26e066b710`, toujours dans la PR #11.
  [CI candidats en suivi](https://github.com/Coubiac/mailtrace/actions/runs/37174070307)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Vingt-neuvième lot FileSource : `LocateFollowOrigins`, localisation de l'ensemble
  en suivi avec budget partagé et aucun chemin partiel utilisable, commit
  `10885fd205025ede06eb0be25a3cc3dd15279a0e`, toujours dans la PR #11.
  [CI localisation](https://github.com/Coubiac/mailtrace/actions/runs/37174564943)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Trentième lot FileSource : `OpenFollowLocations` et propriétaire opaque
  `OpenedFollowSet`, réouverture/revérification sans consommation et nettoyage
  complet sur erreur, commit `35fc07be34b38672e6e9586783dc5490dd7719d2`, dans la PR #11.
  [CI réouverture](https://github.com/Coubiac/mailtrace/actions/runs/37175078596)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Lot 31 FileSource : `OpenedFollowSet.ObserveCurrent`, courant identifié par
  métadonnées physiques, absence/nouvelle génération/capacité explicites sans
  modification des fichiers détenus, commit `3f3d72704c658e6fd03accd96883420a2be34f47`.
  [CI courant](https://github.com/Coubiac/mailtrace/actions/runs/37175634122)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous. PR #11.
- Lot 32 FileSource : `FileSource.FollowOpened`, transfert d'un ensemble rouvert
  avec courant connu au scheduler commun, commit
  `dbf85d443adf0194d4f40a73e5e4552b08450b5a`, toujours dans la PR #11.
  [CI transfert](https://github.com/Coubiac/mailtrace/actions/runs/37176322112)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Lot 33 FileSource : `FollowOpened` ajoute un nouveau courant à une génération
  conservée, ouverture vérifiée avant transfert et préparation/acquisition avant
  toute ligne, commit `c3791358b3c4d6b60c76d894109b1fe9ea9006e7`, dans la PR #11.
  [CI nouveau courant](https://github.com/Coubiac/mailtrace/actions/runs/37176769622)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Lot 34 FileSource : diagnostic `ErrCurrentMissing` au démarrage, recontrôle de
  la décision sans courant arbitraire et propriété conservée, commit
  `14204b35911620242e73325b1f602ca3a34c15af`, toujours dans la PR #11.
  [CI courant absent](https://github.com/Coubiac/mailtrace/actions/runs/37177140539)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Lot 35 FileSource : `PrepareFollowResume`, orchestrateur isolé et borné de
  préparation de reprise, commit `bbbdc66c33547a1703d7dc81f8d32af2704500c9`, PR #11.
  [CI préparation](https://github.com/Coubiac/mailtrace/actions/runs/37177688035)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
  Le raccordement à Run reste le prochain lot.
- Lot 36 FileSource : Run prépare/reprend l'ensemble persisté sous une seule garde,
  PathStateReader requis et budgets configurés, commit
  `ddf00f313195df21c8100833842180efe9dbfb74`, toujours dans la PR #11.
  [CI démarrage Run](https://github.com/Coubiac/mailtrace/actions/runs/37178273467)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Lot 37 FileSource : ErrFollowResumeGap, diagnostic de génération en suivi non
  localisable avec cause de sélection conservée, commit
  `86ff3b1ccac62dba97491309c2f8b5e1ff3419dc`, toujours dans la PR #11.
  [CI lacune de reprise](https://github.com/Coubiac/mailtrace/actions/runs/37178727644)
  réussie (Go 1.26.x/stable, détecteur de courses FileSource et builds Linux
  amd64/arm64 sans CGO), ainsi que les contrôles locaux décrits ci-dessous.
- Lot 38 FileSource : reprise zéro explicite d'une seule génération en suivi
  courante, préfixe revérifié avant transfert et tant que l'acquittement reste zéro.
  Commit `a56bf17815510eb48e1d2562e44055c62c1f7132`, dans la PR #11.
  [CI reprise zéro](https://github.com/Coubiac/mailtrace/actions/runs/37185193175)
  réussie : tests Linux Go 1.26.x/stable, détecteur de courses FileSource et builds
  Linux amd64/arm64 sans CGO ; contrôles locaux également réussis.
- Lot 39 : revue/correctif/clôture de M1. PR #9 fusionnée après revue coordinateur,
  audit par un agent indépendant et CI réussie ; voir docs/reviews/pr-9.md.
  Le correctif est intégré aux branches #10 et #11 en respectant leur dépendance.
- Lot 40 : revue/correctifs/clôture SQLite v1, après audit indépendant et CI.
  PR #10 fusionnée ; correctifs intégrés à FileSource, revue détaillée ci-dessous.
- Seule PR #11 reste en brouillon et cible désormais main. La revue FileSource
  commence par contrats/lecteur/identité, en lots courts, avant clôture d'ensemble.
- Validation du lecteur : `go test ./...` et `go vet ./...` réussis localement.
  [CI du commit de code](https://github.com/Coubiac/mailtrace/actions/runs/37142141635)
  réussie, incluant les builds Linux amd64/arm64 sans CGO.
- Validation locale du lot identité : `go test ./...`, `go vet ./...` et builds
  Linux amd64/arm64 avec `CGO_ENABLED=0` réussis.
  [CI identité](https://github.com/Coubiac/mailtrace/actions/runs/37154817870)
  réussie, notamment les tests d'identité Linux.
- Validation locale du lot lecture d'état : `go test ./...` et `go vet ./...`
  réussis. [CI lecture d'état](https://github.com/Coubiac/mailtrace/actions/runs/37155184130)
  réussie, incluant les builds Linux sans CGO.
- Validation locale du lot ancres : `go test ./...` et `go vet ./...` réussis.
  [CI ancres](https://github.com/Coubiac/mailtrace/actions/runs/37155582363)
  réussie.
- Validation locale du lot candidat : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource pour Linux amd64 sans CGO réussis.
  [CI candidat](https://github.com/Coubiac/mailtrace/actions/runs/37156132714)
  réussie, incluant les cas device/inode sur Linux et les builds sans CGO.
- Validation locale du lot sélection : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI sélection](https://github.com/Coubiac/mailtrace/actions/runs/37156470072)
  réussie, incluant les preuves réelles sur Linux et les builds sans CGO.
- Validation locale du lot génération : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI génération](https://github.com/Coubiac/mailtrace/actions/runs/37156904624)
  réussie, incluant la réouverture SQLite sur Linux et les builds sans CGO.
  La CI du commit documentaire `02c4744` est aussi confirmée réussie.
- Validation locale du lot ingestion : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI ingestion](https://github.com/Coubiac/mailtrace/actions/runs/37157404479)
  réussie, incluant le réessai idempotent SQLite et les builds sans CGO.
- Validation locale du lot suivi ouvert : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI suivi ouvert](https://github.com/Coubiac/mailtrace/actions/runs/37157772435)
  réussie, incluant attente/append/annulation et builds Linux sans CGO.
- Validation locale du lot fichier vide : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI fichier vide](https://github.com/Coubiac/mailtrace/actions/runs/37158062815)
  réussie, incluant attente sans écriture, append et builds Linux sans CGO.
- Validation locale du lot politique zéro : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI politique zéro](https://github.com/Coubiac/mailtrace/actions/runs/37158472028)
  réussie, incluant relecture réelle après réouverture SQLite et builds sans CGO.
- Validation locale du lot ouverture : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI ouverture](https://github.com/Coubiac/mailtrace/actions/runs/37158920606)
  réussie, incluant remplacement/FIFO/symlink Linux et builds sans CGO.
- Validation locale du lot démarrage : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI démarrage](https://github.com/Coubiac/mailtrace/actions/runs/37159624661)
  réussie, incluant démarrage/reprise SQLite, attente initiale/append/annulation,
  concurrence, diagnostics, politique zéro et builds Linux sans CGO.
- Validation locale du lot observation : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI observation](https://github.com/Coubiac/mailtrace/actions/runs/37159970805)
  réussie, incluant rename/create, disparition/réapparition, ancien descripteur,
  liens/FIFO Linux et builds amd64/arm64 sans CGO.
- Validation locale du lot polling : `go test ./...`, `go vet ./...` et compilation
  des tests FileSource Linux amd64 sans CGO réussis.
  [CI polling](https://github.com/Coubiac/mailtrace/actions/runs/37160397278)
  réussie, incluant disparition/append/réapparition SQLite, remplacement, erreurs
  au polling et builds Linux sans CGO. L'étape `go test -race ./internal/source/file`
  est confirmée réussie sur le job Go 1.26.x.
- Validation locale du lot bascule : `go test ./...`, `go vet ./...` et compilation
  des tests FileSource Linux amd64 sans CGO réussis.
  [CI bascule](https://github.com/Coubiac/mailtrace/actions/runs/37161105324)
  réussie, incluant rotations/SQLite, attente vide, capacité, retour d'identité
  conservée, erreurs/fermeture, détecteur de courses et builds Linux sans CGO.
- Validation locale du lot suivi conjoint : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI suivi conjoint](https://github.com/Coubiac/mailtrace/actions/runs/37161595157)
  réussie, incluant append tardif/ligne partielle, équité, sérialisation, checkpoints
  SQLite, successeur vide, erreurs/pending, détecteur de courses et builds sans CGO.
- Validation locale du lot grâce : `go test ./...`, `go vet ./...` et compilation
  des tests FileSource Linux amd64 sans CGO réussis.
  [CI grâce](https://github.com/Coubiac/mailtrace/actions/runs/37162405314) réussie,
  incluant horloge contrôlée, délai/append/capacité réutilisée, partiels/pending,
  chemin absent, détecteur de courses et builds Linux sans CGO.
- Validation locale du lot flux continu : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI flux continu](https://github.com/Coubiac/mailtrace/actions/runs/37163133260)
  réussie, incluant attente restante, rotation avant EOF, expiration en progression,
  Sink EOF prioritaire, détecteur de courses et builds Linux sans CGO.
- Validation locale du lot troncature : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI troncature](https://github.com/Coubiac/mailtrace/actions/runs/37163693457)
  réussie : tests sur Linux (Go 1.26.x/stable), checkpoints SQLite conservés,
  détecteur de courses et builds Linux amd64/arm64 sans CGO.
- Validation locale du lot ancre en suivi : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI ancre en suivi](https://github.com/Coubiac/mailtrace/actions/runs/37164079280)
  réussie : tests sur Linux (Go 1.26.x/stable), checkpoints SQLite conservés,
  détecteur de courses et builds Linux amd64/arm64 sans CGO.
- Validation locale du lot recherche de rotation : `go test ./...`, `go vet ./...`
  et compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI recherche de rotation](https://github.com/Coubiac/mailtrace/actions/runs/37170755564)
  réussie : tests Linux (Go 1.26.x/stable), renommage/preuves/exclusions/fermeture,
  détecteur de courses et builds Linux amd64/arm64 sans CGO.
- Validation locale du lot lecture par chemin : `go test ./...`, `go vet ./...` et
  compilation des tests SQLite Linux amd64 sans CGO réussis.
  [CI lecture par chemin](https://github.com/Coubiac/mailtrace/actions/runs/37171180557)
  réussie : tests Linux (Go 1.26.x/stable), pagination/isolation/littéraux SQL,
  détecteur de courses FileSource et builds Linux amd64/arm64 sans CGO.
- Validation locale du lot parcours des états : `go test ./...`, `go vet ./...` et
  compilation des tests FileSource Linux amd64 sans CGO réussis.
  [CI parcours des états](https://github.com/Coubiac/mailtrace/actions/runs/37171671535)
  réussie : tests Linux (Go 1.26.x/stable), pagination/budget/alias et intégration
  SQLite sur 101 états, détecteur de courses et builds Linux amd64/arm64 sans CGO.
- Validation locale du lot état de suivi : `go test ./...`, `go vet ./...` et
  compilation des tests SQLite Linux amd64 sans CGO réussis.
  [CI état de suivi](https://github.com/Coubiac/mailtrace/actions/runs/37172442015)
  réussie : tests Linux (Go 1.26.x/stable), migration v1/v2 et rollback des
  transitions/checkpoints, détecteur de courses et builds Linux amd64/arm64 sans CGO.
- AD et fournisseur OIDC externe, dont Keycloak :
  [issue #8](https://github.com/Coubiac/mailtrace/issues/8) et ADR-008.

Le suivi des fichiers, l'import, la corrélation, l'authentification locale,
l'interface et les paquets installables restent à développer. Le pilote SQLite
est épinglé à v1.60.1 ; mesures de charge et inventaire complet des notices de
dépendances restent à faire avant distribution. La revue sécurité indépendante
des PR reste à mener.

## Composants FileSource disponibles

`LineReader.Next(ctx)` renvoie une ligne complète avec son séparateur LF/CRLF et
ses offsets physiques. Une ligne partielle à EOF reste en mémoire jusqu'à une
lecture suivante ; une ligne trop longue conserve au maximum 64 Kio, consomme
le suffixe et produit une erreur dans `source.Record` après son séparateur.

Les tests couvrent les offsets non nuls, le seuil exact de taille, EOF répété
puis ajout de données, la ligne trop longue partielle, l'annulation, une erreur
de lecture et le débordement d'offset. Les records retournés restent stables
après les lectures suivantes. Aucun changement au schéma ou aux dépendances.

Limites : le lecteur seul ne remplit pas OriginID, ReadAt et Observation ;
le composant d'ingestion ci-dessous les attribue. Le lecteur ne suit pas les
chemins, ne détecte pas les rotations et ne persiste aucun checkpoint seul.
Une lecture bloquante reste à interrompre par son propriétaire.

## Identité physique des fichiers

`Inspect` valide un fichier régulier à partir du descripteur et expose sa taille
ainsi que device/inode sur Linux. `Identity.SameFile` compare les identités via
`os.SameFile`, y compris après renommage. Les autres plateformes n'exposent pas
de device/inode persistables dans cette version.

`CapturePrefix` lit au maximum 4096 octets avec `ReadAt`, sans déplacer la
position de lecture. `PrefixFingerprint` conserve la longueur et le SHA-256 ;
`Matches` compare exactement cette même fenêtre, même si le fichier a grandi.
Une fenêtre vide ne correspond jamais. `String` encode la longueur avec le
digest (`sha256:<longueur>:<hex>`) pour une future persistance.

Tests : renommage/remplacement, ajout de données, position de lecture inchangée,
préfixe modifié, troncature, fenêtre bornée/vide et refus d'un répertoire. Sur
Linux, le test garde l'ancien descripteur ouvert pendant le renommage ; sur
Windows, il le ferme avant renommage pour respecter le partage des handles Go.

Limites : une identité physique et un préfixe identique ne prouvent pas une
génération ; la réutilisation d'inode et la troncature exigent encore les ancres
de checkpoint et un diagnostic. La capture n'est pas une vue atomique du fichier
pendant une écriture concurrente. Aucune dépendance ou migration ajoutée.

## Lecture des origines et checkpoints persistés

`source.StateReader` définit `FileOrigins` et le lecteur de checkpoint existant.
`OriginQuery` filtre exactement une source, un device et un inode. Les résultats
sont triés par ID et paginés avec `AfterID`/`NextID`, avec 1 à 100 états par page.
L'identité physique vide est refusée, sans recherche large implicite.

Le stockage SQLite renvoie métadonnées et checkpoint optionnel dans une seule
requête pour chaque page. `OriginState.Checkpoint == nil` signifie absent ; un
checkpoint à zéro reste présent. Les empreintes sont renvoyées telles quelles,
sans tentative de sélectionner ou fusionner les générations.

Tests : état absent, réouverture, métadonnées conservées, pagination sans perte,
checkpoints absents/zéro/non-zéro, séparation source/device/inode et annulation.
Limites : les pages successives ne constituent pas un instantané global et
l'ordre des IDs n'est pas chronologique. Le sélecteur ci-dessous utilise cet
ordre uniquement comme curseur. La boucle de suivi reste à développer.

## Ancres et formats persistés

`CaptureAnchor` hache les derniers `min(offset, 4096)` octets avant l'offset,
sans déplacer la position de lecture. Un offset au-delà de EOF ou une fenêtre
partielle produit une erreur. `CheckpointAnchor.Matches` vérifie la même fenêtre
après append et renvoie une non-correspondance si le fichier a été tronqué.

Format canonique de l'ancre : `sha256:<offset>:<longueur>:<hex>`, à enregistrer
dans `Position.AnchorHash`. `ParseCheckpointAnchor` et `ParsePrefixFingerprint`
rejettent encodages malformés, tailles hors bornes, digests invalides et formes
non canoniques. La chaîne est limitée à 128 octets avant décodage.

Offset zéro : ancre valide, longueur zéro, SHA-256 du contenu vide ; `Matches`
renvoie toujours false car elle ne fournit aucune preuve. Les fenêtres bornées
ne détectent pas une modification hors fenêtre et ne prouvent pas à elles seules
la génération du fichier. Ces helpers ne vérifient pas la frontière de ligne.

Tests : fenêtre courte/longue, round-trip, append, position de lecture inchangée,
modification dans/hors fenêtre, troncature, zéro et nombreux formats invalides.
La boucle de suivi reste à développer.

## Vérifier un candidat de reprise

`VerifyCandidate` compare un `source.OriginState` au descripteur ouvert :
identité device/inode, préfixe, ancre, égalité des offsets et séparateur LF
avant le checkpoint. Le chemin enregistré ne sert pas de preuve d'identité.
Le résultat contient un statut et un code de diagnostic fixe, sans contenu
des journaux. La position de lecture n'est pas déplacée.

Une différence d'identité, de contenu ou de taille produit `different`.
Un état malformé, un checkpoint absent/zéro, une empreinte vide, une identité
indisponible ou une frontière de ligne invalide produit `insufficient`.
Les erreurs d'accès au descripteur sont retournées séparément.

Tests : append, chemin enregistré obsolète, position inchangée, changements
d'identité/préfixe/ancre, troncature, états incohérents, frontière de ligne,
absence de preuve, descripteur invalide et répertoire.

Limites : `match` signifie que ces preuves bornées concordent ; les changements
hors fenêtres et les écritures concurrentes ne sont pas couverts. Le contrôle
n'est pas un instantané atomique. L'appelant conserve le namespace de source
fourni par `StateReader`. Aucun choix global ni suivi de fichier n'est ajouté.

## Choisir un candidat unique

`SelectResume(ctx, fichier, sourceID, StateReader)` interroge exactement la
source et l'identité du descripteur, puis réutilise `VerifyCandidate`.
`unique` exige un parcours terminé, une seule correspondance et aucun candidat
aux preuves insuffisantes. Seul ce statut contient une copie de l'origine et
du checkpoint sélectionnés ; aucun choix par date, ID ou offset maximal.

Les autres statuts sont `absent`, `different`, `insufficient`, `ambiguous` et
`limit_reached`. Deux correspondances suffisent pour conclure à l'ambiguïté.
Le parcours examine au maximum 1 000 candidats, dans des pages de 100 au plus,
sans accumuler toutes les origines. Une page incohérente (ordre, curseur,
identité ou taille), une erreur d'accès ou une annulation ne renvoie aucun
résultat partiel sélectionnable. Les plateformes sans device/inode persistable
renvoient `insufficient`, sans requête large de remplacement.

Tests : plusieurs pages, candidat unique avant/après les différences, preuves
insuffisantes bloquant le choix, deux correspondances, copie du checkpoint,
limite exacte/dépassée, pages invalides, erreurs et annulation après une
correspondance. Les tests Linux vérifient aussi la sélection face aux preuves
réelles d'un fichier et la position de lecture inchangée.

Limites : l'appelant doit sérialiser les écritures d'origines/checkpoints de
cette source pendant la sélection et l'application du résultat. Les pages ne
forment pas un instantané global ; le sélecteur n'ajoute ni verrou ni transaction
globale. Les limites des fenêtres et écritures concurrentes du fichier restent
celles de `VerifyCandidate`. Aucun état n'est écrit par ce lot.

## Enregistrer une nouvelle génération

`EnsureGeneration` sélectionne dans le namespace d'une identité de source
`file`. Pour `absent` ou `different`, il capture identité et préfixe du
descripteur. Si le préfixe est non vide, il crée un ID opaque aléatoire de
128 bits puis transmet origine et checkpoint zéro avec ancre canonique dans
un seul `Sink.Commit`, sans record. Une capture vide diffère cet enregistrement.

Le résultat contient l'état et `Created == true` seulement après acquittement.
Pour `unique`, il retourne l'origine et le checkpoint vérifiés sans écrire.
Pour `insufficient`, `ambiguous` ou `limit_reached`, il retourne le diagnostic
avec état absent. Une erreur de lecture, de Sink ou une annulation avant commit
ne produit pas d'état utilisable. Le helper ne déplace pas la lecture.

Tests : validation des entrées, annulation, absence d'identité persistable ;
sur Linux, état initial d'un fichier non vide retrouvé après réouverture
SQLite, origine et checkpoint transmis ensemble, chemin/identité/date conservés,
reprise existante, nouvelle génération après différence, refus des décisions
incomplètes/ambiguës/limitées et erreurs de Sink/lecteur.

Limites : sérialisation de la source à assurer par l'appelant ; captures du
fichier non atomiques. Un arrêt après création mais avant la première ligne
validée laisse un checkpoint zéro qui exige la politique explicite décrite
ci-dessous pour une relecture automatique.
Une ancienne origine capturée vide conserve son empreinte vide immuable et ne
fournit pas de preuve de contenu lors d'une reprise future. Les nouvelles
origines vides sont désormais différées. Leur reprise sans preuve reste à
définir avant de déclarer FileSource terminé.

## Acquitter une ligne complète

`NewIngestor` reçoit l'identité de source, l'état de génération décidé et une
fonction de normalisation injectée. Il vérifie le checkpoint initial, son
contenu/frontière LF, le préfixe et l'identité physique disponible, puis positionne
le descripteur à cet offset. `Position()` expose seulement l'état acquitté.

`CommitNext(ctx, Sink)` envoie une ligne complète et son checkpoint dans un même
batch. Il attribue OriginID, ReadAt et SourceID configuré. Les lignes trop longues
contournent le parser et deviennent des observations `unknown` avec erreur,
tout en validant la consommation physique jusqu'au séparateur.

Le lecteur transmet les fragments consommés à une fenêtre roulante de 4 Kio,
initialisée depuis l'ancre de reprise. Le digest de fin est calculé sur ces
octets, y compris après lecture anticipée ou pour les suffixes de lignes trop
longues ; il n'est pas recalculé depuis un fichier éventuellement réécrit.

EOF partiel n'écrit rien et conserve la ligne en cours. En cas d'erreur du Sink
ou d'annulation avant commit, le batch complet reste en mémoire et sera réémis
à l'identique avant toute lecture suivante. L'offset acquitté avance seulement
après succès ; normalisation et heure de lecture restent stables au réessai.

Tests : normalisation Postfix réelle, CRLF, source configurée, reprise à offset
non nul, fenêtres courtes/longues, EOF répété puis append, ligne trop longue,
réessai après erreur/annulation, fichier réécrit après lecture anticipée,
checkpoint initial invalide et commit SQLite réussi suivi d'un accusé perdu
(un seul record et événement après réessai).

Limites : un seul appelant et ownership exclusif des lectures/seeks du
descripteur ; sérialisation des écritures de source à assurer par l'appelant.
Le Sink ne doit pas modifier le batch ; le normaliseur cède son résultat à
l'ingestion. Aucun polling, suivi de chemin ou diagnostic de rotation/troncature
pendant lecture dans ce lot. Le constructeur n'arbitre pas une reprise jugée
insuffisante : l'état doit venir de la décision de génération.

## Attendre les ajouts à EOF

`Ingestor.Follow(ctx, Sink, intervalle)` poursuit les commits sur la génération
déjà ouverte. Un EOF du lecteur provoque une attente avec timer annulable, puis
une nouvelle lecture conservant la ligne partielle. L'intervalle zéro choisit
une seconde ; une valeur explicite doit être entre 10 ms et une minute.

Les autres erreurs sont retournées immédiatement. Un EOF du Sink, même enveloppé,
reste une erreur de commit et ne déclenche aucune attente ou tentative automatique.
Le batch reste disponible dans l'ingesteur pour un réessai explicite. Aucun timer
n'est actif entre les lignes déjà disponibles ; un seul timer est utilisé par
attente, arrêté au retour. L'annulation à EOF ne dépend pas de l'intervalle choisi.

Tests : append après EOF partiel, parcours de plusieurs lignes, attente réelle
avec timer puis append, annulation pendant une attente longue, erreurs du Sink
(dont EOF direct/enveloppé) sans boucle de réessais, erreur de lecture et intervalles
invalides sans progression. Les tests couvrent aussi le batch réessayable après arrêt.

Limites : suivi du seul descripteur, pas du chemin ; aucun choix de génération,
fermeture de fichier ou diagnostic de rotation/troncature. Un seul appelant ;
l'annulation pendant une lecture bloquante ou un commit reste conditionnée aux
contrats du lecteur et du Sink. FileSource n'est pas encore complet.

## Différer une génération initialement vide

`GenerationStart.WaitingForContent` est vrai uniquement lorsque la sélection
autorisait une nouvelle origine (`absent` ou `different`) mais que le préfixe
capturé contient zéro octet. Aucun ID, origine ou checkpoint n'est enregistré ;
`State` reste absent et `Created` faux. L'appel ne déplace pas la lecture.

L'appelant peut relancer `EnsureGeneration` après ajout de contenu. La décision
est alors refaite, et un préfixe non vide permet l'enregistrement initial normal.
Il n'y a pas de boucle d'attente ajoutée à ce helper. Les origines persistées
avec empreinte vide restent diagnostiquées `insufficient`, même après append,
sans réécriture de leur identité ou remplacement implicite.

Tests Linux : décisions répétées sur fichier vide sans aucun commit, état SQLite
inchangé pour source sans origine ou après troncature d'une génération connue,
position inchangée, enregistrement après append avec préfixe non vide, et
conservation d'une ancienne origine vide avant/après append. Les tests de
registration existants couvrent maintenant aussi un fichier d'un seul octet.

Limites : une ancienne origine vide exige encore une décision explicite ;
checkpoint zéro et course de réécriture du fichier restent des comportements
distincts. Le scheduler de chemin devra gérer `WaitingForContent` avant de créer
un ingesteur. Aucune migration ou dépendance ajoutée.

## Politique explicite de reprise à zéro

Les entrées `VerifyCandidateWithPolicy`, `SelectResumeWithPolicy` et
`EnsureGenerationWithPolicy` acceptent `ResumePolicy{AllowZeroCheckpoint: true}`.
Les entrées existantes et la valeur zéro de la politique restent strictes.

La relecture exige un checkpoint présent à zéro avec OriginID cohérent, une
ancre zéro canonique, une identité physique concordante et un préfixe non vide
concordant. Le résultat est `restart_zero`, distinct de `match`/`unique` fondés
sur une ancre positive. L'ancre vide ne devient pas une preuve de continuité.

Le sélecteur applique les mêmes limites, pagination et règle de choix unique.
Deux candidats à zéro, ou un candidat à zéro et un positif, sont ambigus.
Un candidat incomplet ou à empreinte vide bloque toujours la décision. Aucune
priorité par offset, date ou ID ; aucune nouvelle origine pour une relecture.

La décision de génération renvoie l'origine et son checkpoint zéro sans écrire
au Sink, sans marquer `Created` ou `WaitingForContent`. L'ingesteur peut alors
relire depuis zéro ; son premier commit avancera le checkpoint normalement.
Un préfixe changé, avec politique activée, est une différence et peut conduire
à une nouvelle génération suivant les règles existantes.

Tests : choix paginé à zéro avant/après des différences, ambiguïté zéro/zéro
et zéro/positif dans les deux ordres, preuves insuffisantes. Tests Linux : append,
préfixe modifié/tronqué, ancre incohérente/non canonique, identité/OriginID,
checkpoint absent/négatif, préfixe vide ; réouverture SQLite, mode strict par
défaut, décision sans écriture puis relecture réelle de la première ligne et
retour à une reprise positive.

Limites : l'activation autorise une relecture ; elle ne prouve pas la génération
avec une ancre positive. Les garanties bornées/non atomiques du préfixe et la
sérialisation de la source restent requises. Les anciennes empreintes vides
restent insuffisantes. La politique est acceptée par `file.Config` depuis le lot
démarrage ; elle n'est pas encore exposée par une CLI ou un fichier de configuration.

## Ouverture vérifiée du chemin de journal

`OpenLog(ctx, chemin)` retourne un descripteur en lecture seule, à l'offset zéro,
et son identité physique, après vérification d'un fichier régulier. L'identité
doit être identique avant l'ouverture, sur le descripteur et sur le chemin après
l'ouverture. Les liens symboliques vers des fichiers réguliers sont acceptés.

Un remplacement ou une disparition observée pendant cette séquence produit
`ErrPathChanged` ; la disparition conserve aussi `fs.ErrNotExist`. Tous les
échecs après ouverture ferment le descripteur et ne retournent ni fichier ni
identité utilisables. Après succès, la fermeture appartient à l'appelant.

Sur Linux, `O_NONBLOCK` empêche une course de remplacement par FIFO d'attendre
un écrivain ; le descripteur non régulier est ensuite refusé. Les autres
plateformes font le contrôle préalable et après ouverture mais ne garantissent
pas une ouverture non bloquante pendant une course de remplacement.

Tests : lecture dès zéro, écriture refusée/contenu inchangé, chemin vide/absent,
répertoire, annulation, identités remplacées et fermeture sur échec. Tests Linux :
lien symbolique régulier puis cible changée, rename/create avec ancien descripteur
ouvert, FIFO connu et remplacement par FIFO après un contrôle régulier.

Limites : les vérifications ne verrouillent pas le chemin et ne prouvent pas la
génération ; les changements ultérieurs restent à surveiller. L'annulation est
vérifiée entre appels système et n'interrompt pas un appel de filesystem bloqué.
Aucun Sink, suivi de rotation, migration ou dépendance ajouté.

## Démarrage FileSource

`file.New(Config, StateReader, Normalize)` valide l'identité de source, le chemin,
les dépendances et l'intervalle sans ouvrir ni écrire. La configuration est copiée
et le chemin relatif résolu une seule fois. `FileSource` implémente `source.Source`.

`Run` ouvre avec `OpenLog`, décide avec `EnsureGenerationWithPolicy` puis suit
l'état utilisable via l'ingesteur. Un fichier neuf vide est fermé avant une attente
annulable ; ouverture et décision sont refaites après chaque intervalle. Les
décisions insuffisantes, ambiguës ou limitées retournent `ResumeDecisionError`
avec leur statut fixe, sans commit. La politique explicite de relecture à zéro
est transmise depuis `Config` ; le défaut reste strict.

Tout descripteur ouvert est fermé à la sortie, y compris sur erreur/annulation.
`ErrSourceRunning` refuse deux exécutions simultanées du même objet ; l'appelant
doit toujours sérialiser les écritures d'un ID de source entre objets/processus.
Un appel ultérieur reprend l'état acquitté ; le batch en mémoire n'est pas conservé
entre appels à `Run`, la reprise durable et l'idempotence du Sink restent requises.

Tests : configuration et copie, chemin absent, annulation, fermeture. Tests Linux :
démarrage puis reprise SQLite sur le même objet, append après attentes sans
enregistrement vide, fermeture avant attente initiale et annulation, refus d'un
appel concurrent/libération du verrou, diagnostics sans commit, échec du Sink,
politique explicite zéro avec checkpoint avancé sur la même origine.

Les changements du chemin et la bascule sont décrits dans les lots suivants.
Les erreurs d'ouverture et du Sink arrêtent `Run` sans réessai automatique.
Pas de CLI, migration ou dépendance ajoutée.

## Observation du chemin pendant une rotation

`ObservePath(ctx, descripteur, chemin)` retourne `PathObservation` avec un statut
fixe `same`, `missing` ou `replaced` et les identités observées du descripteur et
du chemin. Pour `missing`, l'identité courante est vide ; ce statut couvre aussi
un lien symbolique dont la cible est absente. Les liens vers des fichiers réguliers
sont suivis. Un type non régulier produit `ErrPathNotRegular` ; les autres erreurs
de filesystem sont conservées. Tout échec retourne une observation vide.

Deux appels de métadonnées, sans ouverture, lecture, seek, fermeture, choix de
génération ou écriture d'état. Le descripteur reste à l'appelant sur tous les chemins.
La taille peut varier avec une identité `same` ; cette observation ne décide pas
d'une troncature et ne prouve pas la continuité d'une génération. Les observations
ne sont pas atomiques ; l'annulation est vérifiée entre appels système.

Tests locaux : identité stable après croissance/troncature, absence, remplacement
avec contenu identique, annulation/entrées invalides, refus d'un répertoire,
position inchangée et descripteur encore lisible après succès ou erreur. Tests Linux :
rename/create, disparition puis retour de la même identité, tailles distinctes et
écriture tardive dans l'ancien fichier, liens stables/retargetés/pendants, boucle de
lien signalée comme erreur, FIFO refusée sans attendre un écrivain.

L'intégration de cette observation dans `Run` est décrite au lot suivant.
Aucune migration ou dépendance ajoutée.

## Polling du chemin et disparition temporaire

Après décision de génération, `FileSource` observe le chemin avant la première
consommation, puis après chaque attente lorsque aucun lecteur ne progresse.
`missing` conserve le
descripteur et l'ingesteur, y compris une ligne partielle. Le statut `replaced`
déclenche désormais le lot de bascule ci-dessous. Une erreur de contrôle du chemin
arrête `Run` et sa fermeture habituelle des descripteurs.
Les erreurs du Sink, y compris EOF, restent retournées sans polling ni réessai.

`LastPathStatus()` est consultable pendant `Run`, sous verrou bref, et expose
uniquement le dernier statut observé avec succès. Valeur vide avant observation,
réinitialisée au début d'un nouvel appel accepté ; un appel concurrent refusé ne
réinitialise rien. Le dernier statut est conservé après arrêt/erreur : il peut être
périmé et ne constitue pas un état de fonctionnement ou une preuve de continuité.
Aucun chemin, contenu de journal ou identité physique n'est exposé par cette méthode.

Tests locaux : observation avant consommation, erreur initiale, erreur Sink/EOF,
annulation sans acquitter une ligne partielle, conservation puis réinitialisation
du statut. Tests Linux : disparition après première ligne, append terminant une
ligne partielle sur l'ancien fichier, retour de son chemin puis nouvelle ligne,
une seule origine SQLite et checkpoint 24 ; erreurs FIFO/boucle de lien au polling
et fermeture. Le test initial sans bascule a été remplacé par les tests de rotation.
La CI Go 1.26 ajoute le détecteur de courses sur le paquet FileSource pour vérifier
la consultation concurrente du statut avec les tests de suivi.

Le polling périodique entre lectures est décrit dans le lot flux continu.
L'absence lors de l'ouverture initiale reste une erreur immédiate.
Pas de migration ou dépendance ajoutée.

## Bascule après rename/create

Sur `replaced`, `Run` ouvre le nouveau chemin avec `OpenLog` et exige que l'identité
ouverte corresponde à celle observée. Une nouvelle course observée arrête le suivi
avec `ErrPathChanged`. Décision de génération et construction de l'ingesteur sont
réutilisées : checkpoints séparés, reprise existante vérifiée, sinon origine neuve
avec lecture depuis zéro. Les décisions inutilisables restent des erreurs typées.

Le descripteur et l'ingesteur précédents sont conservés, y compris les octets
partiels ; le suivi conjoint est décrit ci-dessous. Si leur identité redevient
courante, cet ingesteur est réutilisé sans nouvelle décision
ou registration. La capacité `MaxOpenGenerations = 2` compte les identités ouvertes,
y compris un successeur vide. Une troisième identité simultanée produit `ErrRotationCapacity`
avant toute ouverture ou écriture pour ce troisième fichier.

Un successeur vide reste ouvert et la décision d'enregistrement est différée
jusqu'à contenu non vide. `missing` conserve le fichier actif. Le scheduler possède
désormais tous les descripteurs après initialisation et les ferme à toute sortie
ou expiration. Les erreurs d'ouverture, de décision, d'ingesteur ou du Sink restent
sans réessai.

Tests Linux : origines et checkpoints SQLite distincts après rename/create,
ancien descripteur encore ouvert pendant ingestion du nouveau puis fermeture de
tous à l'annulation ; attente d'un successeur vide sans état persisté puis append ;
troisième identité refusée sans ouverture/registration ; échecs de registration,
de ligne et annulation après registration ; décision de reprise insuffisante ;
retour d'une identité conservée reprenant sa ligne partielle et son checkpoint.
La propriété des descripteurs est vérifiée via `/proc/self/fd` sur les seuls fichiers
synthétiques du test, sans compter les fichiers SQLite/runtime.

Limites : pas de diagnostic de lacune, recherche des rotations après redémarrage
ou détection de troncature. La rotation complète du MVP reste inachevée.
Pas de migration ou dépendance ajoutée.

## Suivi conjoint de l'ancien et du nouveau

Chaque passage tente au plus une ligne complète par génération ouverte, dans
l'ordre d'ouverture. L'ancien ingesteur conserve sa ligne partielle et reprend les
ajouts après bascule, même si le chemin courant reste sur le nouveau fichier. Un
successeur vide attend sans bloquer les ajouts de l'ancien et sans registration
avant contenu non vide. Les deux descripteurs restent bornés par la capacité 2.

Les commits sont appelés successivement sur le même goroutine de `Run` ; pas de
transaction globale entre générations. Chaque ligne et son checkpoint conservent
leur transaction indépendante. Une erreur de lecture ou du Sink sur l'un arrête
immédiatement le scheduler avant les lectures suivantes. EOF du Sink ne déclenche
ni attente ni réessai ; son batch reste pending dans l'ingesteur concerné et son
checkpoint n'avance pas. La fermeture et la reprise durable entre `Run` restent
identiques aux lots précédents.

Une attente intervient seulement si aucun ingesteur n'a acquitté de ligne pendant
le passage et si la prochaine échéance de polling n'est pas encore atteinte.
Le contrôle périodique applique la décision de bascule/capacité habituelle.
La consultation du statut reste synchronisée.

Tests Linux : nouvelle génération avec trois lignes prêtes, append tardif terminant
une ligne partielle de l'ancien puis une autre ligne ; intercalage sans famine,
identités distinctes, ordre par génération et checkpoints SQLite 24/18. Vérification
de l'absence de commits concurrents et d'attente pendant la progression. Successeur
vide avec append sur l'ancien sans registration vide. Erreur du Sink, EOF du Sink
et erreur de lecture sur l'ancien : arrêt avant la ligne suivante du nouveau,
checkpoints inchangés à 6/6, batch pending conservé et acquittable manuellement,
successeur fermé. Les tests de bascule/capacité/retour d'identité restent applicables.

Limites : l'équité est par ligne, pas par durée ; une ligne physique très longue ou
un Sink lent peut retarder l'autre génération et le prochain contrôle. Pas de troncature,
diagnostic de lacune ou recherche d'archives après redémarrage. Une ligne restant
incomplète n'est jamais acquittée. Pas de migration ou dépendance ajoutée.

## Période de grâce après EOF stable

`Config.RotationGrace` fixe la grâce des fichiers qui ne sont plus courants : zéro
sélectionne 30 s ; valeurs explicites entre 10 ms et 24 h. Validation et copie ont
lieu dans `New`, avant ouverture ou écriture. La durée ne s'accumule qu'après un
EOF observé sur un fichier retiré du chemin courant, sans ligne partielle ni batch
pending. Chaque ligne acquittée remet le compteur à zéro ; une fin partielle le
désactive jusqu'à complétion. Une génération neuve vide peut également expirer
si elle n'est plus courante, sans écrire d'origine vide.

Au polling, identifier d'abord le fichier courant, y compris le retour d'une
identité conservée ; le courant est protégé, même si son chemin a disparu. Puis
fermer les autres générations ayant atteint la grâce et libérer leur capacité
avant d'ouvrir un éventuel troisième fichier. La fermeture recontrôle l'absence
de batch/partiel et la taille du descripteur par rapport à l'EOF observé. Une
croissance ou diminution constatée renouvelle l'observation au prochain passage.

La propriété des descripteurs est centralisée : `runOpened` ferme un échec ou un
fichier initial vide ; après transfert, le scheduler possède le premier et ses
successeurs. Une fermeture d'expiration retire le fichier de sa liste avant
propagation d'erreur pour éviter toute double fermeture. Les checkpoints et
origines SQLite des fichiers fermés restent conservés.

Tests locaux : bornes/default de configuration, batch non acquitté protégé puis
retrait après acquittement/EOF, annulation du retrait, fichier courant conservé.
Tests Linux à horloge contrôlée : aucun retrait avant le délai, retrait à la
limite exacte et rotation suivante avec trois origines/checkpoints distincts ;
append pendant l'attente à la limite détecté avant fermeture et grâce renouvelée ;
lignes partielles normales/surdimensionnées bloquant un troisième descripteur ;
chemin courant absent protégé bien au-delà du délai, nettoyage sans double fermeture.
Les scénarios précédents et le détecteur de courses restent dans la CI.

Limites : retrait lors du polling entre passages de lecture. Stat/fermeture ne
sont pas atomiques ; un ajout après la dernière
observation ou après la grâce peut être manqué, et une réécriture de taille identique
n'est pas détectée ici. Pas de détection de troncature, diagnostic de lacune ou
recherche d'archives après redémarrage. Une ligne partielle protégée peut maintenir
la capacité occupée et provoquer `ErrRotationCapacity`. Pas de migration ou dépendance.

## Polling pendant un flux continu

Après le contrôle initial, une échéance est fixée à `PollInterval`. Entre les
passages équitables de lecture, si elle est atteinte, contrôler le chemin et
expirer les générations éligibles, même si des lignes viennent d'être acquittées.
L'échéance suivante est calculée après le contrôle ; pas de rafale de rattrapage
pour des intervalles manqués. Le temps de production repose sur `time.Now` et
ses mesures monotones ; l'horloge contrôlée reste un détail privé des tests.

Sans progression, attendre uniquement la durée restante avant ce contrôle ; si
l'échéance est déjà atteinte, contrôler immédiatement sans attendre. En progression,
aucune attente. Le callback privé d'attente accepte désormais cette durée ; `Run`
utilise le timer annulable existant. Une erreur de lecture/Sink interrompt le passage
avant tout polling supplémentaire, y compris EOF du Sink arrivé à l'échéance.

Tests locaux : un commit consommant 75 ms d'un intervalle de 100 ms produit une
attente de 25 ms, annulable, sans nouvelle ligne acquittée. Tests Linux à horloge
contrôlée : rename/create observé avant consommation de toutes les lignes de
l'ancien, lectures intercalées sans attente et checkpoints distincts ; retrait
de l'ancien à la grâce pendant que le nouveau garde des lignes prêtes ; EOF du
Sink à l'échéance conservant pending/checkpoint avant un contrôle de chemin qui
aurait échoué. Les tests précédents ont seulement adapté la signature d'attente.

Limites : le contrôle attend la fin d'un passage ; une lecture de ligne physique
très longue, un normaliseur ou un Sink lent peut encore le retarder. Les appels
filesystem restent non atomiques avec les écritures. Pas de détection de troncature,
diagnostic de lacune ou recherche d'archives après redémarrage. Pas de migration
ou dépendance ajoutée.

## Diagnostic de troncature observée

Avant l'observation du chemin, l'expiration ou la bascule, chaque polling contrôle
les tailles des deux descripteurs au maximum. Une taille inférieure à l'offset
réellement consommé par `LineReader` arrête le suivi avec `ErrFileTruncated`, dont
le texte fixe ne contient ni chemin ni contenu de journal. L'offset inclut les
octets d'une ligne partielle, même surdimensionnée ; il exclut les octets en avance
dans le tampon et n'est pas limité au checkpoint acquitté.

Le contrôle ne déplace pas la lecture et ne modifie aucune origine ni checkpoint.
Le scheduler ferme les descripteurs à la sortie. Les lignes déjà acquittées restent
persistées ; aucun reset à zéro, nouvelle origine ou réessai automatique. Les
erreurs de stat et l'annulation gardent leur cause.

Tests locaux : troncature du courant après ligne complète ou partiel (taille encore
supérieure au checkpoint), partiel surdimensionné, checkpoint acquitté inchangé et
fermeture ; offset consommé distinct de la lecture anticipée sans seek/close ;
annulation et erreur de stat ; append complétant normalement un partiel. Test Linux
avec SQLite et horloge contrôlée : troncature du fichier conservé à l'échéance de
grâce et troisième rotation simultanée, arrêt avant expiration/enregistrement du
troisième, checkpoints distincts 6/7 inchangés et tous les descripteurs fermés.

Limites : contrôle seulement au polling entre passages ; une lecture, normalisation
ou transaction lente peut le retarder. Stat et écritures restent non atomiques.
Une troncature suivie d'une croissance suffisante avant le contrôle, une réécriture
de taille identique ou les changements d'octets encore non consommés échappent à ce
diagnostic. La récupération copytruncate et le diagnostic de lacunes restent à
développer. Pas de migration ou dépendance ajoutée.

## Contrôle de l'ancre acquittée pendant le suivi

Chaque polling vérifie d'abord les tailles, puis l'ancre du dernier checkpoint
positif acquitté de chaque descripteur, avant observation du chemin, expiration ou
bascule. Le contrôle relit par `ReadAt` au maximum 4096 octets par fichier, jusqu'au
checkpoint, sans déplacer la lecture. Il utilise `Ingestor.Position()`, pas le
buffer de queue enrichi par les octets partiels ou un batch non acquitté.

Une non-correspondance observée arrête avec `ErrCheckpointChanged`, diagnostic fixe
sans chemin ou contenu de journal. Le scheduler ferme ses descripteurs ; les
checkpoints et origines restent conservés, sans reset ou nouvelle origine implicite.
Un checkpoint à zéro ou une génération non enregistrée n'a pas de fenêtre à
contrôler et est ignoré, sans prétendre vérifier son contenu. Une ancre malformée
ou incohérente produit une erreur fixe distincte ; annulation et erreurs filesystem
gardent leur cause. Une diminution observée au contrôle de taille reste prioritaire.

Tests locaux : réécriture du courant à taille identique ou troncature puis
recroissance avant polling, avec partiel au-delà du checkpoint ; arrêt, checkpoint
inchangé et fermeture. Fenêtre bornée malgré tail partiel, modifications hors
fenêtre ignorées, position physique inchangée, checkpoint zéro/génération absente,
ancre malformée/incohérente, annulation et erreur de stat. Les tests existants
d'append et de troncature passent avec le nouveau contrôle. Test Linux avec SQLite
et horloge contrôlée : réécriture du fichier conservé à la grâce et troisième
rotation simultanée, arrêt avant expiration/enregistrement, checkpoints distincts
6/7 inchangés et fermeture de tous les descripteurs.

Limites : fenêtre bornée au dernier checkpoint acquitté ; une réécriture hors
fenêtre, des octets partiels/non acquittés ou un contenu rétabli avant vérification
peut échapper au contrôle. Il ne détermine pas la cause de la non-correspondance
et ne prouve pas l'intégrité du fichier entier. Les stat/lectures ne sont pas
atomiques avec les écritures. Une transaction/lecture lente peut retarder le
polling ; des lignes déjà acquittées ne sont pas annulées. La récupération
copytruncate et le diagnostic des lacunes restent à développer. Pas de migration
ou dépendance ajoutée.

## Sélection d'une rotation accessible pour un état fourni

`SelectRotation(ctx, directory, state, limit)` recherche un état fourni dans un
répertoire fourni, résolu en chemin absolu. Le budget explicite va de 1 à
`MaxRotationEntries` (1000) et compte toutes les entrées, y compris celles ignorées.
Les pages contiennent au maximum 32 entrées ; une entrée supplémentaire non
vérifiée permet de distinguer la fin exacte du budget d'un parcours incomplet.
Les sous-répertoires, liens symboliques observés, autres fichiers non réguliers,
noms `.gz` (casse ignorée) et signatures gzip sont écartés. Aucun parcours récursif
ou décodage d'archive. Le répertoire et au maximum un candidat sont ouverts ensemble.

Chaque candidat régulier passe par `OpenLog`, comparaison avec les métadonnées
observées de l'entrée, puis `VerifyCandidate` strict : identité physique, préfixe,
ancre et frontière LF. L'appelant fournit un état de sa source. Un checkpoint zéro
reste sans preuve suffisante. `RotationSelection` donne statut fixe et nombre
d'entrées examinées ; `Path` n'est renseigné que pour `unique`, après parcours
terminé avec exactement une correspondance et aucune preuve insuffisante. Deux
liens physiques concordants sont deux chemins ambigus. Les statuts `absent`,
`different`, `insufficient`, `ambiguous` et `limit_reached` ne donnent aucun chemin.

Erreurs et annulation suppriment tout résultat partiel. Tous les descripteurs
temporaires sont fermés avant retour, y compris sur échec. Origines/checkpoints
fournis restent inchangés ; ce composant n'appelle ni Sink ni SQLite et ne relance
pas l'ingestion. Le chemin sélectionné doit être rouvert et revérifié par son futur
utilisateur. Aucun état utilisable n'est conservé sous forme de descripteur ouvert.

Tests locaux : unicité après épuisement, budget exact et dépassement après match,
entrées ignorées comptées, pages bornées, absence/différence/insuffisance/ambiguïté,
erreur après correspondance, annulation, pages invalides et configuration/répertoire.
Tests Linux : renommage puis append tardif avec nouveau fichier au chemin original,
preuve positive sélectionnée, checkpoint/origine inchangés, hard links ambigus,
checkpoint zéro, préfixe modifié, exclusions gzip/lien/sous-répertoire, budget,
entrée disparue ou remplacée et fermeture des descripteurs de fichiers/répertoire.

Limites : pas de snapshot atomique des pages, métadonnées ou empreintes ; des
mutations concurrentes du répertoire peuvent cacher une entrée. L'identité
persistable reste disponible sur Linux ; les fenêtres ne prouvent pas l'ensemble
des octets. Les signatures gzip sont seulement exclues, les autres formats sont
traités comme octets bruts. Context est vérifié entre appels filesystem, sans
interrompre un syscall bloqué ; les limites d'ouverture non Linux restent celles
d'`OpenLog`. Pas de sélection d'états persistés au démarrage, raccordement à `Run`,
récupération copytruncate ou diagnostic de lacunes. Pas de migration/dépendance.

## Lecture des origines enregistrées pour un chemin

`source.PathStateReader.FileOriginsByPath(ctx, source.OriginPathQuery)` fournit les
origines d'une source dont le chemin stocké est exactement égal au chemin demandé,
avec checkpoint optionnel. Ce contrat distinct complète le lecteur par identité
physique, sans imposer cette capacité aux lecteurs existants. ID de source et
chemin doivent être non vides ; `Limit` vaut de 1 à `MaxOriginPageSize` (100).
`AfterID` est un curseur exclusif dans l'ordre lexicographique des IDs ; vide au
départ, il peut ensuite être un ID absent. `NextID` termine le parcours lorsqu'il
est vide. Les chemins ne sont ni normalisés, ni comparés sans casse, ni traités
comme motifs ; Source, Path et curseur sont des paramètres SQL liés.

SQLite joint origines et checkpoints dans une seule requête par page, avec une
ligne supplémentaire pour détecter la suite. Les deux lecteurs partagent le
décodage des pages et exploitent le schéma existant, notamment l'index source/chemin.
Les métadonnées sont rendues telles quelles, y compris une identité physique vide
ou des empreintes non validées. Un checkpoint absent reste nil ; zéro reste présent.
Aucune écriture d'origine/checkpoint, migration ou dépendance ajoutée.

Tests locaux : réouverture SQLite, pages de taille 1 sans perte, ordre par ID malgré
FirstSeen inversés, origines du même chemin avec identités physiques différentes
ou vides, checkpoint absent/zéro/positif, isolation des sources et chemins proches
(casse, normalisation ou caractères de motif), valeurs littérales SQL dans source,
chemin et curseur, curseur exclusif absent/après la fin, borne maximale, arguments
invalides et annulation sans résultat partiel. Les tests existants du lecteur par
identité physique passent après mutualisation du décodage.

Limites : une page est un snapshot de requête ; des pages successives ne constituent
pas un snapshot global si l'état change. L'ordre des IDs n'est pas une chronologie
et ne sélectionne aucune génération active. Le chemin représente la dernière
métadonnée enregistrée, pas une preuve d'emplacement actuel sur disque. Les résultats
ne valident pas identité/préfixe/ancre et ne déclenchent aucune recherche de rotation.
La reprise automatique, gzip et la récupération copytruncate restent à développer.

## Parcours borné et validé des états par chemin

`LoadPathOrigins(ctx, sourceID, path, reader, limit)` utilise `PathStateReader` avec
budget explicite de 1 à `MaxPathOrigins` (1000) états. Chaque demande est bornée à
100 états et au budget restant. Les pages sont contrôlées : taille, chemin exact,
IDs non vides strictement croissants au-delà du curseur précédent et continuation
égale au dernier ID d'une page non vide. Toute page sans progression est refusée
avec `ErrInvalidPathOriginPage`, diagnostic fixe sans contenu enregistré.

Les états et positions optionnelles sont copiés à réception de chaque page. Le
résultat `PathOrigins` ne les expose que sur `complete`, après épuisement du
parcours. Un parcours vide donne `absent`. Si le budget est atteint avec une
continuation, `limit_reached` garde le compte examiné et aucune liste exploitable ;
le budget exact sans continuation reste complet. Erreurs et annulation renvoient
un résultat vide, y compris après une première page valide. Les copies restent
indépendantes si le lecteur réutilise ses buffers ou si le consommateur les modifie.

Tests locaux : plusieurs pages, budgets exact/réduit sur 101/102 états, absence,
bornes des demandes et absence de liste partielle ; checkpoint nil/zéro/positif,
réutilisation de checkpoint entre deux appels et mutations dans les deux sens ;
pages trop grandes, chemin incorrect, ID vide/dupliqué/non trié/périmé, curseur
incohérent et page vide avec continuation ; arguments invalides, erreur et
annulation après une page, annulation avant tout appel. Intégration SQLite locale :
101 états lus avec dernière page réduite à 1, puis budget 100 sans résultat partiel.

Limites : le lecteur garantit le filtrage source (les états n'embarquent pas leur
source ID). Les pages ne sont pas un snapshot global ; l'appelant doit sérialiser
les écritures d'état de sa source pendant parcours/application. Les métadonnées et
ancres restent brutes ; aucune preuve d'existence sur disque ni choix de génération
active. ID/date ne servent pas à déduire une chronologie. Ce composant n'ouvre aucun
journal et n'écrit aucun état. Pas de migration ou dépendance ajoutée. Le
raccordement à la recherche/reprise, gzip et copytruncate restent à développer.

## État durable du suivi des générations (stockage)

ADR-009 consigne la distinction durable entre inconnu (0), en suivi (1) et retiré
(2), exposée par `OriginState.FollowState`. La migration v2 ajoute une colonne
SQLite contrainte à ces trois valeurs, avec 0 par défaut. Schéma v1 conservé ;
ses générations, observations et checkpoints restent inchangés, sans suivi/retrait
inventé. Une nouvelle origine sans déclaration reste également inconnue. Les deux
lecteurs d'origines exposent cet état dans la même page que le checkpoint.

`Batch.FollowTransitions` déclare une origine, l'état attendu et la cible. Paires
autorisées : inconnu → en suivi, en suivi → retiré, retiré → en suivi pour une
réacquisition explicite. Un retour à inconnu, un retrait direct d'inconnu, une
transition sans changement ou deux transitions pour la même origine sont refusés.
La source doit être de type file. SQLite exige la source/origine exacte et un état
stocké égal à l'état attendu ou déjà à la cible (réessai idempotent). Origine absente,
étrangère ou état incompatible : `ErrFollowStateConflict`, texte fixe.

Transitions, origines, observations et checkpoints sont acquittés par une seule
transaction. Échec/annulation : rollback complet. Un upsert d'origine ou un batch
sans transition conserve l'état. Retrait/réacquisition ne suppriment ni provenance
ni checkpoint. La migration v2, son historique et user_version sont atomiques ;
l'ouverture vérifie l'historique de chaque version présente. Version future (>2),
version négative et base étrangère non versionnée restent refusées avant changement
du journal. Aucune dépendance ajoutée.

Tests locaux : acquisition avec origine/observation/checkpoint, réessais sans
doublon, upsert conservant l'état, retrait, acquisition obsolète refusée et
réacquisition explicite persistée après réouverture ; lectures par identité et
chemin ; isolation source/origine, conflit annulant une première transition,
échec du second checkpoint annulant état/observation/premier checkpoint puis batch
corrigé acquitté ; transitions invalides/dupliquées/non file, contrainte SQL et
annulation. Migration depuis v1 avec données réelles synthétiques conservées comme
inconnues ; échec injecté après ALTER annulant colonne/historique/version ; histoire
v2 manquante refusée. Les tests des anciennes migrations ont adapté version/historique
attendus et le cas version future, avec un cas négatif supplémentaire.

Limites : le Sink ne vérifie ni fichier, empreinte, EOF ni grâce. La source doit
justifier acquisition/retrait et sérialiser les transactions/réessais ; l'état
attendu n'est pas une époque de propriétaire et n'empêche pas un ancien réessai
après un cycle complet de réacquisition. Retiré n'est pas une promesse d'absence
d'écritures futures. À la fin de ce lot de stockage, le scheduler ne publiait
aucune transition. La reprise automatique n'exploite pas encore ce champ. Gzip,
copytruncate et diagnostic de lacunes restent à développer.

## Acquisition acquittée par FileSource

`prepareGeneration` attend désormais le Sink après décision et vérification par
`NewIngestor`, avant toute consommation de ligne. Origines inconnues/neuves et
retirées : batch de transition uniquement vers en suivi, sans record, origine ni
checkpoint. L'état déjà en suivi réutilise son acquittement durable, sans commit
superflu. État invalide : `ErrInvalidFollowState`, diagnostic fixe. Le même chemin
est utilisé au démarrage et pour les successeurs de rotation. Fichier vide non
enregistré : attente sans acquisition. Le composant `EnsureGeneration` seul garde
son contrat d'enregistrement/reprise sans transition.

Erreur du Sink, y compris EOF, conflit ou annulation : arrêt sans première ligne
ni réessai, fermeture de tous les descripteurs détenus. Annulation après le commit
d'acquisition : état en suivi conservé, sans consommation. Aucun retrait n'est
inventé à la sortie. Provenance et checkpoint sont conservés.

Tests : nouvel enregistrement, reprise positive, état déjà en suivi et réacquisition
retirée, relecture zéro explicite ; position physique au checkpoint et normalisation
encore absente au commit d'acquisition, état durable vérifié avant le premier record,
provenance/checkpoint identiques de part et d'autre de la transition. Erreur, EOF,
conflit injecté et annulation avant/après acquittement sans lecture ni déplacement
du checkpoint, fermeture du descripteur ; préfixe modifié après enregistrement
refusé avant acquisition ; échec d'acquisition du successeur ferme les descripteurs
sans réessai. Tests existants fichier vide conservés ; callbacks de relecture zéro
et erreur au polling adaptés pour distinguer transition et record. Tests portables
de l'état invalide, de l'acquittement déjà présent et de l'annulation préalable.

Vérifications locales réussies : `go test ./...`, `go vet ./...` et compilation
des tests FileSource Linux amd64 sans CGO. Exécution des nouveaux cas Linux,
détecteur de courses et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI](https://github.com/Coubiac/mailtrace/actions/runs/37173188977) Go 1.26.x/stable
du commit `f10ea5e0b16d6b9d5a65ee16b35cd3795e1fb2db`.
Premier passage CI `37173131616` : échec de l'assertion de provenance des nouveaux
tests d'erreur, dû à une date zéro du helper hors plage UnixNano. Fixtures du lot
corrigées avec une date synthétique représentable ; nouvelle CI réussie ci-dessus.

Limites : enregistrement initial et acquisition sont deux transactions. Un échec
après enregistrement ou un arrêt avant le premier record conserve le checkpoint
zéro ; sa reprise reste soumise à la politique explicite `AllowZeroCheckpoint`.
Vérification bornée avec lectures de métadonnées avant acquisition, sans preuve
atomique face aux écritures concurrentes. Écritures d'état à sérialiser par source ;
pas d'époque de propriétaire. À la fin de ce lot, le retrait durable restait à
développer ; l'exploitation des états à la reprise, gzip/copytruncate/lacunes aussi.

## Retrait acquitté à EOF stable/grâce

`retireExpired` reçoit le Sink et, pour une génération enregistrée admissible,
acquitte en suivi → retiré après les contrôles de taille/ancre du polling et la
revérification finale de taille, avant fermeture/libération de capacité. Batch de
transition seul, avec source et origine de l'ingesteur ; aucune modification de
provenance/checkpoint. Un ancien fichier vide encore non enregistré est fermé
sans transition. Génération courante (même chemin absent), partiels ordinaires ou
surdimensionnés et batch pending restent protégés ; ajout observé renouvelle la
grâce. Le suivi continu réutilise la même expiration.

Erreur du Sink, dont EOF/conflit, ou annulation avant acquittement : arrêt sans
réessai, lecture ou ouverture supplémentaire ; état en suivi conservé et fermeture
des descripteurs restants par le nettoyage du scheduler. Annulation après commit :
retrait durable conservé, sortie et fermeture par nettoyage. Échec de fermeture
après commit : retrait conservé, descripteur retiré de la collection avant retour
d'erreur, donc pas de seconde fermeture. Aucun retrait inventé sur arrêt hors
expiration.

Tests : retraite durable avant réutilisation de capacité pour une troisième
génération, descripteur encore ouvert au Sink et checkpoint conservé ; seules
les générations expirées sont retirées, les autres restent en suivi après
annulation. Ajout tardif/grâce renouvelée, partiels et courant disparu conservent
leur état attendu. Erreur, EOF, conflit injecté, annulation avant/après commit et
fermeture en erreur : un seul essai, deux records/générations, pas d'enregistrement
ou descripteur de troisième génération, provenance/checkpoint identiques et aucun
retrait du successeur. Tests portables : pending/annulation préalable sans transition,
acquittement avant Close, fichier vide sans transition, échec de Close retirant
le descripteur de la collection. Données synthétiques, aucune migration/dépendance.

Vérifications locales réussies : `go test ./...`, `go vet ./...` et compilation des
tests FileSource Linux amd64 sans CGO. Exécution Linux, détecteur de courses et
builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI retrait](https://github.com/Coubiac/mailtrace/actions/runs/37173662429)
Go 1.26.x/stable sur `baf30a13353d9a16fa6216670529a6789132cdd2`.

Limites : taille, transaction du Sink et fermeture ne sont pas atomiques avec les
écritures. Un ajout après le dernier contrôle, y compris pendant l'acquittement,
peut être manqué ; retiré ne signifie pas absence d'écritures ultérieures. Un seul
écrivain sérialise les transactions de la source ; pas d'époque de propriétaire.
À la fin de ce lot, la reprise n'exploitait pas encore les marqueurs. Gzip/copytruncate/lacunes restent
à développer, ainsi que la revue de sécurité indépendante déjà prévue.

## Candidats en suivi pour la reprise

`LoadFollowOrigins(ctx, sourceID, path, reader, limit)` réutilise le parcours
complet et borné de `LoadPathOrigins` (budget de 1 à 1000 états). Après épuisement
du parcours, les états retirés sont écartés ; l'ensemble en suivi n'est exposé
que s'il ne reste aucun état inconnu/invalide et au plus `MaxOpenGenerations` (2)
candidats. Aucun choix selon ID, date ou offset. Les résultats fixes sont :

| Statut | Résultat |
| --- | --- |
| `complete` | 1 ou 2 candidats en suivi, copies détenues par l'appelant |
| `absent` | Parcours vide ou historique entièrement retiré, aucun candidat |
| `limit_reached` | Parcours inachevé, aucun résultat partiel |
| `invalid_state` | Valeur de FollowState hors contrat, aucun candidat |
| `unknown_state` | Au moins un état inconnu, aucun candidat automatique |
| `capacity_exceeded` | Plus de deux générations en suivi, aucun sous-ensemble choisi |

Après parcours complet, priorité : invalide → inconnu → capacité → absence →
complet. La limite du parcours précède la classification. `Examined` compte tous
les états parcourus, y compris retirés/inconnus/invalides. Erreur du lecteur, page
incohérente ou annulation : résultat entièrement vide avec cause conservée.
Copies des checkpoints héritées du parcours ; nil et zéro restent distincts.

Tests : vide/retrait seul, 1/2/3 générations en suivi, inconnus hérités/mélangés,
valeurs négative/future et priorités quand plusieurs causes coexistent ; dates et
offsets opposés à l'ordre ID sans sélection chronologique. Nil/zéro, métadonnées
non vérifiées conservées et mutations sans alias dans les deux sens. Inconnu en
dernière page après 100 états : limite ou inconnu sans candidats ; erreur/page
invalide/annulation après première page et annulation/argument invalide avant
lecture. Intégration SQLite : 101 origines, 99 retirées et deux en suivi en dernière
page, budget réduit refusé, isolation de source, checkpoint absent/zéro et lectures
répétées sans changement d'état. Chemin de journal inexistant, aucune ouverture
ni écriture de suivi pendant la préparation. Données synthétiques, aucune dépendance
ou migration ajoutée.

Vérifications locales réussies : `go test ./...`, `go vet ./...` et compilation
des tests FileSource Linux amd64 sans CGO. Exécution Linux, détecteur de courses
et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI candidats en suivi](https://github.com/Coubiac/mailtrace/actions/runs/37174070307)
Go 1.26.x/stable sur `addd572898b0095c71952b232d1f5d26e066b710`.

Limites : seuls les états de suivi sont classés. Un ensemble complet ne prouve
ni présence sur disque, ni identité/empreinte/ancre/frontière LF, ni possibilité
de reprise ; les checkpoints nil/zéro et empreintes restent bruts. Historique
inconnu exige une décision explicite, sans inférence depuis les autres champs.
Les états retirés ne garantissent pas absence d'écritures futures. Filtrage source
garanti par le lecteur, pas de snapshot global ; sérialiser les écritures jusqu'à
application. Le budget compte l'historique entier, pas seulement les candidats.
La capacité finale devra aussi tenir compte d'un éventuel nouveau fichier courant.
Ce composant ne choisit pas le courant, n'ouvre aucun journal et n'écrit aucun état.
Localisation/revérification et raccordement à Run restent à développer, ainsi que
gzip/copytruncate/lacunes et la revue de sécurité indépendante prévue.

## Localisation bornée des candidats en suivi

`LocateFollowOrigins(ctx, configuredPath, origins, entryLimit)` exige un ensemble
`FollowOriginsComplete` de 1 ou 2 états en suivi, IDs non vides distincts et chemin
enregistré exactement égal au chemin configuré. Toute l'entrée est validée avant
accès disque ; `ErrInvalidFollowOrigins` est un diagnostic fixe. Les checkpoints
optionnels sont copiés avant les recherches, sans modifier la provenance.

`SelectRotation` est appelé séquentiellement dans le répertoire du chemin
configuré absolu, incluant le fichier courant. Budget partagé de 1 à 2000 entrées
examinées (`MaxFollowLocationEntries`), au plus 1000 par recherche et le reste du
budget ; les entrées réexaminées pour un autre candidat comptent à nouveau.
Budget épuisé avant le suivant : limite sans nouvelle recherche. L'entrée
supplémentaire de lecture pour établir fin/limite reste celle du chercheur existant.

`FollowLocations` expose `Locations` seulement si tous les candidats ont un
chemin unique distinct (`unique`). Chaque élément conserve état/checkpoint et
chemin localisé séparément. Deux origines sélectionnant le même chemin donnent
`ambiguous`, sans résultat partiel. La première décision non unique est retournée
dans l'ordre des candidats : `absent`, `different`, `insufficient`, `ambiguous`
ou `limit_reached`, avec somme `Examined` mais aucun chemin utilisable. Erreur et
annulation renvoient un résultat vide avec cause conservée. Aucun choix de
courant ni chronologie déduite des IDs/dates/offsets.

Tests portables : budget exact/épuisé avant second candidat/réduit pour le second,
plafond par recherche, chaque décision bloquante après un premier chemin valide,
entrée invalide/mélangée/dupliquée/trop grande refusée avant recherche ; erreur et
annulation avant/après première/seconde recherche sans chemins partiels. Copies
de métadonnées nil/zéro sans alias et ordre reçu conservé ; collision de chemin
refusée. Tests Linux : un ou deux candidats, fichier courant et rotation renommée,
états/checkpoints inchangés et aucun descripteur conservé ; absence, réécriture du
second, checkpoint nil/zéro strict, hard links ambigus, deux origines sur un seul
fichier et budget partagé dépassé, sans chemin partiel ni fuite de descripteurs.
Tous les fichiers sont synthétiques, aucune migration/dépendance ajoutée.

Vérifications locales réussies : `go test ./...`, `go vet ./...` et compilation
des tests FileSource Linux amd64 sans CGO. Exécution Linux, détecteur de courses
et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI localisation](https://github.com/Coubiac/mailtrace/actions/runs/37174564943)
Go 1.26.x/stable sur `10885fd205025ede06eb0be25a3cc3dd15279a0e`.

Limites : la vérification stricte et les exclusions du chercheur existant sont
conservées (pas de replay zéro implicite, liens observés/gzip/non réguliers écartés,
pas de récursion). Chaque recherche est une observation distincte et bornée,
pas un snapshot global ; changements hors fenêtres restent sans preuve. Le
répertoire courant du processus détermine une configuration relative à l'appel.
Le namespace source reste garanti par le résultat/lecteur fourni, les états
n'embarquant pas source ID. Sérialiser les écritures d'état jusqu'à application.
Tous les descripteurs temporaires sont fermés ; les chemins renvoyés exigent
réouverture et revérification. La capacité contrôlée porte seulement sur les
candidats persistés ; un nouveau fichier courant devra aussi compter dans les
deux descripteurs. Pas d'ingestion/transition ni raccordement à Run dans ce lot.
Gzip/copytruncate/lacunes et revue de sécurité indépendante restent à développer.

## Réouverture et revérification de l'ensemble localisé

`OpenFollowLocations(ctx, identity, locations, normalize)` exige un ensemble
entièrement `unique` de 1 ou 2 états en suivi. Toute l'entrée est validée avant
ouverture : identité file et normaliseur requis ; IDs non vides distincts, même
chemin d'origine non vide, chemins sélectionnés absolus et distincts après
nettoyage lexical. Diagnostic fixe `ErrInvalidFollowLocations`. Checkpoints
optionnels copiés pour tout l'ensemble avant la première ouverture.

`OpenLog` ouvre chaque fichier en lecture seule ; `VerifyCandidate` contrôle
strictement la preuve actuelle, puis `NewIngestor` revérifie et se place au
checkpoint positif acquitté. Absence/zéro/preuve insuffisante : décision
`insufficient`, jamais de replay implicite ; contenu/identité différents :
`different`. Collision physique avec une autre ouverture : `ambiguous`, même
pour deux chemins distincts. Aucune ligne consommée/normalisée, aucun commit.

Le succès rend un propriétaire opaque `OpenedFollowSet`, collection privée de
descripteurs/identités/ingesteurs, avec `Len` et `Close`. Close vide la collection
avant les fermetures, ferme tous les fichiers même si l'un échoue, conserve les
erreurs et reste idempotent. Valeur zéro/récepteur nil acceptés. Aucun état durable
ne change sur Close. Objet non copiable et usage sérialisé, sans méthodes de
transfert au scheduler dans ce lot.

Sur erreur/annulation, les ouvertures déjà acquises, dont celle du candidat en
échec, sont fermées ; résultat nil, cause et erreurs de nettoyage conservées.
Pas de réessai ni ouverture après annulation constatée. Le propriétaire n'est
rendu qu'après succès et contrôle d'annulation pour chaque candidat.

Tests portables : entrée entière invalide/mélangée/dupliquée/trop grande,
chemin relatif/alias lexical et configuration invalide refusés avant ouverture ;
annulation préalable ; Close normal/en erreur ferme tous les fichiers, aucune
seconde fermeture, propriétaire vide/nil. Tests Linux : un/deux fichiers courant
et renommé, ajout tardif conservant les preuves, offsets/positions/pending inchangés
avant consommation, zéro normalisation, lecture seule et un descripteur par fichier ;
mutation du checkpoint appelant sans alias, et copie du second checkpoint avant
première ouverture. Disparition/remplacement/réécriture/troncature du second,
checkpoint nil/zéro/ancre invalide, hard link créant collision, erreur du second,
annulation après première/avant seconde/après seconde ouverture et erreur de
nettoyage : aucun ensemble partiel, cause conservée, toutes les ouvertures fermées.
Données synthétiques, aucune migration/dépendance ajoutée.

Vérifications locales réussies : `go test ./...`, `go vet ./...` et compilation
des tests FileSource Linux amd64 sans CGO. Exécution Linux, détecteur de courses
et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI réouverture](https://github.com/Coubiac/mailtrace/actions/runs/37175078596)
Go 1.26.x/stable sur `35fc07be34b38672e6e9586783dc5490dd7719d2`.

Limites : fenêtres bornées et contrôles successifs, pas de verrou/snapshot atomique
sur les fichiers. La vérification stricte précède le constructeur, qui relit les
fenêtres avant seek ; pas de consommation de ligne dans ces lectures de preuve.
Les chemins absolus localisés évitent une redirection par changement de répertoire
courant. Les états n'embarquent pas de source ID ; l'appelant garantit le namespace
et sérialise les écritures d'état jusqu'à application. La capacité est celle des
candidats rouverts ; un nouveau fichier courant devra aussi compter. Propriété
et fermeture explicites, mais sélection du courant/transfert/scheduler/Run restent
à raccorder ; gzip/copytruncate/lacunes et revue de sécurité indépendante aussi.

## Dernier lot terminé : observer le courant parmi les fichiers rouverts

`OpenedFollowSet.ObserveCurrent(ctx, absolutePath)` exige un propriétaire valide
non vide de 1 ou 2 fichiers, IDs d'origine distincts. Il inspecte tous les fichiers
détenus (incluant le second), refuse une collision physique, puis réutilise
`ObservePath` pour le chemin configuré et `SameFile` pour identifier le courant.
Diagnostic fixe `ErrInvalidOpenedFollowSet` pour collection invalide/fermée.

`FollowCurrent` fournit quatre décisions :

| Statut | Données exploitables |
| --- | --- |
| `known` | OriginID correspondant et snapshot physique courant |
| `missing` | Aucun courant, ni identité ni ID inventé |
| `new_generation` | Snapshot courant distinct, seulement avec une place libre |
| `capacity_exceeded` | Aucune identité/ID utilisable, avant toute troisième ouverture |

Les fichiers sont choisis par identité physique, jamais par ordre/ID/date/offset.
Les liens réguliers sont suivis comme dans l'observation existante. Erreur de stat,
chemin non régulier ou annulation : résultat vide, cause conservée. Le propriétaire
conserve tous ses fichiers sur toute sortie ; aucun open/read/seek/close,
normalisation, changement de checkpoint/grâce, transition ou transfert.

Tests portables : deux fichiers de même contenu distingués physiquement, sélection
de chacun et ordre inversé sans choix arbitraire ; chemin absent sans courant ;
nouveau fichier à capacité 1/2, snapshots seulement quand exploitables. Erreurs de
chemin/observation et annulation avant/après observation sans résultat, descripteurs
et positions/ingesteurs/grâce conservés. Propriétaire nil/vide/invalide/trop grand,
second descripteur fermé et propriétaire après Close refusés sans fermeture du
premier. Tests Linux avec ensemble réellement localisé et rouvert : courant connu,
taille après ajout sans consommation, troisième fichier refusé sans ouverture,
disparition puis retour du fichier conservé identifié par origine ; descripteurs
conservés puis fermés par le propriétaire. Lien régulier vers une génération
conservée, boucle de lien et FIFO sans ouverture ni perte de propriété. Données
synthétiques, aucune migration/dépendance ajoutée.

Vérifications locales réussies : `go test ./...`, `go vet ./...` et compilation
des tests FileSource Linux amd64 sans CGO. Exécution Linux, détecteur de courses
et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI courant](https://github.com/Coubiac/mailtrace/actions/runs/37175634122)
Go 1.26.x/stable sur `3f3d72704c658e6fd03accd96883420a2be34f47`.

Limites : snapshots de métadonnées successifs, pas de verrou ni preuve de
checkpoint/intégrité. `known` signifie même fichier physique observé, pas mêmes
octets ; les contrôles de taille/ancre du suivi restent nécessaires. Recontrôler
le chemin lors de l'ouverture/adoption d'un nouveau courant. Méthodes du propriétaire
et écritures d'état de source à sérialiser jusqu'à application. La décision `missing`
exige une politique explicite au démarrage ; aucun courant retenu n'est inventé.
Capacité vérifiée avant toute ouverture, mais nouvelle génération/acquisition,
transfert au scheduler et raccordement à Run restent à développer, ainsi que
gzip/copytruncate/lacunes et revue de sécurité indépendante.

## Lot 32 : transfert au scheduler avec courant connu

`FileSource.FollowOpened` accepte un ensemble vérifié de 1–2 fichiers et une
décision `known`. Validation du contexte/Sink/garde partagée avec Run, nouvelle
observation du chemin et concordance OriginID/identité physique, identité complète
de source égale pour chaque ingesteur. Tout refus avant transfert conserve le
propriétaire et LastPathStatus ; décision obsolète : ErrPathChanged.

Collection vidée avant remise au scheduler commun : Close du propriétaire devient
inoffensif, fermeture désormais exclusive au scheduler au retrait ou à sa sortie.
Contrôles taille/ancre, suivi conjoint, polling/grâce/retrait durable réutilisés ;
aucun enregistrement/acquisition répété des ingesteurs déjà prêts. Courant choisi
par OriginID, quelle que soit sa position dans la collection. LastPathStatus remis
à zéro au transfert puis actualisé par le polling.

Tests portables : refus (contexte/Sink/exécution concurrente/décision/source/chemin/
descripteur fermé/ensemble vide) sans consommation ni perte de propriété/statut ;
erreur/EOF/annulation du Sink après transfert sans avancement du checkpoint ni
double fermeture ; propriétaire déjà vide lors du commit et Close inoffensif.
Troncature/réécriture à identité inchangée après décision connue : contrôle avant
consommation, arrêt et fermeture par le scheduler. Tests Linux avec vrais fichiers
localisés/rouverts et SQLite : 1–2 fichiers, courant premier ou second, ajouts
tardifs sans replay, checkpoints distincts à 9, retrait du seul ancien après grâce,
courant toujours en suivi, annulation et absence de fuite/double fermeture.

Vérifications locales réussies : `go test ./...`, `go vet ./...`, `git diff --check`
et compilation des tests FileSource Linux amd64 sans CGO. Exécution des tests Linux,
détecteur de courses et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI transfert](https://github.com/Coubiac/mailtrace/actions/runs/37176322112)
Go 1.26.x/stable sur `dbf85d443adf0194d4f40a73e5e4552b08450b5a`.

Limites : ce transfert exige un courant connu ; décisions missing/new/capacity
refusées avant transfert. Pas de verrou ni snapshot atomique : les contrôles de
suivi revalident les ancres après transfert. Une disparition ultérieure conserve
le courant déjà observé selon le scheduler existant. Run n'utilise pas encore le
pipeline de reprise d'ensembles persistés. Pas de migration/dépendance ajoutée ;
gzip/copytruncate/lacunes et revue de sécurité indépendante restent à développer.

## Lot 33 : nouveau courant avec une génération conservée

`FollowOpened` accepte désormais `new_generation`, sans OriginID et avec un seul
fichier conservé. Deux fichiers détenus : ErrRotationCapacity avant toute ouverture.
Nouvelle observation du chemin et concordance physique, identités de source et
taille/ancre de l'ancien vérifiées ; ouverture OpenLog puis concordance avec le
snapshot courant. Les refus avant transfert gardent l'ancien propriétaire/statut,
et ferment le nouveau descripteur temporaire s'il a été ouvert.

Après ouverture vérifiée : collection vidée, statut réinitialisé, préparation/
acquisition du nouveau avant toute consommation des deux fichiers. Erreur/EOF du
Sink/annulation ferme les deux, sans retrait inventé ni avancement du checkpoint
ancien. Puis scheduler commun, ancien ingesteur réutilisé sans réenregistrement.
Nouveau courant vide conservé sans origine/acquisition jusqu'au contenu, pendant
que le scheduler continue à lire l'ancien.

Tests portables : capacité avant open, décision périmée, source incohérente,
ancien réécrit/tronqué, erreur d'ouverture, identité remplacée entre observation
et ouverture, annulation après ouverture : propriétaire/positions/statut conservés,
aucune écriture et fermeture des seuls fichiers temporaires. Tests Linux avec
localisation/réouverture et SQLite : courant neuf ou vide puis append, ajout tardif
ancien sans replay, acquisition avant première ligne, checkpoints distincts ;
échec d'enregistrement/acquisition, EOF, annulation avant/après ack et checkpoint
zéro insuffisant, sans consommation/retrait ni fuite/double fermeture.

Vérifications locales réussies : `go test ./...`, `go vet ./...`, `git diff --check`
et compilation des tests FileSource Linux amd64 sans CGO. Exécution des tests Linux,
détecteur de courses et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI nouveau courant](https://github.com/Coubiac/mailtrace/actions/runs/37176769622)
Go 1.26.x/stable sur `c3791358b3c4d6b60c76d894109b1fe9ea9006e7`.

Limites : enregistrement/acquisition distincts ; un échec après enregistrement
laisse un checkpoint zéro et exige une décision de reprise explicite. Annulation
après acquisition conserve l'état durable en suivi. Vérifications non atomiques :
nouvelle rotation après transfert peut bloquer à capacité pleine. Missing au
démarrage reste refusé, sans courant inventé ; pipeline encore absent de Run.
Aucune migration/dépendance ajoutée. Gzip/copytruncate/lacunes et revue de sécurité
indépendante restent à développer.

## Lot 34 : diagnostic explicite du courant absent au démarrage

`FollowOpened` traite une décision missing canonique sans origine/snapshot :
recontrôle de l'ensemble, des descripteurs, du chemin toujours absent et de
l'identité complète de source. ErrCurrentMissing, diagnostic fixe exploitable
via errors.Is sans chemin/contenu, refuse le démarrage sans courant inventé.
Propriétaire, positions/ingesteurs/grâce, état durable et LastPathStatus conservés.
Aucun open/lecture de contenu/attente/écriture/transfert ; garde libérée au retour.

Chemin réapparu : décision missing obsolète refusée avec ErrPathChanged ; nouvelle
observation requise avant application de known/new ou traitement de capacity.
Erreur de stat/ensemble/source/contexte conserve sa cause, sans diagnostic missing
qui la masque. L'appelant peut réobserver ou fermer son propriétaire ; aucun
réessai automatique ni choix d'une génération retenue comme courant pendant l'absence.

Tests portables : ensembles de 1–2 fichiers, appels missing répétés sans open/wait/
commit, positions/propriété/statut conservés et garde libérée ; décision périmée
sur nouveau fichier (new ou capacity) ou courant connu, diagnostic stable. Décision
avec origine/snapshot, source incohérente, second fichier fermé, propriétaire nil,
annulation/exécution concurrente refusés sans perte de propriété. Linux avec vrais
fichiers localisés/rouverts et SQLite : disparition, ajouts tardifs pendant absence,
checkpoints/états inchangés après refus, retour de l'ancien par renommage et nouvelle
décision known ; reprise des seuls ajouts à offset 4, checkpoints à 9 distincts,
aucune acquisition/enregistrement répété ni fuite/double fermeture.

Vérifications locales réussies : `go test ./...`, `go vet ./...`, `git diff --check`
et compilation des tests FileSource Linux amd64 sans CGO. Exécution des tests Linux,
détecteur de courses et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI courant absent](https://github.com/Coubiac/mailtrace/actions/runs/37177140539)
Go 1.26.x/stable sur `14204b35911620242e73325b1f602ca3a34c15af`.

Limites : diagnostic d'une observation non atomique, pas preuve de stabilité ni
de continuité du contenu ; une nouvelle décision exigera toujours les contrôles
de suivi. Run n'utilise pas encore le pipeline de reprise des ensembles persistés.
Pas de migration/dépendance ajoutée. Gzip/copytruncate/lacunes et revue indépendante
restent à développer.

## Lot 35 : orchestrateur isolé de préparation de reprise

Périmètre réduit conformément à la consigne de petits lots : la préparation
orchestrée est terminée ; son intégration à Run reste explicitement incomplète.
Le StateReader de Run et PathStateReader sont deux contrats distincts, et les
budgets/compatibilité des lecteurs seront fixés lors du raccordement suivant.

`PrepareFollowResume` valide avant lecture identité file/nom/ID, chemin absolu,
PathStateReader/normaliseur et FollowResumeLimits explicites (Origins 1–1000,
Entries 1–2000 partagées). Compose les helpers existants : lecture des états en
suivi, localisation de tout l'ensemble, réouverture stricte puis observation.
`absent` seulement sur parcours complet vide/tout retiré, sans journal ouvert.
Unknown/invalide/capacité/limite bloque avec diagnostic fixe, jamais une absence
ni fallback. Sélection non unique transmet ResumeDecisionError et zéro résultat.

Après réouverture, absence du courant/capacité/erreur/annulation ferme l'ensemble
entier et renvoie zéro résultat, erreurs de nettoyage jointes à la cause. `ready`
seul remet le propriétaire complet à l'appelant avec courant known/new ; il doit
fermer ou appliquer via FollowOpened. Pas de ligne consommée/normalisée, Sink,
transition, nouveau courant ouvert ou transfert scheduler dans la préparation.

Tests portables : validation avant lecture/disque, budgets stricts, absent vide/
retiré sans FS, unknown/invalide/capacité/limite sans plan partiel, erreurs/annulation
du lecteur conservées, requête source/chemin/limite exacte. Linux SQLite : ensembles
1–2 connus, nouveau courant non vide/vide observé sans ouverture, ajouts aux fichiers
en suivi sans consommation, positions au checkpoint positif et état durable inchangé ;
propriétaire idempotent. Missing/capacité après réouverture, localisation bornée/
ambiguë/différente/absente et checkpoints nil/zéro insuffisants : diagnostic sans
ensemble partiel ni fuite. Erreur/annulation après réouverture et erreur de nettoyage
jointes, fermeture de tous les descripteurs et état conservé.

Vérifications locales réussies : `go test ./...`, `go vet ./...`, `git diff --check`
et compilation des tests FileSource Linux amd64 sans CGO. Exécution des tests Linux,
détecteur de courses et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI préparation](https://github.com/Coubiac/mailtrace/actions/runs/37177688035)
Go 1.26.x/stable sur `bbbdc66c33547a1703d7dc81f8d32af2704500c9`.

Limites : Run n'utilise pas encore cet orchestrateur. `absent` ne prouve pas la
présence/absence physique du courant et ne l'enregistre pas. Les fenêtres/pages
et observations restent non atomiques ; source state à sérialiser jusqu'à application,
courant à recontrôler au transfert. Pas de reprise zéro implicite ni migration/
dépendance ajoutée. Gzip/copytruncate/lacunes et revue indépendante restent à faire.

## Lot 36 : reprise d'ensemble au démarrage de Run

Run appelle désormais PrepareFollowResume puis le cœur d'application commun de
FollowOpened sous une seule garde, sans réacquisition. Reader doit implémenter
StateReader et PathStateReader ; ErrPathStateReaderRequired avant accès journal
si le second manque. New reste utilisable pour les helpers avec StateReader seul.
Config.ResumeLimits : zéro par champ → maximum borné (1000 états/2000 entrées),
copie/validation avant lecture et refus des valeurs négatives/excessives.

Préparation bloquée : arrêt sans fallback. Seul absent complet (aucun en suivi)
utilise le démarrage courant existant et l'attente annulable du neuf vide. Ready
applique known/new en réutilisant les ingesteurs et leurs checkpoints ; propriétaire
fermé si un refus survient avant transfert, vide après transfert et fermeture par
scheduler. Run accepté réinitialise LastPathStatus, même sur préparation bloquée.

Politique stricte : unknown de lifecycle bloque même avec AllowZeroCheckpoint ;
nil/zéro d'un ensemble en suivi demeure insuffisant. Replay zéro explicite reste
possible au démarrage courant sans ensemble en suivi, par exemple état retiré
explicitement. Aucune migration/reclassification ni fallback implicite ajouté.

Tests utiles : budgets constructeur/defaults, capacité de lecteur manquante,
garde concurrente et tests existants de démarrage/attente/cancellation adaptés au
contrat par chemin. Test replay zéro précisé avec état courant explicitement retiré.
Linux SQLite : Run reprenant 1–2 connus, ancien renommé + nouveau courant non vide/
vide, ajouts tardifs sans replay/acquisition répétés, acquisition du nouveau avant
ligne, checkpoints distincts et guard/FDs libérés. Unknown/zero même avec politique
explicite, missing/capacité/limite états ou entrées/réécriture sans fallback ni mutation.
Rejet ou annulation après préparation et avant transfert : fermeture complète,
cause conservée sans double fermeture.

Vérifications locales réussies : `go test ./...`, `go vet ./...`, `git diff --check`
et compilation des tests FileSource Linux amd64 sans CGO. Exécution des tests Linux,
détecteur de courses et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI démarrage Run](https://github.com/Coubiac/mailtrace/actions/runs/37178273467)
Go 1.26.x/stable sur `ddf00f313195df21c8100833842180efe9dbfb74`.

Limites : nil/zéro en suivi et lifecycle inconnu exigent encore une récupération
explicite distincte. Métadonnées/pages et observations non atomiques, writer de
source à sérialiser ; aucune reprise copytruncate automatique. Gzip/lacunes et
revue de sécurité indépendante restent à développer, aucune dépendance/migration
ajoutée. Ce lot raccorde le redémarrage, sans interface ou corrélation ajoutée.

## Lot 37 : diagnostic de lacune de reprise

PrepareFollowResume/Run associe ErrFollowResumeGap aux localisations terminées
absent/different d'une génération persistée en suivi. FollowResumeGapError a un
message fixe sans chemin/ID/offset/contenu et garde le ResumeDecisionError original
accessible par errors.As. La classification ne change pas SelectRotation ou
LocateFollowOrigins standalone ni les diagnostics unknown/capacité/missing.

Sans historique (ou tout retiré) : absence complète, démarrage courant autorisé.
Générations localisées mais courant absent : ErrCurrentMissing. Aucune génération
vérifiée localisable : lacune de reprise ; pas de lecture du nouveau courant ni
d'un sous-ensemble, reset ou retrait artificiel. Checkpoints/provenance/lifecycle
conservés pour revérification après restauration. Insufficient/ambiguous/limit et
erreurs filesystem ne sont pas convertis en lacune confirmée.

Tests portables : seuls absent/different classifiés, message stable, codes originaux
via errors.As, diagnostics insufficient/ambiguous/limit distincts et aucune confusion
avec missing. Linux SQLite : archive conservée seule absente, premier/second de
deux fichiers absent (y compris après première localisation réussie), courant
disponible jamais ingéré pendant refus ; état inchangé, aucune normalisation/écriture,
garde et FDs libérés. Archive synthétique déplacée hors du répertoire puis restaurée
en conservant son inode : Run reprend les ajouts à offset 4/checkpoint 9 et provenance
inchangée, nouveau courant seulement après ensemble complet revérifié. Distinction
source neuve, courant missing après localisation et répertoire vidé avec état durable.

Vérifications locales réussies : `go test ./...`, `go vet ./...`, `git diff --check`
et compilation des tests FileSource Linux amd64 sans CGO. Exécution des tests Linux,
détecteur de courses et builds Linux amd64/arm64 sans CGO confirmés réussis par la
[CI lacune de reprise](https://github.com/Coubiac/mailtrace/actions/runs/37178727644)
Go 1.26.x/stable sur `86ff3b1ccac62dba97491309c2f8b5e1ff3419dc`.

Limites : lacune de continuité vérifiable, pas preuve de suppression/perte ni
quantification des messages manqués ; un fichier réécrit peut donner different.
Budget et exclusions gzip/liens applicables, fenêtres/pages non atomiques. Diagnostic
non persisté dans SQLite ; récupération explicite des états inconnus/zéro encore
à développer. Pas de migration/dépendance ajoutée, revue indépendante encore requise.

## Lot 38 : reprise zéro explicite d'une génération en suivi unique

Résultat attendu : reprendre explicitement après acquisition durable mais avant
première ligne, sans reset ni acquisition répétée. PrepareFollowResume reste strict ;
PrepareFollowResumeWithPolicy et Run avec Config.ResumePolicy.AllowZeroCheckpoint
autorisent seulement un parcours complet donnant une seule génération en suivi avec
checkpoint zéro présent. Ouverture du chemin courant uniquement, identité physique,
préfixe non vide et ancre zéro canonique revérifiés. Archives zéro non recherchées ;
ensembles multiples/nil/invalides/unknown restent bloqués. Tout échec ferme l'ouverture,
sans résultat partiel, normalisation, écriture ou transition de lifecycle.

Le propriétaire retient une copie du préfixe validé, revérifiée sans seek avant
transfert et au polling tant que le checkpoint acquitté reste zéro. Divergence avant
transfert conserve le propriétaire/offsets ; après transfert, arrêt avec fermeture.
Premier acquittement positif : contrôles habituels d'ancre et de taille. Diagnostic
different sans lacune de recherche inventée ; courant absent reste ErrCurrentMissing.

Tests : recontrôle portable du préfixe sans consommation/seek et passage à l'ancre
positive ; Linux SQLite, arrêt réellement après acquisition avant lecture, refus strict
sans mutation, replay explicite avec provenance conservée/checkpoint 5, puis reprise
stricte de l'ajout à offset 5/checkpoint 11 sans registration/acquisition répétées.
Préparation seule sans lecture/écriture, refus nil/ancre invalide/préfixe vide/unknown/
ensemble multiple/réécriture/remplacement/missing, annulation et remplacement durant
observation avec fermeture ; réécriture après ready refusée avant transfert, propriété
conservée. Ancien test d'archive zéro distincte du courant reste bloquant (different).

Vérifications locales réussies : go test ./..., go vet ./..., git diff --check,
compilation des tests FileSource Linux amd64 sans CGO. Tests Linux Go 1.26.x/stable,
détecteur de courses FileSource et builds Linux amd64/arm64 sans CGO confirmés
réussis par la [CI reprise zéro](https://github.com/Coubiac/mailtrace/actions/runs/37185193175)
sur `a56bf17815510eb48e1d2562e44055c62c1f7132`. Lot terminé et publié.

Limites : politique explicitement opt-in, pas preuve de continuité par ancre positive
à zéro ; préfixe borné à 4096 octets et contrôles/pages non atomiques. Anciennes
empreintes vides et lifecycle inconnus restent non récupérés. Pas de migration ni
dépendance ajoutée ; revue indépendante toujours requise. Les relances entre le lot
37 et ce lot n'avaient produit aucun nouveau commit ; le travail effectif et les
vérifications de cette reprise sont décrits ici.

## Lot 39 : revue et fusion de la PR #9

La demande de clôture des PR devient prioritaire sur le prochain comportement
FileSource. Résultat : revue du commit M1 exact, correction d'un P2, revérification
puis fusion ; pas de revue SQLite/FileSource globale dans ce lot.

Checkout isolé M1 : revue des parseurs syslog/Postfix, modèle, corpus et CI par le
coordinateur et un auditeur agent indépendant en lecture seule. L'audit a trouvé
qu'un sender entre guillemets contenant `> to=<…>` pouvait créer un faux champ
destinataire avec l'ancienne regex. Scanner respectant guillemets/échappements et
chevrons, régressions (faux to/status/uid, sender vide, smtpd, valeur incomplète),
seed fuzz ajouté. Relecture indépendante du correctif : aucun autre blocage concret.
Rapport et limites dans docs/reviews/pr-9.md ; revue COMMENT consignée sur GitHub,
analyse assistée par agents, sans prétendre à une approbation humaine externe.

Sur M1 corrigé : go test ./..., go vet ./..., builds Linux amd64/arm64 sans CGO,
fuzz Postfix/syslog trois secondes chacun (deux workers), git diff --check réussis.
Manifest de 30 scénarios vérifié : fichiers présents, gzip identique au fichier normal.
La [CI du correctif](https://github.com/Coubiac/mailtrace/actions/runs/37185777332)
réussit sur `0c224a213d3148cef866f72f48e28fd0ced26a78` (Go 1.26.x/stable,
tests/vet/builds Linux). PR sortie du brouillon, fusion par merge avec SHA de tête
attendu, commit `b52e7fb78022049146d87eb69de002fc239e0f12`. Arbre de main identique
à la tête revue ; [CI main](https://github.com/Coubiac/mailtrace/actions/runs/37185831304)
également réussie. Les fichiers sont désormais visibles sur la branche principale.

PR #10 reciblée vers main ; intégration du correctif par merge de main dans sa
branche, commit `6b861179f915917aae5f8c7452194e939b93681e`, sans clôturer sa revue.
Cette branche est intégrée à #11 : pas de réintroduction du parseur vulnérable.
[CI SQLite intégrée](https://github.com/Coubiac/mailtrace/actions/runs/37185908928)
réussie sur `6b861179f915917aae5f8c7452194e939b93681e`.
go test ./..., go vet ./... et git diff --check réussis après intégration dans
FileSource ; parseurs identiques à main. [CI FileSource intégrée](https://github.com/Coubiac/mailtrace/actions/runs/37185969499)
réussie sur `89816990fda14fa09a32ae6e5c873877a62b6803` : tests Linux
Go 1.26.x/stable, détecteur de courses FileSource et builds Linux amd64/arm64 sans CGO.
Lot terminé et publié ; descriptions des trois PR actualisées.

Limites : audit statique/données synthétiques, fuzz court, pas de Postfix réel.
M1 ne livre pas encore de service/paquet installable. Revues sécurité SQLite et
FileSource encore à faire ; main local aligné sur origin/main, branches dépendantes
conservées. Aucun merge de #10/#11 ni nouveau comportement de source dans ce lot.

## Lot 40 : revue et fusion de la PR #10

Revue coordinateur et audit agent indépendant du commit SQLite v1 exact
6b861179f915917aae5f8c7452194e939b93681e, hors FileSource/schéma v2. Deux P2
reproduits via Open/Commit avant correctif : table étrangère sqliteApp/vue seule
admises par un comptage incomplet, et dates RFC5424 hors plage int64 nanosecondes
indexées comme un faux instant. Version négative modifiait aussi WAL avant refus.

Correctifs : comptage de tous objets utilisateur avec préfixe sqlite_ littéral,
avant WAL et dans migration ; refus des versions négatives ; aller-retour exact
des dates avant projection UTC, sinon NULL avec métadonnées intactes et acquittement
normal du record/checkpoint. Open contrôle fichiers réguliers et sidecars Unix
privés (-wal/-shm/-journal) avant connexion ; contexte déjà annulé sans création.
ReadAt/FirstSeen restent des instants actuels fournis par les sources internes.

Tests des bornes int64, voisins ±1ns et années 0001/9999, refus foreign/négatif sans
mutation, fichiers neufs privés, WAL préexistant conservé 0644 par le pilote puis
refusé par Open, SHM/journal trop ouverts, FIFO et annulation. Relecture indépendante
sans blocage concret restant. Rapport docs/reviews/pr-10.md et revue COMMENT GitHub,
sans prétendre à une approbation humaine externe.

go test ./..., go vet ./..., git diff --check, builds Linux amd64/arm64 sans CGO
et compilation des tests SQLite Linux réussis localement. [CI du correctif](https://github.com/Coubiac/mailtrace/actions/runs/37186524966)
réussie sur 463d767418cb366b87aaf983530a42da6bba2f35, incluant tous les tests Unix.
[CI finale de PR](https://github.com/Coubiac/mailtrace/actions/runs/37186639855)
réussie sur 44e54f7b3b551ec5d073a616b18a67842026ad76 (Go 1.26.x/stable,
tests/vet/builds Linux). PR sortie du brouillon puis fusion avec SHA de tête attendu,
merge 69ab91e6a830914102ac96f4b9434be89f5702e6 dans main.

PR #11 reciblée vers main. Merge de main dans FileSource : code fusionné sans
conflit, historique détaillé de ce point de reprise conservé pour résoudre le seul
conflit documentaire. Schéma v2/état de suivi restent propres à #11 ; aucune
modification du schéma v1 ni dépendance. go test ./..., go vet ./... et git diff
--check réussis après intégration. [CI FileSource intégrée](https://github.com/Coubiac/mailtrace/actions/runs/37186772554)
réussie sur bcda85b0e4b80f30d67a44b9bc6145d1753f3346 : Go 1.26.x/stable,
tests Linux, détecteur de courses FileSource et builds Linux amd64/arm64 sans CGO.
Arbre de main identique à la tête SQLite revue ; [CI main](https://github.com/Coubiac/mailtrace/actions/runs/37186695880)
réussie sur 69ab91e6a830914102ac96f4b9434be89f5702e6. Lot terminé et publié,
descriptions des PR actualisées. Les branches #9/#10 sont conservées.

Limites : parent du fichier à protéger, contrôles filesystem non atomiques, ACL
Windows non validées par bits Unix. Audit assisté par agents/données synthétiques,
pas de certification humaine. Débit, licences transitives, sauvegarde et rétention
opérationnelles restent à traiter avant distribution. Revue FileSource encore à faire.

## Lot 41 terminé : revue FileSource — contrats, lecteur et identité

Revue partielle de #11 sur 1968fc494c5191cf6ad8ea4cc9b11f792f2c85ea,
coordinateur et auditeur agent indépendant dans le checkout isolé. Aucun blocage
concret trouvé dans Source/Sink et Record, reader.go, ingestor.go, identity*.go,
anchor.go et resume.go. Contrats d'acquittement/retry et bornes de Record précisés,
sans modification de comportement. Rapport docs/reviews/pr-11-part-1.md.
Tests FileSource et vet sources réussis sur la référence isolée sous Windows ;
auditeur : tests ciblés -count=1 et diff propre. Vérifications finales locales
go test ./..., go vet ./... et git diff --check réussies. Commit du lot
3fe6a7063f88f2c396e18b8a386ebf95a2cd2cc2 publié ; [CI Linux](https://github.com/Coubiac/mailtrace/actions/runs/37187398041)
réussie : Go 1.26.x/stable, tests/vet, détecteur de courses FileSource et builds
Linux amd64/arm64 sans CGO. Tests spécifiques d'identité Linux et de vérification
positive exécutés par cette CI. Aucun test nouveau sans défaut concret à reproduire.

La PR reste en brouillon : ce lot ne valide pas sélection, registration, chemins,
rotation, reprise orchestrée/Run ni lifecycle SQLite v2. Preuves bornées/non
atomiques et sérialisation requise restent des limites explicites du contrat.

## Lot 42 terminé : revue FileSource — décision de génération

Référence c8a5b46a03289819f4b5d53208b94dd8377ee0a5, coordinateur et auditeur agent
indépendant en lecture seule. Aucun blocage concret trouvé dans selection.go et
generation.go : scan paginé borné, unicité/refus, copies, registration acquittée,
attente sur fichier vide et politique zéro explicite. Rapport
docs/reviews/pr-11-part-2.md. Code inchangé, aucun test ajouté sans défaut concret.

Tests portables ciblés Windows -count=1 réussis pour les deux relecteurs.
[CI Linux de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37187511562)
réussie : Go 1.26.x/stable, tests/vet, détecteur de courses FileSource et builds
amd64/arm64 sans CGO, incluant tests Linux de génération et preuves réelles.
Consulter la PR #11 pour le résultat de la CI du commit documentaire publié.

Limites : écritures à sérialiser sur toute la décision, captures non atomiques,
pas de batch pending conservé pour retry automatique de registration. Une réponse
perdue peut laisser une origine durable à checkpoint zéro ; décision stricte
insuffisante et arrêt, sans génération remplacée implicitement. Registration et
acquisition restent distinctes ; récupération unknown encore à cadrer.
La PR reste en brouillon pour les parties restantes.

## Lot 43 terminé : revue FileSource — ouverture et observation du chemin

Revue de 25447608b60b1f0c6abdf96c9a7d3abdd215b3f4, coordinateur et auditeur agent
indépendant : open*.go, path.go et source_path.go. Défaut Windows reproduit : le
FileInfo d'os.Stat charge ses IDs tardivement et peut accepter un remplacement
contre le snapshot préouverture. Helper statPath corrigé : Windows metadata handle,
File.Stat avec IDs chargés, fermeture avant retour ; autres plateformes os.Stat.
Régression du remplacement et test réel de chemin long passent après correction.
Job CI Windows ciblé ajouté. Rapport docs/reviews/pr-11-part-3.md.

Relecture du patch sans autre blocage concret. Tests ciblés Windows -count=1 pour
les deux relecteurs, go test ./..., go vet ./..., git diff --check et compilation
des tests FileSource Linux amd64 sans CGO réussis localement. Correctif publié sur
cded0a87fd22f96220d3f519cb70b85f55f2cf3b ; [CI du correctif](https://github.com/Coubiac/mailtrace/actions/runs/37202322863)
réussie : nouveau job windows-path (substitution et chemin long réellement exécutés),
Linux Go 1.26.x/stable, tests/vet, race FileSource et builds amd64/arm64 sans CGO.
Consulter la PR #11 pour la CI de publication du point de reprise final.
UNC distant/liens Windows non testés localement ; observations non atomiques et
annulation entre syscalls restent des limites. Rotation/Run/lifecycle hors périmètre.

## Lot 44 terminé : revue FileSource — lecteurs d'état SQLite

Référence 9007404264ba9b4a1a99a8563ce0fb9ab2155f40, coordinateur et auditeur agent
indépendant : state.go, contrats de requête/page et tests de lecture. Aucun blocage
concret d'exécution. Paramètres liés, scope source/identité/chemin, ordre/cursor,
Limit+1 borné, snapshot par page et nil/zéro/positif vérifiés. Commentaire de
Checkpoint corrigé : l'absence ne propose plus un redémarrage automatique à zéro.
Rapport docs/reviews/pr-11-part-4.md, aucun comportement modifié ni test ajouté.

Tests TestFileOrigins* sous Windows -count=1 réussis pour les deux relecteurs.
TestLoadPathOriginsWithSQLiteAndReducedFinalPage -count=1 réussi pour le coordinateur,
avec 101 origines et plafond 100 sans résultat partiel. git diff --check réussi.
[CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37202460211)
verte : tests/vet Linux Go 1.26.x/stable, race FileSource, builds amd64/arm64 sans CGO
et job Windows ciblé chemins. Consulter la PR #11 pour la CI du rapport publié.

Limites : pages sans snapshot global, sérialisation des écritures de source requise ;
nombre de résultats borné, taille des chaînes et coût SQL non mesurés. Corruption
ou concurrence non injectées localement dans ce lot. Migration/transitions v2,
rotation et Run encore à relire ; PR en brouillon.

## Lot 45 terminé : revue FileSource — migration et transitions SQLite v2

Référence a0c0d13319a27c38ffad8054cc6b081fbec76299, coordinateur et auditeur agent
indépendant : migration v2, FollowTransition, validation et Commit. Aucun blocage
concret identifié. V1 conservée unknown, DDL/historique/version atomiques, paires
valides, namespace/état attendu ou cible idempotente, refus et rollback du batch
vérifiés. Rapport docs/reviews/pr-11-part-5.md, code inchangé et aucun nouveau test
sans défaut concret à reproduire.

Coordinateur : TestFollow(Transitions|StateMigration) -count=1 ; auditeur : TestFollow*
-count=1, réussis sous Windows. git diff --check réussi, checkout isolé propre.
[CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37202826203)
verte : tests/vet Linux Go 1.26.x/stable, race FileSource, builds amd64/arm64 sans CGO
et job Windows chemins. Consulter la PR #11 pour la CI de publication du rapport.

Limites ADR-009 maintenues : justification filesystem à la source, écritures/réessais
sérialisés par source, pas d'époque de propriétaire ni protection ABA. Pas de crash
réel/concurrence/corruption injectés ; historique non équivalent à validation exhaustive
du schéma. Open/WAL hors transaction de migration. Acquisition/retrait FileSource,
rotation et orchestration encore à relire ; PR en brouillon.

## Lot 46 terminé : revue FileSource — acquisition avant première ligne

Référence 55c820b0ae31387506a1754e97ec8d0edfa40762, coordinateur et auditeur agent
indépendant : runOpened/prepareGeneration/acquireGeneration, tests acquisition et
propriété des successeurs à leur préparation. Aucun blocage concret identifié.
Validation avant acquisition, transition seule avant normalisation/consommation,
réutilisation following/réacquisition retired, erreurs/EOF/annulation et fermeture
vérifiés. Rapport docs/reviews/pr-11-part-6.md, code inchangé et aucun nouveau test.

Coordinateur : TestAcquisitionRejectsInvalidStateAndHonorsCancellation -count=1 ;
auditeur : ce test et TestRunOpenedClosesOwnedDescriptorOnCancellation, réussis sous
Windows. git diff --check réussi, checkout isolé propre.
[CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37203133730)
verte : tests/vet Linux Go 1.26.x/stable, race FileSource, builds amd64/arm64 sans CGO
et job Windows chemins. Intégrations Linux acquisition/erreurs/cleanup relues et
exécutées par cette CI, pas localement sous Windows. Consulter la PR #11 pour la CI
du rapport publié.

Limites : ReadAt bornés de vérification avant acquisition, Sink durable et écritures
sérialisées requis, filesystem/acquisition non atomiques. Registration/acquisition
distinctes ; unknown ou zéro peuvent demander une décision explicite à la reprise.
Retrait/grâce, rotation et reprise globale encore à relire ; PR en brouillon.

## Lot 47 terminé : revue FileSource — retrait durable et grâce

Référence f468a7ad6e2327be58ce0c074b29b1491a03fed3, coordinateur et auditeur agent
indépendant : grace.go, tests grace/retirement et appels du scheduler nécessaires
au retrait et au nettoyage. Aucun blocage concret identifié. EOF stable, protection
courant/partiels/pending, renouvellement après ajout, transition seule acquittée
avant fermeture/libération et erreurs/annulation/Close relus. Rapport
docs/reviews/pr-11-part-7.md, code inchangé et aucun nouveau test.

Coordinateur et auditeur : les trois tests portables TestGrace* -count=1 réussis
sous Windows. git diff --check réussi, checkout isolé propre.
[CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37203460955)
verte : tests/vet Linux Go 1.26.x/stable, race FileSource, builds amd64/arm64 sans CGO
et job Windows chemins. Intégrations Linux grâce/erreurs/cleanup relues et exécutées
par cette CI, pas localement sous Windows. Consulter la PR #11 pour la CI du rapport.

Limites : Sink durable et écritures sérialisées requis ; dernier contrôle, retrait
et fermeture non atomiques, ajout après contrôle potentiellement manqué. Pas de
crash réel injecté. Rotation globale, ordonnancement et reprise orchestrée/Run
encore à relire ; PR en brouillon.

## Lot 48 terminé : revue FileSource — courant et successeurs aux polls

Référence 82635d7e27fa3da5447a6e23fd27925a82f417aa, coordinateur et auditeur agent
indépendant : poll dans rotation.go, taille/ancre, observation, courant/successeur,
capacité et propriété sur erreur. Aucun blocage concret identifié. Contrôles avant
bascule/retrait, same/missing, retour du retenu avec son ingestor, plafond avant
OpenLog, comparaison à l'observation et nettoyage relus. Rapport
docs/reviews/pr-11-part-8.md, code inchangé et aucun nouveau test.

Coordinateur : tests SourcePathFollow, PollStopsOnCurrent, SizeCheck et LiveAnchor
-count=1 réussis sous Windows ; auditeur : tests portables observation/suivi du
chemin/tailles/ancres réussis. git diff --check réussi, checkout isolé propre.
[CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37203882023)
verte : tests/vet Linux Go 1.26.x/stable, race FileSource, builds amd64/arm64 sans CGO
et job Windows chemins. Intégrations Linux rotation/capacité/retour/erreurs/contrôles
relues et exécutées par cette CI, pas localement sous Windows. Consulter la PR #11
pour la CI du rapport.

Limites : contrôles/ouvertures non atomiques, preuves bornées, pas d'injection locale
du changement entre observation et ouverture. Plafond de générations, pas tous les
handles du processus. Ordonnancement et reprise orchestrée/Run encore à relire ;
PR en brouillon.

## Lot 49 terminé : ordonnancement et flux continu

Référence initiale 337ceb56f0cfc28cef868bf52cb5ab19b28fb3c0. Défaut identifié :
CommitNext pouvait consommer une quantité illimitée de fragments jusqu'à LF/EOF,
retardant indéfiniment l'autre génération et les polls sur un flux sans LF.
Correctif relu indépendamment : chemin interne limité à 16 fragments/64 Kio par
génération/passage, yield conservant les fragments sans normalisation ni checkpoint,
progression vers autre génération/poll. Contrats Next/CommitNext publics conservés.

Régression portable échoue avant (196608 octets lus), passe après (au plus 65536).
Test plusieurs yields/oversized/annulation/pending EOF/réessai identique/ancre réussi.
Tests ciblés, go test ./..., go vet ./..., git diff --check et compilation des tests
Linux amd64 sans CGO réussis localement sous Windows ; relecture du patch sans
blocage restant. Nouveau test conjoint Linux exécuté en CI : longue ligne de
l'ancien cède au successeur sans avancer son checkpoint ni fuite.
Rapport docs/reviews/pr-11-part-9.md. Correctif publié
cd931e08886677511b8521b32553c3714c0d3019 et
[CI réussie](https://github.com/Coubiac/mailtrace/actions/runs/37204731135) :
tests/vet Linux Go 1.26.x/stable, nouveaux scénarios Linux, race FileSource, builds
amd64/arm64 sans CGO et job Windows chemins. Consulter la PR #11 pour la CI de la
clôture documentaire.

Limites : budget en octets, pas en temps ; syscalls/normaliseur/Sink peuvent retarder
un passage. Aucun benchmark/crash réel ajouté. PR reste en brouillon.

## Lot 50 terminé : classification des états durables de reprise

Référence 848aa0694591e735ff9153d43b68e404aa8c8582, coordinateur et auditeur agent
indépendant : LoadFollowOrigins et dépendance LoadPathOrigins. Aucun blocage concret
identifié. Classement après parcours complet, retired écartés, priorités invalid >
unknown > capacity, absence de candidats partiels sur refus/limite/erreur/annulation,
copies et nil/zéro distincts relus. Rapport docs/reviews/pr-11-part-10.md, code inchangé.

Coordinateur : TestLoadFollowOrigins* -count=1 ; auditeur : ces tests et
TestLoadPathOrigins*, intégration SQLite incluse, réussis sous Windows. 101 états
dont 99 retired et deux following, limite 100 refusée, isolation source et aucune
mutation vérifiées. git diff --check réussi, checkout isolé propre.
[CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37204843549)
verte : tests/vet Linux Go 1.26.x/stable, race FileSource, builds amd64/arm64 sans CGO
et Windows chemins. Consulter la PR #11 pour la CI de publication du rapport.

Limites : filtrage source assuré par le lecteur, pages sans snapshot global et
écritures sérialisées ; métadonnées encore non vérifiées, pas de preuve de présence
ou de reprise sûre. Taille des chaînes/coût SQL non mesurés. PR reste en brouillon.

## Lot 51 terminé : recherche de rotation

Référence initiale dcc1455c1978ebd3431366f908599748e685de7d. Reprise après quota,
aucun changement laissé par l'interruption. Scan/pages/budget/exclusions/preuves,
refus sans chemin partiel et fermeture relus par coordinateur et auditeur indépendant.
Deux défauts Windows reproduits : os.Stat du répertoire pouvait charger les IDs
après substitution ; entry.Info fallback pouvait attribuer à l'entrée précédente
l'identité du remplacement. Corrigés par statPath(directory) et helper Windows
figeant les IDs puis comparant un snapshot cohérent avant OpenLog. Linux inchangé.

Régressions Windows échouant avant, passant après ; entrée inchangée éligible,
diagnostic de preuve physique indisponible conservé. Tests ciblés, go test ./...,
go vet ./..., git diff --check et compilation des tests Linux sans CGO réussis.
Patch relu indépendamment sans blocage restant ; job Windows étendu aux régressions.
Rapport docs/reviews/pr-11-part-11.md. Correctif publié
de81c4e86fc3deab0692bf471ae5ffeb6aa8f6b9 et
[CI réussie](https://github.com/Coubiac/mailtrace/actions/runs/37220202856) :
tests/vet Linux Go 1.26.x/stable, race FileSource, builds amd64/arm64 sans CGO et
job Windows étendu exécutant les trois nouveaux tests. Consulter la PR #11 pour
la CI de publication de cette clôture documentaire.

Limites : fallback Windows simulé, pas de test réel sur autre filesystem ; identité
acquise au chargement, erreurs natives SameFile masquées/refus sûr. Aucun faux unique
Windows démontré, reprise persistante toujours Linux seulement. Snapshots non atomiques,
chemin à rouvrir/revérifier. PR reste en brouillon.

## Lot 52 terminé : localisation de l'ensemble en suivi

Référence e95f84ecc53096bd8af11d67356bf78075f9a103, coordinateur et auditeur agent
indépendant : LocateFollowOrigins et tests. Aucun blocage concret identifié.
Validation/copie complète avant recherche, budget partagé et plafond par scan,
première décision bloquante sans chemins partiels, collision, nil/zéro, causes
erreurs/annulation et propriété temporaire relus. Rapport docs/reviews/pr-11-part-12.md,
code inchangé et aucun nouveau test.

Coordinateur et auditeur : quatre tests portables TestLocateFollowOrigins* -count=1
réussis sous Windows. git diff --check réussi, checkout isolé propre. Intégrations
Linux localisation réelle/1–2 fichiers/refus/limite/fermeture relues et exécutées par
la [CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37220323146),
pas localement sous Windows. CI verte : tests/vet Linux Go 1.26.x/stable, race FileSource,
builds amd64/arm64 sans CGO et Windows chemins. Consulter la PR #11 pour la CI du rapport.

Limites : namespace fourni par l'appelant, écritures sérialisées, recherches non
atomiques et preuves bornées, chemins à rouvrir/revérifier. Budget d'entrées examinées,
pas de temps réel. Reprise persistante Linux seulement. PR reste en brouillon.

## Lot 53 terminé : réouverture de l'ensemble localisé

Référence a04afbb42fa93a678a4316017ba8b019f0a70894, coordinateur et auditeur agent
indépendant : OpenFollowLocations/OpenedFollowSet. Aucun blocage concret identifié.
Validation/copie entière avant disque, propriété immédiatement après open, collision
physique, preuves strictes/Seek sans consommation, erreurs/annulation sans ensemble
partiel, causes de nettoyage et Close idempotent relus. Rapport
docs/reviews/pr-11-part-13.md, code inchangé et aucun nouveau test.

Coordinateur et auditeur : deux tests portables ciblés -count=1 réussis sous Windows.
git diff --check réussi, checkout isolé propre. Intégrations Linux réouverture réelle,
copies, collisions/échec du second fichier et fermeture relues et exécutées par la
[CI de la référence revue](https://github.com/Coubiac/mailtrace/actions/runs/37220607670),
pas localement sous Windows. CI verte : tests/vet Linux Go 1.26.x/stable, race FileSource,
builds amd64/arm64 sans CGO et Windows chemins. Consulter la PR #11 pour la CI du rapport.

Limites : namespace garanti par l'appelant, écritures sérialisées, snapshots non
atomiques ; propriétaire à ne pas copier/utiliser concurremment. Reprise persistante
Linux seulement. PR reste en brouillon ; critère de fin et reste M2 indiqués en tête.

## Lot 54 terminé : observation du courant

Référence c7623d21c63e3c2ba0d21dd0ddbd84564bd3b8df, coordinateur et auditeur indépendant.
Pas de blocage d'exécution ; tous fichiers inspectés, known/missing/new/capacity et
propriétaire préservé vérifiés. Commentaire/ADR précisés : handle metadata temporaire
Windows, pas lecture de données/fermeture des fichiers détenus. Rapport partie 14.
Quatre tests TestObserveCurrent* -count=1 et diff Windows réussis. Intégrations Linux
relues et exécutées par CI de référence 37221250736 verte, pas localement. CI de
publication sur PR #11. Limites : observations non atomiques, known sans preuve de
continuité ; état/set sérialisés, décision à revérifier. PR toujours en brouillon.

## Lot 55 terminé : transfert connu et refus du courant absent

Référence `8bcc1a7bd137155248235cefbec3477b81bcd9cd`, coordinateur et auditeur
indépendant : pas de blocage concret. Propriété conservée avant transfert, collection
vidée avant scheduler/cleanup exclusif ; missing canonique reobservé sans ingestion,
attente ou écriture. Rapport partie 15. Cinq tests ciblés Windows réussis ; intégrations
Linux relues et exécutées par CI 37221589000 verte, pas localement. Code inchangé,
diff propre ; CI de publication sur #11. Fenêtres d'annulation avant/après transfert
relues sans injection ciblée ; snapshots non atomiques, état/set sérialisés requis.

## Lot 56 terminé : nouveau courant au transfert

Référence `c5e928fb762e7445c67f16ea57557e5c5bc2c1ca`, coordinateur et auditeur :
aucun blocage identifié, capacité/preuves avant open, identité/cleanup temporaire,
propriété complète après transfert, préparation/acquisition avant ligne et courant
vide différé. Rapport partie 16. Test portable huit sous-cas Windows réussi ; Linux
relu et exécuté par CI 37221815044 verte, pas localement. Code inchangé ; CI de
publication sur #11. Snapshots non atomiques, unknown après registration sans
acquisition reste bloquant, écritures/set sérialisés. PR en brouillon.

## Lot 57 terminé : préparation orchestrée stricte

Référence `4e2c1fa890cc7778edb0303e66ac28412287c440`, coordinateur et auditeur :
aucun blocage concret. Validation avant état, composition entière sans fallback,
Absent après parcours complet, Ready propriétaire complet sans ingestion/écriture,
cleanup de l'ensemble sur erreur/annulation avec causes jointes. Rapport partie 17.
Deux tests Windows (19 sous-cas) réussis ; Linux relu et exécuté par CI 37221985374
verte, pas localement. Code inchangé ; CI de publication sur #11. Namespace lecteur,
état sérialisé, pages/preuves non atomiques. PR en brouillon.

## Lot 58 terminé : politique zéro et diagnostic de lacune

Référence `4e2c1fa890cc7778edb0303e66ac28412287c440`, coordinateur et auditeur :
aucun blocage concret. Replay opt-in following unique au courant avec preuves
canoniques/non vides, préfixe retenu jusqu'à positif ; gap absent/different seulement
après localisation complète, sans fallback/reset, causes et diagnostic fixe.
Rapport partie 18. Deux tests Windows réussis, Linux relu/CI 37221985374 verte ;
lot 57 documentaire sans changement d'exécution/CI 37222118286 verte. Publication
sur #11 ; snapshots/preuves bornés, état sérialisé. PR en brouillon.

## Lot 59 terminé : configuration et démarrage Run

Référence `c1782f7fab72555ddf3e0d490ce1f92e2ff0bf7c`, coordinateur et auditeur :
aucun blocage concret. Config copiée/bornée, lecteur PathStateReader explicite,
garde unique, fallback seulement absent complet, cleanup complet avant/après
transfert, attente vide avec fermeture/réouverture, erreurs Sink sans retry.
Rapport partie 19. New/RunOpened réussis Windows ; concurrent skipped Windows et
exécuté par CI Linux 37222118286 verte, comme les intégrations relues. Code inchangé,
publication sur #11. ADR précise retired zéro : préfixe vérifié avant acquisition,
pas preuve retenue aux polls (branche following seule). Contrat actuel, pas blocage
de cette revue ; extension à traiter séparément. État sérialisé, observations bornées
non atomiques. PR encore en brouillon.

## Lot 60 terminé : revue finale et fusion FileSource

Référence `ba1a20eaba5c72a02304c404e34bc46b925ea501`, coordinateur et auditeur :
tous composants couverts par 19 rapports, aucun blocage identifié restant, correctifs
Windows/quantum relus et CI consignées. Rapport final `docs/reviews/pr-11.md`.
CI de référence 37222381679 verte ; diff/checkout isolé propres, aucun rerun sans
risque nouveau. Clôture `12eba5a0a45520298691c101ec6f1f1e08c0d145` publiée,
CI 37222534218 verte puis ready et fusion avec tête attendue. PR #11 merged,
main `27b9d98bba1f51749f10e8b9930e9c4da0302068` vérifiés ; CI push main
37222669898 verte (API REST, connecteur sans ce run). Limites et travaux M2
différés conservés ; audit assisté par agents, sans certification externe.

## Lot 61 terminé : cadrage de récupération explicite

Contrat ADR-009 relu indépendamment : RecoverUnknownCurrent standalone, ID exact,
garde partagée, scan entier borné, unique non-retired unknown ou following idempotent,
preuves courant positif/zéro canonique puis PathSame, transition seule sans ingestion.
Acquittement perdu/annulation/fermeture sans compensation ; reader/Sink même stockage,
écritures sérialisées. Zéro lifecycle n'autorise pas implicitement son replay par Run.
Pas d'implémentation ni nouveau test sur ce lot de cadrage. README/avancement mis à jour :
quatre jalons restants, environ 50–90 lots après lot 60, M2 8–18, premier chantier 4–5.
Diff propre ; CI 37241041999 verte sur `51933e0`, PR #12 créée et attachée.

## Lot 62 terminé : classification entière sans journal

LoadRecoveryOrigin implémenté sur base `51933e0`, relu indépendamment sans blocage.
Unique non-retired ID demandé, unknown/following seulement ; invalides, retired cible,
concurrents/ID discordant et limite refusés sans candidat. Copies et nil/zéro bruts,
erreurs/annulation résultat vide, aucune ouverture/écriture. Rapport docs/reviews/pr-12.md.
Trois tests ciblés Windows, suite locale/vet/diff réussis ; CI de publication à vérifier
sur #12 : 37241340339 verte sur `f0067dd74f2d63f8d52ea096c6264f54b1fa3cb6`.
Limites reader/namespace et pages non atomiques, écritures à sérialiser.

## Lot 63 terminé : préparation du courant et preuves

Base `f0067dd`, helper privé prepareRecoveryCurrent/Close opaque implémentés, revue
indépendante sans blocage. Copie avant open, preuve positif/zéro canonique puis
PathSame/identité, propriétaire immédiat, cleanup errors/cancel joints. Aucun Seek,
ligne ou état écrit, retour possédé/Close idempotent. Rapport PR #12.
Deux tests ciblés Windows, suite locale/vet/diff et compilation Linux réussis.
Linux relu, exécution/race/builds vérifiés par CI 37241721021 verte sur `daa7b05` ;
pas exécuté localement. Limites preuves bornées/observations non atomiques, état sérialisé.

## Lot 64 terminé : récupération explicite et transition seule

Base `daa7b05`, opération public RecoverUnknownCurrent implémentée, revue indépendante
sans blocage. Garde jusqu'après cleanup, scan/preuves puis unique unknown→following,
retry following sans Commit, retired refusé ; statut/CP/provenance conservés et aucune
ingestion. Sink errors/EOF/ACK perdu sans retry/compensation ; annulation avant/aprèsACK.
Rapport PR #12. Test portable onze cas, suite Windows/vet/diff/compilation Linux réussis.
Linux relu (SQLite, ACK/cancel/Close/retry/zero strict), exécution par
CI 37242113816 verte sur `31520075b7fc9ad18a30c23ace1a7c3dd356d96b`.
Reader/Sink même stockage, écritures sérialisées, preuves non
atomiques. Opération bibliothèque, pas de CLI/service/récupération multiple/archives.

## Lot 65 : synthèse et clôture de récupération explicite

Revue indépendante du checkout publié `31520075` : les huit fichiers recovery*.go
correspondent aux lots relus 62–64, anciens composants/dépendances/workflow inchangés.
Aucun blocage identifié ; classifications/preuves/transition seule, réessai following,
refus retired et séparation lifecycle/replay zéro cohérents avec ADR-009.
CI 37242113816 terminée avec succès : tests Linux, vet, race, builds sans CGO et
Windows chemins. Diff propre ; pas de test relancé sans modification d'exécution.
Rapport intégré complété et récupération unique retirée des travaux futurs.
Clôture `ca5db281d776e9e5c27874386212d78f520eedbc` publiée, CI 37242452694 verte,
ready puis fusion #12 avec tête attendue. Main `69dfe6b`/CI 37242571458 verte vérifiés.

## Lot 66 : capture privée du départ initial à EOF

Résultat attendu : une taille de descripteur capturée, ancre bornée de 4096 octets
maximum sans Seek, LF final exigé, pas d'écriture ni lecture de lignes. Implémenté
avec zéro vide canonique, append hors snapshot ignoré, EOF partiel refusé explicitement,
erreurs/troncature/cancel sans résultat. ADR-010 définit le futur bootstrap initial
sans historique, distinct des reprises et rotations ; ces chemins restent inchangés.
Trois tests Windows -count=1, suite/vet/diff réussis. Test de fichier fermé adapté
à l'erreur Stat Windows (invalid handle, pas fs.ErrClosed garanti). Revue indépendante
sans blocage, rapport pr-13.md ; obligation vide→append depuis zéro précisée pour 67.

## Prochaine action concrète

Publier le lot 66, ouvrir la PR du chantier et vérifier sa CI.
Lot 67 : configuration start_at:end, preuve d'absence historique, registration/Run
initials avec tests SQLite Linux ; aucune application aux reprises/rotations.
Réutiliser le checkout géré `C:\Users\benoi\.codex\worktrees\review-m1\mailtrace`
pour la tête publiée du nouveau chantier ; principal sur codex/m2-start-at-end.

## Suite à découper au fil des reprises

1. Démarrage explicite start_at:end et compléments du suivi #4.
2. Autres décisions insuffisantes/inconnues, dont anciennes empreintes vides et
   ensembles multiples ; diagnostics/compléments du suivi #4.
3. Import historique normal, puis gzip dans un lot distinct.

Chaque demande de continuation traite par défaut un seul petit lot et actualise
ce point de reprise avec le résultat et la prochaine action.
