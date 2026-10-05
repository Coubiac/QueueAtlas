# ADR-011 — import historique borné et identité de contenu

Statut : prévalidation normale/gzip lots 69–71 fusionnée ; copie vers writer lot 72
et fichier privé détenu lot 73 publiés, relus sans blocage et CI vertes.
Lots 72–74 fusionnés dans #15, CI finale verte. Lot 75 identité source-scopée publié,
CI verte ; lots 76–77 migration/lecture et trace de préparation publiés/CI vertes.
Lots 75–79 manifest fusionnés dans #16, CI finale et main vertes. Lot 80 :
préparation d'ingestor publiée dans #17, CI verte. Lot 81 CommitNext publié, CI
verte et revue sans blocage. Lot 82 Binding publié/CI verte, revue sans blocage.
Lot83 Attempt.Run un fichier développé et relu sans blocage ; orchestration globale future.

## Décision et séparation des étapes

Le cadrage #5 exige ordre explicite, gzip validé jusqu'à EOF, SHA-256 décompressé,
reprise/idempotence et bornes. Commencer par une inspection en flux avant ingestion :
taille et SHA-256 du contenu entier, séparateurs et éventuel suffixe partiel inclus.
Pas de normalisation, observation, checkpoint ni écriture du manifest à cette étape.
La lecture ne déduplique pas les lignes identiques : elles restent des octets distincts.

`InspectPlain(ctx, reader, maxBytes)` utilise un buffer fixe de 32 Kio. Limite positive
obligatoire, au plus limite+1 octets consommés pour distinguer EOF exact d'un excès,
sans débordement int64. Hash hex minuscule seulement après EOF réussi. Une erreur,
un dépassement ou une annulation ne rend aucune métadonnée partielle exploitable.
Lectures vides répétées bornées ; lecteur et fermeture restent sous responsabilité
de l'appelant, qui fournira fichier régulier et deadline de l'import. Annulation
entre lectures bornées, pas interruption d'un syscall de lecture bloqué.

Le résultat indique si un suffixe ne se termine pas par LF. Ce suffixe doit être
traité explicitement par l'importeur : son SHA peut être connu mais cela ne signifie
ni que tous les records sont ingérés ni qu'un import_run peut devenir complete.
Le lecteur de lignes conserve ses bornes et ne normalise pas seulement un suffixe.

## Étape réalisée et lots suivants

1. Réalisé aux lots 72–73 : conserver les octets décompressés inspectés dans une
   copie privée bornée, SHA au même passage, writer fermé puis reader détenu.
   Entrée régulière ouverte avec propriété explicite ; échec/annulation refuse la
   copie et tente son cleanup, les erreurs de suppression étant signalées.
2. Suite : ordre/nombre de fichiers/durée globale bornés.
   Ne pas déduire une identité de contenu d'un chemin, inode ou en-tête gzip ;
   recompressions/renommages identiques doivent pouvoir retrouver la même origine.
3. Lot 76 : migration/lecture source-scopée du manifest développées, v1/v2 inchangés.
   Lots 77–78 : écriture de préparation puis contenu/progression développées dans
   le Commit du Sink. Application future : prouver EOF/ancres avant complete.
4. Application du contenu validé à parser/Sink, reprise du checkpoint exact,
   depuis la copie privée validée. Une inspection n'est pas un snapshot filesystem
   et ne prouve pas un second passage identique de l'entrée originale.
5. CLI import hors service actif, ordre fourni sans tri implicite, recalcul M3,
   contraintes globales et scénarios de chevauchement avec suivi continu.

La stratégie d'identité ci-dessous précède le lot SQL. Pas de déduplication sur
seul hash de ligne. Reconnaître un
chevauchement FileSource uniquement avec provenance/positions et preuves concordantes ;
sinon le signaler incertain. Conserver les hypothèses des timestamps sans année.

## Limites actuelles

Les briques d'inspection/copie normale/gzip et préparation de fichier détenu existent.
Migration/lecture/manifest fusionnés ; ingestion élémentaire développée dans #17.
Pas de Source.Run import, CLI ni garantie de snapshot atomique. Les tests
restent synthétiques ; aucun accès Web à des chemins locaux d'import. MIT conservée,
authentification AD/OIDC après MVP.

## Prévalidation gzip du lot 70

