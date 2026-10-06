# Revue du chantier recherche

## Lot111 : recherche exacte et pagination

Résultat attendu : rechercher sender/recipient persistés dans une instance/période
explicite, parcours indexé, curseur lié aux critères, aucun résultat partiel sur erreur.
SearchEvents développé : enum fermé, paramètres littéraux, période UTC <=31 jours,
limite1..200, comparaison de tuple temps/ID après curseur, hits physiques avec réserves
de date/NOQUEUE. Aucun nouveau schéma, parser, verdict, DTO de projection ou Web.

Six tests SearchEvents Windows pass : corpus11/09/06, champ vide vs absent, casse,
instance étrangère/date inconnue/NOQUEUE ; timestamps égaux, From/Until, six hits
sans doublon, curseur incompatible/limit changé/location UTC équivalente ; SQL,
wildcards et octets invalides littéraux ; treize refus, annulation, page maximale200 ;
deux corruptions après un hit valide rendent page zéro et erreur sans donnée ;
EXPLAIN vérifie sender/recipient index avec curseur et sans tri temporaire.
Suite SQLite/vet/diff pass avant remplacement OR par tuple ; six tests/vet/diff
pass sur tuple final. Premier build corrigeait un nom de type model.EventKind
inexistant, remplacé par model.Kind avant tests ; aucun défaut runtime associé.

Revue indépendante code/docs favorable sans blocage ; six tests via overlay Windows
isolé pass, checkout propre/root inchangé. Deux mentions obsolètes de reprise
corrigées (CI108 et branche courante). Publié28eddb9 dans #25 créée/attachée.
CI37372754946 : Windows pass, deux jobs Linux annulés sans runner acquis,
annotations GitHub vérifiées ; aucune étape de test dans ces deux jobs.
Le commit112 déclenchera la validation entière111–112, sans rerun local des fondations.
Pas de mesure de charge ni couverture, pas de snapshot conservé entre pages.
Les adresses sont exactes et présentes, jamais normalisées depuis l'absence.

## Lot112 : identifiants exacts et index v5

Résultat attendu : Queue ID/Message-ID littéraux, résultats non fusionnés, index
temps et pagination, migration sans changement des faits existants. Enum fermée,
non vides32/1024, Queue ID <>'' pour index existant ; nouvel index Message-ID/temps
non unique uniquement, historique/user_version dans la même transaction.

Cinq nouveaux tests et suite SQLite/vet/diff Windows pass : corpus13 huit faits
de deux cycles conservés/instance étrangère exclue ; corpus11 deux files avec même
Message-ID paginées distinctes ; SQL/wildcards/octet invalide littéraux, bornes et
curseur incompatible ; EXPLAIN avec/sans curseur pour deux index sans tri ; v4→v5
préserve faits/CP/projection et reopen ; échec provoqué rollback index/historique/version.
Premiers tests corrigés : nom du corpus13 et attente Message-ID avec angles alors
que le parser existant les retire. Aucun parser changé. Assertions courantes à5,
version6 refusée. Revue code indépendante favorable, cinq tests via overlay isolé
Windows pass, root/Git inchangés, fondations non relancées. Revue documentaire finale
favorable après précision du rollback vers la version initiale (cas v4 testé).
Aucun test relancé ; implémentation112 publiéec44746d dans #25,
CI37415832690 en cours (trois jobs démarrés). Dernière tête incluant ce bilan
documentaire à valider en CI avant clôture du chantier.
