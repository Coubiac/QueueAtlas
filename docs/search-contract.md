# Contrat de recherche des événements

## Lot111 : adresse exacte, période et curseur

`Store.SearchEvents` recherche un champ persisté exact : sender ou recipient,
dans une instance configurée explicite. L'adresse est une valeur SQL paramétrée,
sans trim, lowercase, découpage de domaine, LIKE ni réinterprétation de caractères.
Valeur vide admise : elle correspond à un champ explicitement vide, jamais à NULL
(champ natif absent). Instance non vide <=1024 octets, valeur <=1024, aucun NUL,
CR/LF/tabulation ; l'UTF8 invalide reste une valeur littérale, sans conversion JSON.
Le champ provient d'une énumération fermée, jamais d'une colonne fournie par l'appelant.

Période obligatoire : From inclusif, Until exclusif, strictement croissante et
au plus31 jours. Les deux instants doivent tenir exactement dans UTC UnixNano,
sans débordement. Limite1..200 résultats par page, une ligne supplémentaire vérifie
s'il existe une suite. Le caller fournit son contexte/deadline. Les événements
sans instant UTC persisté sont exclus de cette recherche temporelle : absence
de résultat ne prouve ni absence dans les journaux ni couverture. Qualité de date
conservée ; aucun nouveau contexte d'année/fuseau, aucun reparse du journal.