InspectGzip utilise compress/gzip en mode multistream par défaut : chaque membre
et son trailer CRC/taille puis EOF final doivent réussir. Le digest porte seulement
sur les octets décompressés, quel que soit le nom/en-tête, niveau ou recompression.
Gzip vide valide rend le digest vide ; entrée compressée vide sans en-tête échoue.
Suffixe partiel signalé comme dans InspectPlain. Pas de succès d'import implicite.

GzipLimits exige trois limites entières positives : octets décompressés, octets
compressés (en-têtes compris) et ratio maximum. Deux budgets indépendants jusqu'à
limite+1 au niveau de leurs Reader, mémoire fixe ; les prélectures internes restent
distinctes des octets livrés. Contrôle du ratio après chaque lecture décompressée
sur les octets compressés consommés, prélecture comprise. Division/reste évitent
le débordement d'une multiplication. Un préfixe trop expansif est refusé même si
un membre ultérieur peu compressible aurait abaissé le ratio final.

Le lecteur compressé vérifie contexte/budget/n valide et borne 100 lectures vides :
la lecture d'en-tête gzip via io.ReadFull peut autrement boucler avant toute sortie.
Le décodeur est fermé et sa cause jointe sur retour ; l'input reste à l'appelant.
Erreurs/CRC/troncature/junk/limites/annulation ne rendent aucune métadonnée complète.
Tests synthétiques : identité normal/gzip/recompression, membres successifs,
second membre corrompu, CRC/taille/header/troncature/junk, budgets/ratio, contexte,
reader invalide et sans progrès. Exécution Linux vérifiée par CI 37244378495 verte
sur `8c48d1d7bdefa8e27aa21f0a8c3efd0a56ef4466`.

## Copie pendant inspection du lot 72

CopyPlain/CopyGzip écrivent vers un io.Writer fourni les mêmes octets utilisés pour
le digest, au même passage, y compris séparateurs/suffixe. Pour gzip, seuls les
octets décompressés sont copiés, tous les CRC restent requis. Budget contrôlé avant
Write ; aucune copie d'octets au-delà du budget. Pas de fermeture ou de seek.

Une sortie peut être partielle, voire contenir tout un payload CRC invalide, avant
l'erreur finale : le caller doit la jeter, sans ingestion. Writer nil refusé avant
lecture ; n invalide/short write/erreur/cancel échouent sans metadata et sans retry.
Si lecture et écriture échouent au même appel, leurs causes sont jointes, sauf EOF
exact qui reste un succès de lecture. Contrats io et usage exclusif requis.
Deadline entre lectures/écritures, sans interruption d'un syscall bloquant.

Ce mécanisme prépare la copie privée détenue du lot suivant ; une simple sortie
io.Writer n'est pas encore un fichier privé, ni un snapshot de l'entrée originale.

## Copie privée détenue du lot 73

PrepareRegular valide options/contexte, résout les chemins puis réutilise OpenLog
pour une entrée régulière possédée jusqu'à fermeture. Gzip est un choix explicite
d'appelant, pas inféré par le nom ou l'API Web. Plain utilise ContentBytes seulement,
gzip exige également CompressedBytes et MaxRatio positifs. Deadline globale au caller.

Une sous-directory MkdirTemp 0700 et un CreateTemp 0600 sont créés sous TempDir
(défaut os.TempDir), dont le parent doit être protégé/de confiance. Copie/digest au
même passage, writer fermé puis ouverture read-only à offset zéro, taille contrôlée.
Le résultat opaque PreparedContent fournit Info, Read, ReadAt, Seek et Close, un seul
consommateur. Les mutations ultérieures de l'entrée ne changent pas la copie validée.
Partiel reste metadata, sans déclarer l'import complet. La copie est éphémère, pas
un manifest durable et pas un snapshot atomique du fichier d'origine en mutation.

Les erreurs, annulations ou fermeture d'entrée en échec refusent le résultat et
ferment/nettoient la copie. Close libère la propriété avant fermeture/removal,
joint les causes et reste idempotent même sur échec : les removals échoués sont
signalés, sans retry silencieux. Remove de son fichier puis de son directory vide
seulement ; jamais de suppression récursive ou du parent TempDir/entrée d'origine.
Windows ACL non vérifiées ; protections de bits Unix exécutées en CI Linux verte
37245585832 sur `93cf83cec9a1a39dff5600f93ddd5a0bb93fc745`. La prochaine
application devra jeter la copie sur toute erreur et fermer le propriétaire final.

