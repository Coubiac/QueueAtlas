# Timeline Web protégée — lot161

GET/HEAD `/messages/{id}/events` via `NewConsultationHandler`, lien canonique
depuis le détail Web. API seule inchangée. Même garde, Store, admission/délai et
lecture de [timeline API](http-timeline.md), sans listener ou nouveau stockage.

## Lecture et pagination

ID canonique de candidat et paramètres fermés `limit`, `cursor`, `raw`. Limite
1..200/défaut50, raw0ou1/défaut0 ; mêmes codecs/bornes et digest liant position,
candidat/révision et mode brut. Curseur public non signé, pas une autorisation.
Le lien suivant conserve ID/limite/raw ; le formulaire redémarre sans curseur,
ce qui permet de changer de mode. Un `?` vide est accepté comme dans l'API.

Lecture complète de file pour vérifier budget/révision, puis uniquement les faits
de la génération choisie, même hors période de recherche. Ordre date/provenance
pour affichage, pas un ordre causal entre observations simultanées. Réserves de
couverture/temps/continuité conservées ; les résultats ne remplacent pas le résumé
prudent des destinataires. Le détail conserve unknown et toutes références en
cas de tentatives simultanées contradictoires.

ID/query invalides400 avant base ; segment supplémentaire404. Révision modifiée409
avant application de position, même hors génération ; candidat absent404 sans
conclure absence de logs. Budget complet422 même avec limit1, admission429,
erreur privée/délai/annulation/HTML>1MiB503 sans préfixe partiel. HEAD mêmes
lectures/contrôles sans corps. Méthodes/corps/TLS/Host/Origin/proxy/session comme
les autres vues ; garde/protocole antérieurs gardent leurs réponses texte/JSON.

## Métadonnées et lignes brutes

Date UTC/qualité/date native, hôte déclaré/service/PID, kind/parse_failed,
source/origine/offsets int64 exacts, expéditeur/Message-ID présents. Tentative
reconnue : adresse/orig_to, statut natif et interprété, transport, relais/DSN/réponse.
Absence distincte du vide, casse conservée ; métadonnées privées accessibles au
compte même sans ligne brute. Pas d'Observation/Message/maps/compte sérialisés.

`AllowRawLogs` reste false par défaut ; raw1 interdit403 avant base après auth.
La permission serveur seule n'affiche aucune ligne : il faut cocher le formulaire
ou demander raw1. Le contrôle n'est pas proposé sans permission. Aucun rôle,
provider ou configuration YAML de cette permission ajouté. URL/historique privée.

Rendu `html/template`/CSP et CSS embarquée comme recherche/détail. Valeurs native
UTF-8 en texte échappé ; ligne brute dans `pre`, jamais HTML/URL active. Base64
standard paddé explicitement étiqueté pour les octets non UTF-8. Contrôles CR/LF/
tabulation/ANSI/C1 et direction Unicode rendus en notation visible `\r`, `\n`,
`\t`, `\xNN`, `\uNNNN` ; antislash littéral doublé pour éviter de confondre une
séquence native avec un contrôle. Les octets DTO/SQLite restent inchangés ; le
texte de présentation n'est pas une exportation brute à l'identique.
Cette notation s'applique aux champs/provenance de la timeline ; les autres vues
restent à relire pour les caractères de présentation lors de la revue Web.
Même plafond HTML encodé1MiB, distinct de la RAM totale ; calculs coopératifs.

## Vérifications et suite

Quatre nouveaux tests,32HTTPAPI total Windows Go1.26 `-count=1` passent (2.336s),
vet/format/diff passent. SQLite réel lien depuis détail, pages/faits hors fenêtre,
réserves/origines séparées, raw opt-in, budget complet et stale409 avant position.
Protocole/permission avant base, API seule inchangée. Ligne brute binaire SQLite
en base64 exacte dans HTML ; client HTTPS réel payload hostile/contrôles/vide/
absence/métadonnées sans raw/tentatives simultanées/HEAD/révocation. Champs binaires
DTO via seam, sans revendication d'import SQLite de champs binaires. Admission
API/recherche/détail/timeline Web commune, annulation tardive et expansion HTML
de raw64KiB×4 refusée ; limit1 conserve une ligne complète, sans troncature.

Aucun navigateur réel/Linux local revendiqué. Revue XSS de la source au rendu,
CSP, clavier, caractères de présentation et affichage adaptatif reste à faire.
Prochain162 : connexion Web locale, même PR #37 ; montage et compléments ensuite.
Issue #7/M4 restent ouverts. MIT, AD/OIDC/Keycloak après MVP.
