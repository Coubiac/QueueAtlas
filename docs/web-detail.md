# Détail Web protégé du candidat — lot160

`NewConsultationHandler` ajoute GET/HEAD `/messages/{id}` aux routes de recherche
Web et API. Les candidats attribués de recherche proposent un lien vers ce détail.
NOQUEUE/streams non résolus restent sans lien. Même routeur, garde, Store, budget
d'admission et délai ; `NewSearchHandler` reste limité à l'API.

## Identité et lecture

L'ID et la lecture sont ceux du [détail API](http-detail.md) : identifiant canonique
base64url borné, non signé, ni secret ni permission ; une génération d'une file
exacte, pas un message global. Aucun query, même `?` vide, ni corps n'est accepté.
ID invalide400 avant base, segment supplémentaire404, GET/HEAD seulement.

Le détail appelle directement le même `h.detail` : toutes les origines/cycles/
dates de la file participent au budget et à sa révision, puis seule la génération
sélectionnée est présentée. Les faits hors fenêtre de recherche sont conservés.
Une révision différente409 impose une nouvelle recherche ; aucune autre génération
ou nouvelle révision n'est dévoilée en remplacement. Candidat absent404 ne prouve
pas l'absence des journaux. Budget de faits422 entier, admission429, erreur privée/
annulation/délai/HTML>1MiB503. Aucun résultat partiel. HEAD mêmes lectures sans corps.

## Contenu et rendu

- File/instance/génération, réception native observée, ancre/retrait et références
  physiques exactes, réserves temporelles/entre origines, comptes prudents,
  expirations rapportées et tentatives non interprétées.
- Destinataires exacts, casse distincte, adresses vides explicitement observées et
  réserve d'adresse non spécifiée. Valeur binaire du DTO en base64 étiqueté, sans
  remplacement ni décodage comme HTML ; cela ne change pas le codec JSON SQLite.
- Résultat observé et nombre de tentatives ; références de toutes les dernières
  tentatives simultanées, conservées comme incertaines. `sent` reste du transport.
- Provenance en texte et offsets int64 décimaux, sans liens actifs issus des logs.
  Pas de lignes/messages bruts, DSN/relay/réponse ou maps/credential dans ce détail.

Même `html/template`, stylesheet immuable embarquée/hash CSP, anti-framing,
no-referrer/no-store/nosniff que la [recherche Web](web-search.md). Le lien de
recherche utilise uniquement l'ID produit par l'encodeur canonique et un préfixe
local fixe. Texte hostile toujours échappé ; aucun champ SMTP n'est une URL active.
Les erreurs HTML sont fixes et ne reflètent ni l'ID ni les données privées.
Le garde/protocole antérieur conserve ses réponses texte/JSON. Plafond encodé
<=1MiB avant succès, pas plafond de RAM totale, délais CPU coopératifs.

## Vérifications et suite

Trois nouveaux tests,28HTTPAPI total passent Windows Go1.26 `-count=1` (2.276s),
vet/format/diff passent. SQLite réel recherche Web→lien canonique→détail complet,
origines distinctes, réception/retrait/livraison hors fenêtre, génération absente,
budget complet et import tardif409. Protocole/auth/query/ID avant base et route
API seule inchangée ; client HTTPS réel GET/HEAD et session révoquée avec texte
hostile/casse/vide/base64/offset>2^53 et résultats simultanés inconnus. Binaire via
seam privé, pas preuve d'import SQLite de champs non UTF-8. Admission API/recherche
Web/détail partagée ; annulation, erreurs internes et expansion des nombreuses
références HTML>1MiB refusées sans données partielles, slot libéré ensuite.

La validation navigateur de bout en bout, clavier/rendu adaptatif et traitement
des caractères de présentation restent pour la revue Web ; aucun navigateur réel
ni Linux local revendiqué. Depuis161, lien vers la [timeline Web](web-timeline.md)
paginée avec tentatives/permission brute explicite/rendu texte. Même PR #37.
Prochain162 : connexion Web locale. Connexion/montage serveur,
filtres/diagnostics et revue restent ; l'issue #7/M4 ne sont pas clos. MIT,
AD/OIDC/Keycloak après MVP.