## Identité durable du lot 75 et contrat du manifest suivant

ImportOriginID prend l'ID exact et non vide de la source et le SHA-256 entier
décompressé, exactement 64 caractères hexadécimaux minuscules. Il refuse toute
autre représentation, sans trim ni casse implicite, et ne prouve aucune lecture
ou validation de fichier. Le caller fournit le digest d'une préparation réussie.

Contrat stable : `import-v1:` suivi du SHA-256 hex minuscule de la concaténation
des octets `queueatlas/import-origin/v1` puis NUL, de la longueur en octets de
l'ID source sur huit octets big-endian, de l'ID source, puis des 32 octets du digest.
Préfixe versionné et longueur évitent les ambiguïtés de concaténation. IDs source
opaques, octets Go exacts sans transcodage ou validation UTF-8 ; espaces/casse/NUL/
Unicode conservés. Un test golden fixe le format durable.
Renommage ou recompression du même contenu retrouve cette origine dans la même
source ; un digest ou une source différents donnent une origine différente.
La provenance garde chaque offset : deux lignes identiques ne sont pas supprimées.

Le lot 76 ajoute une migration v3 sans modifier les SQL v1/v2.
Un run sera une tentative explicite avec ID positif fourni par l'appelant,
globalement unique dans la base, y compris parmi les lignes legacy,
source de kind import distincte du suivi live, chemin original, état, digest/taille
du contenu validé, origine associée et offsets. Les lignes v1 sans source restent
héritées, non attribuées : aucune adoption ou reprise implicite. Le lien doit être
source-scopé, l'identité de contenu immuable ; les lecteurs n'interprètent pas
le chemin comme preuve. Préparation échouée sans digest valide ne doit pas produire
une origine ni des records ; la tentative peut être tracée failed sans contenu.

L'écriture du manifest est intégrée au même Commit que records/checkpoint,
avec réessai identique après ACK perdu, sans avance séparée. Complete exigera EOF
validé, absence de suffixe partiel et offset égal à la taille ; cette validation
ne se déduit pas du SHA seul. Ces contrôles SQL existent au lot 78 ; application
future : Sink vérifie metadata/progression, preuve du
fichier à l'appelant. Mono-écrivain/service arrêté au MVP. Aucune déduplication
inter-source prouvée par ImportOriginID, aucun chemin fourni par l'API Web.

## Migration et lecteur du lot 76

V3 reconstruit seulement import_runs avec les huit colonnes v1 conservées et
source_id/generation_id/trailing_partial nullable. Les anciennes valeurs, même
sans digest canonique ou sans fin renseignée, sont copiées sans adoption. Le DDL,
l'historique et user_version=3 sont dans la même transaction ; refus d'historique
manquant ou version future conservé. Index digest recréé, index source/origine ajouté.

Pour une ligne associée : ID positif, source/chemin non vides, FK source et FK
composite source/origine, SHA canonique et taille/partial présents ensemble ; longueur
TEXT et BLOB égales à 64 avec GLOB hex minuscule pour exclure aussi les NUL. Sans
contenu, offset zéro et état running/failed seulement. Running sans fin, états
terminaux avec fin >= début. Offset <= taille, complete seulement à taille exacte
sans partial. SQL ne prouve ni la lecture du fichier ni le checksum : caller requis.

ImportStateReader.ImportRun effectue une lookup exacte source/ID positif et une
jointure unique vers source/origine. Legacy ou autre source : absence, distincte
de zéro. Kind import, ID dérivé, fingerprint sha256 et absence d'identité physique
contrôlés avant exposition ; état incohérent refuse avec résultat vide. Dates UTC,
pointeurs rendus possédés par caller. Aucun write/adoption, décision de reprise,
checkpoint ou page globale implicite. Une lecture de checkpoint ultérieure est
un autre snapshot ; sérialiser les écritures/réessais reste à l'application.

## Trace de préparation du lot 77

