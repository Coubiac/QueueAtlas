# Fiche de suivi opérationnelle — demande utilisateur du 8 octobre 2026

La demande porte sur une vue utilisable par l'opérateur, avec informations du
mail et de son chemin SMTP. Les références internes sont des preuves consultables
dans un volet technique, pas les informations principales. Cette correction de
la revue164 ajoute une projection **HTML seulement** des faits déjà disponibles.
Le tableau suivant distingue ce qui est affiché de ce qui reste à collecter.

Retour humain du 10 octobre : références internes retirées de la liste de
recherche, accessibles dans le diagnostic du détail et les volets fermés de
l’historique. Offsets expliqués comme repères techniques, aucun numéro de ligne
inventé. La provenance de l’année/fuseau est expliquée directement ; les dates
normales n’affichent pas une appréciation de « qualité ».

| Élément demandé | Origine et affichage actuel | Limite / travail restant |
| --- | --- | --- |
| Date et heure | Instant de l'événement en UTC, fractions de seconde conservées ; première/dernière observation dans le détail | Calendrier et fuseau choisi restent au chantier des filtres ; hypothèses temporelles conservées |
| Expéditeur SMTP | `from` journalisé par qmgr, toutes les valeurs distinctes de la génération | Adresse observée après éventuelle réécriture, pas preuve du MAIL FROM initial avant réécriture ; vide explicite conservé |
| Expéditeur visible | Champ From: présent dans la fiche avec « Non observé — en-tête non collecté » | Collecte facultative d'en-têtes à définir avec Subject ; ne jamais substituer le from d'enveloppe |
| Destinataires SMTP multiples | Destinataires des tentatives reconnues, exacts et distincts ; `orig_to` quand observé dans les dernières tentatives | Pas preuve d'un inventaire complet des RCPT TO d'entrée ; aliases/réécritures possibles |
| Message-ID | Champ cleanup de la génération sélectionnée | Absent distinct de vide ; ne fusionne pas les générations ni les instances |
| Queue ID | Identifiant natif Postfix et lien local du détail | Queue ID réutilisable, jamais une identité globale à lui seul |
| Serveur source | `client` du smtpd associé à cette génération, nom/IP tels que journalisés | Aucun remplacement par syslog Host, source d'import, HELO ou DNS inventé ; non observé si absent |
| Serveur destination | `relay` de chaque dernière tentative | Relais, pas forcément la boîte finale |
| Événement | Type de fait en recherche, tous les événements dans la timeline | Réception/traitement ne sont pas un verdict global de réussite ; libellés de timeline à rendre plus explicites |
| Statut SMTP | Code littéral au début d'une réponse SMTP/LMTP quand reconnu ; réponse complète reste visible | Ne pas convertir dsn=2.0.0 en 250 ; les codes intégrés dans d'autres formats de réponse restent à extraire prudemment |
| Motif | Réponse native sélectionnée, échappée et bornée, même sans permission de ligne brute | Les rejets NOQUEUE/non attribués demandent leur fiche propre ; ne pas inventer de motif |
| État final | Dernier résultat observé **par destinataire**, conflits simultanés non déterminés et toutes leurs dernières références/tentatives | Aucun état global final déduit de removed ; « en attente » actuel exige une source d'état de file, pas l'absence d'une livraison dans les logs |
| Durée de traitement | Valeur native `delay` rapportée par Postfix pour chaque dernière tentative, si présente | Pas de durée globale fabriquée avec premier/dernier événement ; pas de soustraction des horloges sous hypothèse |
| Sujet, demandé précédemment | Champ Subject: affiché comme non collecté | Source facultative d'en-tête, parsing borné/MIME et index à réaliser avant filtre |

Postfix traite séparément les adresses d'enveloppe et d'en-tête, avec
[réécritures possibles](https://www.postfix.org/ADDRESS_REWRITING_README.html).
Les [réponses SMTP](https://www.rfc-editor.org/rfc/inline-errata/rfc5321.html#section-4.2)
ne sont pas les codes étendus DSN. La durée reste le champ journalisé, selon les
[paramètres de journalisation des délais Postfix](https://www.postfix.org/postconf.5.html#delay_logging_resolution_limit),
pas un chronomètre calculé par QueueAtlas.

## Implémentation et protections

`buildWebMessage` ne parcourt que les références de la génération reconstruite
sélectionnée. Les valeurs distinctes des champs autorisés sont conservées, sans
choix arbitraire ni normalisation. Il conserve les dernières tentatives de chaque
destinataire, y compris simultanées/contradictoires. Champs absents et vides sont
distincts ; données non UTF-8 étiquetées base64, contrôles/bidi visibles, HTML
échappé. Aucun lien natif, JavaScript, corps de mail ou ligne brute ajouté.

Vue de recherche HTML seulement : from, destinataires, Message-ID, Queue ID et
résultats par adresse. Vue de détail : identité, source SMTP, routes/réponses/DSN/
code SMTP explicite/delay par destinataire, informations techniques repliées.
Le DTO JSON de recherche exclut la vue Web et la branche API ne la calcule pas.
Le détail JSON reste inchangé. Aucun nouveau SELECT ni schéma/source/filtre de
production : même snapshot, garde, admission, deadline et plafond HTML1MiB.
La sélection des métadonnées du détail Web s'élargit explicitement pour répondre
à la demande ; les maps arbitraires, credentials et lignes brutes restent exclus.

## Vérifications et démonstration

Test import Postfix réel→SQLite→recherche/détail HTML : adresses multiples,
Message-ID, client SMTP distinct de l'hôte syslog, deux relais, états mixtes
transmis/différé, réponses250/450 et DSN distincts, durées natives2.0/3.0,
From:/Subject non fabriqués. Autre origine même Queue ID exclue ; sélection
Web absente du JSON. Test de code SMTP : préfixes seulement, pas DSN/quote/
réponse locale, sans effet sur les projections. Tests existants de limites,
révocation, données hostiles, binaire/vide et conflits simultanés conservés.

La fixture TLS humaine utilise désormais **cinq** événements lisibles : smtpd
client, cleanup Message-ID, qmgr expéditeur, smtp alice transmis250, smtp bob
différé450. Aucun retrait ni état de file actuel inventé. La fixture adversariale
à trois événements reste dans les tests de sécurité et l'aperçu statique.
Nouveau rendu et détail à confirmer par l'utilisateur après redémarrage.

Les [filtres calendaires/textuels/ET-OU](search-ux-follow-up.md), collecte
d'en-têtes, vue des rejets non attribués et états de file restent à réaliser.
La revue164 n'est pas clôturée ; PR37 brouillon, MIT et AD/OIDC après MVP.