Les index existants events_sender et events_recipient incluent temps et ID.
Le parcours est croissant `(time_utc_ns, events.id)`, avec comparaison de tuple
pour avancer après le dernier résultat ; aucun OFFSET. Voir la
[pagination par row values SQLite](https://www.sqlite.org/rowvalue.html#scrolling_window_queries).
Le plan EXPLAIN du pilote installé vérifie index et absence de tri temporaire sur
la requête avec curseur. Cette vérification n'est pas une mesure de performance
sur une charge réelle. La fenêtre/limite bornent les résultats, pas indépendamment
le travail SQL lorsque beaucoup de faits d'autres instances doivent être filtrés.

Le curseur contient position temps/ID interne et QueryRevision : SHA256 avec domaine
event-search-v1 et chaînes encadrées par longueur uint64 BE, instance/champ/valeur/
FromNS/UntilNS. Il refuse une réutilisation avec d'autres critères ; taille de page
modifiable. Les instants équivalents dans une autre time.Location gardent le même
hash. Ce curseur n'est ni une autorisation ni une identité de fait. Il n'est pas
signé, et le caller peut choisir de sauter des résultats. API/token opaque futur M4.

Chaque page lit un snapshot SQL ; les pages successives n'immobilisent pas la base.
Un import tardif avant la position déjà franchie nécessite une nouvelle recherche.
Pas de garantie d'ensemble exhaustif pendant ingestion/rétention concurrentes ;
aucun compteur total ou diagnostic d'import tardif déduit de ces pages.

SearchHit contient FactRef physique, instance/file/NOQUEUE, instant, qualité et type
d'événement. Il ne contient pas de brut ni de verdict global ; un rejet NOQUEUE reste
un rejet et un succès SMTP n'est pas promu. Les résultats sont des événements, pas
des messages dédupliqués. Une page ne constitue jamais l'entrée complète d'une
projection : le caller choisit ensuite un périmètre complet via CorrelationFacts.

Requête invalide : ErrSearchQuery ; curseur incompatible : ErrSearchCursor.
Conversion/provenance stockée invalide : ErrSearchStoredHit fixe sans valeur privée.
Sur erreur/annulation, page zéro sans résultat partiel ; contexte annulé distinct.
Une ligne supplémentaire n'est pas décodée avant la page suivante. Ces contrôles
ne constituent pas une authentification contre une réécriture externe de la base.

## Lot112 : Queue ID et Message-ID exacts

SearchQueueID et SearchMessageID : critères fermés, non vides, limites32/1024 octets,
contrôles/instance/période/curseur identiques à111. Valeurs persistées exactes sans
nouvelle normalisation. Le parser Postfix historique retire les délimiteurs `< >`
du Message-ID avant stockage : `filter-11@example.net` est la valeur du corpus11,
distincte de `<filter-11@example.net>` dans la recherche. Aucun parser modifié.

Un Queue ID recyclé conserve tous ses événements ; un Message-ID répété peut
retrouver plusieurs files. Aucun DISTINCT, fusion ou identité globale déduite du
critère. Dates inconnues et limites de snapshot/page restent celles de111.

Queue ID utilise explicitement le prédicat non vide de events_queue existant.
Migration v5 ajoute seulement events_message_id_time(message_id,time_utc_ns,id),
partiel sur Message-ID non NULL et non unique ; anciens schémas/index conservés.
DDL, historique5 et user_version atomiques. Faits/checkpoints/manifests préservés,
échec entièrement rollback à la version initiale ; cas v4 testé. EXPLAIN vérifie index et absence de tri temporaire
avec et sans curseur sur le pilote installé, sans prétendre à une mesure de charge.

## Lot113 : domaines exacts dans des colonnes dédiées

SearchSenderDomain et SearchRecipientDomain recherchent un domaine complet,
pas ses sous-domaines ni un suffixe. Normalisation ASCII en minuscules uniquement,
appliquée au critère et au domaine extrait ; le curseur est lié au critère normalisé,
donc EXAMPLE.ORG et example.org sont équivalents. Instance/période/page restent111.

Sous-ensemble DNS accepté : 1..253 octets, labels1..63, lettres ASCII/chiffres/tirets,
sans tiret au début/fin ; label unique tel localhost admis. A-labels `xn--` littéraux
admis sans validation IDNA. Aucun trim, point final, Unicode de domaine, wildcard,
underscore ou conversion de littéral entre crochets. Requête hors sous-ensemble
refusée ErrSearchQuery ; ce contrat ne constitue pas une validation DNS réseau.

Extraction depuis sender/recipient natifs persistés : exactement un @, partie locale
non vide, UTF8 valide, aucun espace/control/angle/guillemet. Partie locale Unicode
admise si ces conditions sont respectées ; domaine ASCII obligatoire. Ce n'est pas
un parseur complet RFC : les formes citées/ambiguës sont exclues, conservées en
adresse native et accessibles par recherche exacte sous les bornes du critère
adresse111 (<=1024 octets, aucun NUL/CR/LF/tab). Champ absent,
vide ou non extractible donne domaine NULL ; aucune invention de domaine.

La migrationv6 crée event_search_domains : event_id FK events avec cascade,
instance/date copiées, sender_domain et recipient_domain. Index partiels distincts
(instance,domaine,time_utc_ns,event_id), parcours temps/ID avec et sans curseur sans
tri temporaire vérifié par EXPLAIN. Pas de LIKE, calcul à chaque recherche ou fusion.
Les adresses/events/raw restent inchangés, y compris ceux référencés par un manifest.

Backfill des colonnes natives dans une transaction, mémoire par lots de256 ; tous
les événements existants sont parcourus, temps de migration non borné indépendamment
de la taille DB. Table/index/backfill/history6/user_version6 atomiques. Même helper
d'extraction pour migration et nouvelles observations ; ligne dérivée écrite dans
la transaction d'ingestion, échec annule faits et checkpoint, doublon physique ne
réécrit pas. FK cascade évite les lignes orphelines quand une suppression est permise ;
ce mécanisme n'est pas encore une API de rétention. Date inconnue reste NULL, rejet
NOQUEUE reste événement distinct, réserves de couverture et de snapshot inchangées.

## Lot114 : enchaînement explicite avec la reconstruction

Les API de bibliothèque existantes s'enchaînent ainsi ; ce lot ajoute des tests
d'intégration et ce contrat, sans modifier leur runtime ou créer un pilote applicatif.

1. SearchEvents retourne les observations correspondantes, par pages. Un hit désigne
   une provenance physique et propose une instance/file ; ni page ni curseur ne
   définit une identité globale, une génération ou une entrée complète de projection.
2. Le caller choisit explicitement CorrelationScope. Une QueueKey sélectionne tous
   les faits persistés de cette instance/Queue ID, toutes origines et cycles recyclés,
   y compris hors période et sans date. Ne pas transmettre les filtres temporels ou
   les seules adresses/Message-ID de la recherche à cette lecture. Un Message-ID
   répété ou un lien candidat n'élargit pas implicitement le scope aux autres files.
3. CorrelationFacts relit ce périmètre au snapshot SQL courant, avec sa propre limite
   1..4096 indépendante de la taille de page. Dépassement : ErrPartitionLimit et nil,
   aucune entrée tronquée installable. InstallProjection revalide ces faits sous
   réservation d'écriture puis installe le manifest ; ajout de faits dans ce périmètre
   après leur lecture donne ErrProjectionStale, sortie zéro et ancien manifest intact.
4. CurrentProjection reconstruit le manifest vérifié dans son snapshot de lecture.
   Ajout postérieur de faits dans le scope le rend périmé ; relecture complète et installation
   explicite sont nécessaires. Il ne répare pas implicitement une ancienne révision.

Pour NOQUEUE, aucune QueueKey vide ni session déduite du hit. Le caller peut choisir
UnqueuedInstances explicitement : tous les faits sans file des instances choisies,
pas seulement ce rejet, PID ou cette fenêtre. Ce périmètre peut dépasser la limite.
Les sessions restent candidates avec couverture non prouvée ; les rapports sans date
restent non assignés, sans rattachement à une file acceptée.

Les appels recherche/lecture/installation/lecture courante n'ont pas un snapshot
global commun. Une sélection signifie un choix de périmètre, pas l'existence future
garantie du hit ni une preuve de complétude des journaux. Limites d'origine, liens,
dates et réserve coverage_unproven restent celles de la reconstruction.

Trois tests synthétiques vérifient les six critères vers une même file recyclée
(16 faits dont8 sans date et4 hors période), NOQUEUE vers un scope explicite de6 faits,
et un import tardif après sélection (4→8 faits, refus stale/limite, refresh distinct
par origine). Aucun nouveau statut, comportement de corrélation ou parcours ajouté.

## Lot115 : borne de seek et bilan local

La borne scalaire time_utc_ns>=From est resserrée à cursor.TimeNS pour une page
avec curseur validé ; tuple(time,id)>cursor conservé. Le hash garde les critères
d'origine, dont From ; égalités de date et compatibilité des curseurs restent intactes.
Les [mesures locales avant/après](search-measurements.md) décrivent protocole,
données brutes, coût des premières pages filtrant une autre instance et migration
intégrale. Aucun budget SQL autonome ou seuil de production déduit de ce profil.

Chantier111–115 : six critères indexés, domaine dérivé/migrations atomiques, limites,
pagination et chaîne de reconstruction explicite testées/documentées. La clôture
de #25 nécessite CI entière sur tête finale, revue finale et fusion/main vérifiées.
Ce bilan ne clôture pas M3 ; pas de performance MVP représentative ou API livrée.

## Prochaine étape

Rétention cohérente en lots et invalidation explicite des manifests affectés.
Lecture des dates inconnues hors recherche temporelle reste un comportement distinct.
API/Web/authentification et politique de période par défaut restent au jalon M4.
