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
corrigées (CI108 et branche courante) ; publication/CI111 à vérifier.
Pas de mesure de charge ni couverture, pas de snapshot conservé entre pages.
Les adresses sont exactes et présentes, jamais normalisées depuis l'absence.