Batch.ImportChange optionnel porte Before attendu (nil = création) et Target.
Première étape : créer running sans contenu, puis tracer failed avec fin après
préparation refusée. Source kind import, IDsource exact, IDrun global positif,
chemin/début immuables, dates représentables en nanosecondes int64 sans wrapping.
Une reprise d'écriture accepte le Target déjà identique ; état périmé, ID étranger/
legacy ou tentative terminale à ranimer refusés. Le caller conserve son batch
et tous les pointeurs sans mutation, sérialise écritures et réessais de sa source.

Source et changement de run sont commités dans la même transaction. Refus SQL,
annulation ou conflit n'acquittent rien, même si un ACK peut être perdu après un
commit durable. Pas de retry interne. Origines/records/checkpoints/contenu ne sont
pas acceptés avec une trace de préparation ; records/checkpoints d'import sans
changement explicite du manifest refusés. La lecture interne utilise le même Tx.
Le lot 78 ajoute association de contenu et progression atomique ; aucune
lecture de fichier ni appel PrepareRegular/normalizer n'est effectué par ce Sink.

## Contenu et progression transactionnelle du lot 78

ImportChange peut associer une fois un Content à un running sans contenu, ou créer
une tentative préparée running à un checkpoint existant concordant. Aucun record
au moment d'associer. Contenu immuable dans le run ; taille/partial/SHA doivent
aussi concorder entre tentatives du même contenu/source. Renommage représenté
par nouvelle tentative avec même origine, chemin/début de chaque run immuables.

Origine fournie au plus une : ID dérivé, chemin de tentative, fingerprint sha256,
device/inode vides. Origine persistée également contrôlée même si non fournie.
Avant records/checkpoints, expectedstate et checkpoint de départ vérifiés dans Tx :
offset Before pour progression, Target pour association ; nouveau zéro exige
registration explicite, positif exige checkpoint existant. Aucun saut arbitraire
ni réparation de position incohérente.

Records contigus depuis Before.LastOffset jusqu'à Target.LastOffset, même origine.
Checkpoint fourni au plus un, même offset/origine et anchor non vide. Après writes,
checkpoint durable doit égaler Target et l'anchor proposé si présent ; un anchor
différent à offset égal ne peut pas être ignoré puis acquitté. Manifest, records,
events, checkpoint et source rollback/commit ensemble.

Target déjà identique accepte réessai après ACK perdu, avec CP durable au moins à
son offset. À offset égal, vérifier aussi l'anchor fourni. Une tentative ultérieure
peut avoir avancé le CP partagé ; l'ancien Target reste accepté sans le modifier.
Complete exige taille exacte et aucun suffixe partiel. Vide validé peut devenir
complete à zéro après association ; suffixe partiel peut laisser des records
complets puis une tentative failed au dernier offset acquitté.

Sink ne calcule pas le digest, ne valide pas gzip/EOF, ne décode pas l'anchor et ne
relit pas le fichier. Preuves, bornes et batch conservé restent à l'importeur futur ;
un état SQL canonique ne certifie pas le contenu. Mono-écrivain, aucune compensation
sur erreur d'ACK.

## Préparation d'ingestor du lot 80

NewIngestor lie PreparedContent détenu à Identity kind import, run running associé
et position source-scopée. SHA entier/taille/partial/ID dérivé égaux à Info, offset
égal au LastOffset et dans le contenu ; run explicite/ID positif/début nano exact.
Ancre canonique vérifiée par CaptureAnchor sans déplacer le reader, taille actuelle
égale à Info et byte avant checkpoint positif LF. Zéro peut être prouvé ici grâce
au digest entier de la copie validée, sans prétendre à une preuve d'ancre vide live.

Seek seulement après preuves et vérification contexte ; LineReader créé sans lire
de record ni normaliser. Position et RunState exposent état acquitté, content retourné
copié pour éviter mutations par caller. Pas de Sink/manifest/CommitNext dans ce lot.
Le caller conserve Close, consommation exclusive et bytes immuables dans répertoire
protégé ; le constructeur ne rehash pas la copie ni ne rouvre le path original.
Copie privée de taille changée refusée ; immutabilité des bytes au même descripteur
reste une hypothèse explicite de cette propriété. Erreur/cancel après Seek peut
laisser position déplacée ; aucun transfert de propriété ni cleanup implicite.

## Application élémentaire du lot 81

