# Contrat de rétention — chantier M3

## Lot116 : prévisualisation bornée en lecture seule

`Store.PreviewRetention(ctx, RetentionQuery)` retourne un aperçu des faits anciens
datés pour une instance configurée exacte. Instance non vide <=1024 octets, sans
NUL/CR/LF/tab ; valeur SQL littérale. Before obligatoire, exact en UTC UnixNano,
exclusif ; limit1..256. Aucun âge par défaut, horloge ou fenêtre inférieure choisi.

Un SELECT joint events/raw_records/checkpoints sur source ET origine physiques,
avec time_utc_ns < Before, end_offset <= checkpoint.offset et ancre non vide.
Les dates UTC NULL et les positions non couvertes restent exclues. La sélection
utilise la date d'événement persistée, pas ReadAt ou la date d'import : une hypothèse
d'année/fuseau reste une hypothèse. La qualité native est renvoyée sans promotion.

Ordre `(event.time_utc_ns, raw_record.id)` ; ID interne seulement pour départager,
identité exposée FactRef physique. Une ligne supplémentaire indique More, pas un
total ou une garantie d'exhaustivité. L'aperçu n'est pas paginé et le refaire sans
changement redonne les mêmes premiers candidats. La ligne supplémentaire n'est pas
décodée ; More signifie existence d'une autre ligne correspondant au filtre SQL.

Chaque appel lit un snapshot de ce SELECT. Résultats : provenance, instantUTC et
qualité ; aucun brut, adresse, ancre, verdict ou token de suppression. Aucun write,
invalidation de projection, modification de checkpoint/manifeste d'import ou
migration. Le test query_only=ON et la lecture de projection avant/après vérifient
cette propriété sur des données synthétiques. Un preview périmé n'est pas une
autorisation : une future purge devra sélectionner/revalider sous réservation
d'écriture et rendre suppression/invalidation atomiques.

Checkpoint couvert/ancre non vide ne prouvent ni fin d'origine, ni absence de
rejeu en cours, ni correspondance actuelle aux octets du fichier ou couverture des
journaux. Origines suivies/inconnues et imports actifs ne sont pas qualifiés terminés
par ce preview. Cette API ne doit pas servir seule à autoriser leur suppression.

Requête invalide : ErrRetentionQuery ; provenance, conversion de colonne/checkpoint
ou qualité stockée invalide : ErrRetentionStoredCandidate fixe, sortie zéro sans
valeur privée. Contexte annulé distinct. La limite borne les résultats, pas les
octets alloués ni le travail SQL indépendamment des instances/positions filtrées ;
le caller fournit son contexte/deadline. Protection du répertoire DB toujours requise,
ces contrôles ne constituent pas une authentification de données modifiées hors API.

## Suite séparée

La suppression d'un ancien fait enlève la clé d'idempotence de raw_records. Un lot
déjà committé peut être rejoué après perte d'ACK : sans protection de provenance,
il pourrait réapparaître ou une collision de bytes passer inaperçue. Conserver le
checkpoint seul ne résout pas cette question, et sa valeur ne justifie pas un ACK
aveugle de bytes inconnus. Ce risque motive le découpage prévu :

1. Lot117 : contrat et protection persistée de provenance pour le rejeu après purge,
   avec tests d'idempotence et de collision ; aucun ACK déduit du seul offset.
2. Lot118 : suppression bornée sous transaction, invalidation explicite de tous
   manifests affectés avant suppression facts/raw/domaines, checkpoints et imports
   conservés ; qualification des origines et revalidation à définir explicitement.
3. Lot119 : intégration reprise/projections/recherche, bilan et clôture de la PR.

Pas de purge automatique, CLI, calendrier ou VACUUM dans116. La durée globale de
purge, la conservation des dates inconnues, le devenir des métadonnées de provenance
et la restauration WAL restent des politiques distinctes à documenter. Sauvegarde
et pilote représentatif restent àM5 ; M3 n'est pas clôturé par ce premier aperçu.

## Lot117 : provenance persistée et rejeu après suppression

La migration v7 ajoute purged_records, initialement vide, sans toucher les faits,
checkpoints, imports ou manifests. Clé physique source/origine/start, end exact et
empreinte SHA256 binaire32. Préimage versionnée avec longueurs séparées pour raw et
read_error ; aucune concaténation ambiguë. ReadAt, observation parsée et verdict ne
font pas partie de l'identité de rejeu, comme pour les doublons raw existants.

Le helper interne rememberPurgedRecord lit les octets persistés et le checkpoint
exact source/origine sous la transaction fournie, exige couverture/ancre non vide,
vérifie conversions et forme, puis mémorise la provenance. Le marqueur doit être
écrit AVANT le DELETE du même raw dans la future transaction de purge. La couverture
checkpoint ne qualifie pas l'origine terminée : ce contrôle supplémentaire reste118.
Le helper ne supprime rien et aucune API publique de purge n'est livrée117.

Commit consulte la provenance persistée avant INSERT. End et empreinte identiques :
record déjà appliqué, pas de résurrection raw/event/domaines. End, octets ou erreur
lecture différents : ErrPurgedRecordCollision fixe ; marqueur malformé/conversion
invalide : ErrPurgedRecordState fixe. La transaction complète est annulée, y compris
faits précédents du lot, nom de source et checkpoint. Contexte annulé reste distinct.
La progression d'import conserve ses propres conditions de manifest/checkpoint :
le marqueur ne permet pas de contourner un conflit d'import ni d'acquitter sur le
seul offset. Tests couvrent également le rejeu d'un import complete après ACK perdu.

L'origine/source/start reste le périmètre : aucune déduplication inter-origines ou
reconnaissance d'une lecture resegmentée. Un offset différent demeure une autre
provenance selon le contrat d'ingestion existant. Aucun fait n'est recréé depuis
l'empreinte et aucune projection historique supprimée n'est restaurée par un retry.

FK vers l'origine conservée et trigger UPDATE immutable. Pas de suppression ou
d'expiration de marqueur par API ; leur durée est actuellement celle du namespace
physique dans la DB. Leur volume peut croître avec les purges. Le digest non clé
n'est ni anonymisation ni authentification contre une modification externe de la
DB : des octets connus peuvent être testés par dictionnaire. Répertoire protégé,
politique de métadonnées/sauvegarde et migrations futures restent nécessaires.
Aucune promesse d'effacement sécurisé des pages SQLite, du WAL ou des sauvegardes.

Les six tests117 simulent le DELETE sous transaction uniquement pour vérifier le
garde de rejeu. Migration/rollback/reopen préservent facts/CP/manifests ; aucune
purge applicative automatique. Le benchmark de migration repart désormais de v5
vers la version courante7 et son reset retire aussi v7. Son smoke1x vérifie le
fonctionnement, pas une nouvelle mesure représentative ; les mesures115 restent
historiquement celles de v5→v6 et leurs sorties brutes ne sont pas réécrites.
