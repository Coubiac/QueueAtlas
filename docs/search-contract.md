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

## Suite

Ajouter séparément les critères Queue ID/Message-ID, recherche de domaine avec
colonnes dédiées, intégration à la reconstruction et mesures sur corpus représentatif.
Rétention cohérente et lecture des dates inconnues restent des comportements distincts.
API/Web/authentification et politique de période par défaut restent au jalon M4.