CommitNext consomme une ligne LF complète depuis la copie détenue : ancre sur
les octets physiques jusqu'à End, normalization bornée, SourceID imposé, record/
checkpoint/ImportChange commités ensemble. Ligne surdimensionnée : prefix borné
unknown avec erreur, End et ancre conservent toute la provenance physique.
Record staged avant toute preuve qui pourrait échouer ; pas de saut à la ligne
suivante après cancellation ou erreur d'ancre. Batch entièrement construit avant
appel Sink, retenu tel quel jusqu'au nil, sans nouvelle normalization ni date.
Read/proof/cancel/Sink error laisse état acquitté intact ; aucune compensation
failed sur ACK ambigu. Caller sérialise, conserve la copie et décide du retry.

À EOF fini : commit terminal seul, sans record ou checkpoint artificiel. Copie
complète sans suffixe, offset exact taille : complete puis io.EOF. Suffixe partiel :
failed au dernier LF acquitté puis ErrImportPartial ; suffixe ni normalisé ni jeté
comme si ingéré. Date terminale conservée au retry, au moins CreatedAt même si
l'horloge recule. Appel ultérieur terminal sans écriture. RunState copie les pointeurs.
Un Sink peut rendre lui-même io.EOF ou ErrImportPartial sans ACK : le caller doit
vérifier RunState.Status terminal pour reconnaître une fin, pas seulement l'erreur.
Read/anchor error non terminal : retry possible, caller décide fermeture et reprise
durable ; CommitNext ne possède pas la copie et ne gère pas plusieurs fichiers.

## Association prouvée du lot 82

PrepareBinding reçoit une copie déjà validée détenue et une tentative running
acquittée ; lit le CP exact source/ID dérivé du contenu. Run non préparé : sa position
initiale doit être zéro, Target peut reprendre le CP partagé prouvé ; CP absent :
nouveau zéro explicite. Run déjà associé : CP obligatoire, metadata et LastOffset
exactement concordants, aucune adoption de l'avancement d'une autre tentative.
NewIngestor prouve taille/digest/ancre/LF sur la copie avant tout Sink.

Binding conserve le batch d'association (origine, CP exact et Before/Target), sans
record ni date variable. Commit expose l'ingestor seulement après ACK. Erreur ou
annulation : nil ingestor, batch intact et retry du même objet/copie, écritures
de source sérialisées. CP inclus même s'il existe pour vérifier l'ancre dans le Tx.
Resume déjà associé : aucune écriture supplémentaire. Getter du futur ingestor
ne peut exposer une association non acquittée. Caller garde Close/exclusivité et
bytes privés immuables ; lecture manifest/CP n'est pas un snapshot multi-écrivain.

## Pilote d'une tentative du lot 83

Attempt implémente Source pour un seul chemin/run explicite. NewAttempt résout
path et TempDir une fois, valide kind/import/ID/limits/dépendances sans IO. Run
exige un contexte avec deadline avant toute écriture ; TryLock protège cet objet,
caller mono-écrivain pour la source. Dépendances honorent ctx ; deadline entre
opérations bornées, pas interruption d'un syscall de fichier régulier bloqué.

Pending exact d'un précédent Run réessayé avant lookup/ouverture. Run absent :
création running non préparé ; run existant : source/ID/path exacts. Complete déjà
durable revient succès sans rouvrir le path : l'ID identifie cette tentative, pas
un nouvel import d'une entrée peut-être changée. Failed ne se ranime pas ; nouvelle
demande nécessite nouvel ID. Legacy/ID étranger non adopté, conflit SQL conservé.

PrepareRegular possède copie jusqu'au Close différé, toutes sorties joignent
erreurs cleanup. Binding puis ingestion finie, status terminal acquitté distingue
EOF source/Sink. Erreur Sink : retenir batch exact avant fermeture de copie, puis
arrêter ; Run suivant l'acquitte avant revalidation/CP durable. Après perte du
process, pending mémoire perdu, reprendre état commité et contenu entier validé.
Reprise content changé/absent refusée sans transformer run préparé en failed.

Préparation non interrompue refusée sur run sans contenu : trace failed avec date
stable et batch conservé si erreur Sink. Interruption de contexte pendant préparation
reste running pour permettre reprise. Suffixe partiel : failed lastLF acquitté.
Cleanup refusé est signalé, jamais suppression récursive/compensation du manifest
déjà complete. Aucun retry automatique ni plusieurs fichiers dans ce pilote.
